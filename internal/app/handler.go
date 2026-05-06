// Package app は、フォーム画面のHTTP制御を扱うpackageである。
package app

import (
	"fmt"
	"html/template"
	"net"
	"net/http"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"f2m-golang/internal/config"
	"f2m-golang/internal/mailer"
	"f2m-golang/internal/security"
	"f2m-golang/internal/storage"
)

const (
	submitTokenFieldName = "f2m_submit_token"
	defaultTokenExpire   = 30 * time.Minute
)

/**
 * フォーム画面制御。
 *
 * 入力、確認、完了の画面遷移を扱うHTTP handler。
 */
type Handler struct {
	configSet         *config.ConfigSet
	mailService       *mailer.Service
	submitTokenSigner *security.SubmitTokenSigner
}

/**
 * フォーム画面制御の生成。
 *
 * 設定集合を保持したHTTP handlerを返す。
 */
func New(configSet *config.ConfigSet) http.Handler {
	return newHandler(configSet, mailer.NewService(mailer.NewGoMailSender()))
}

/**
 * フォーム画面制御の生成。
 *
 * テスト用のメール送信処理を差し替えたHTTP handlerを返す。
 */
func newWithMailSender(configSet *config.ConfigSet, mailSender mailer.Sender) http.Handler {
	return newHandler(configSet, mailer.NewService(mailSender))
}

/**
 * フォーム画面制御の生成。
 *
 * 設定集合とメール送信サービスを保持したHTTP handlerを返す。
 */
func newHandler(configSet *config.ConfigSet, mailService *mailer.Service) http.Handler {
	submitTokenSigner, err := security.NewSubmitTokenSigner()
	if err != nil {
		panic(err)
	}

	return &Handler{
		configSet:         configSet,
		mailService:       mailService,
		submitTokenSigner: submitTokenSigner,
	}
}

/**
 * HTTPリクエスト処理。
 *
 * メソッドとmode値に応じた画面遷移制御。
 */
func (handler *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// ---------------------------------------------
	// メソッド制御
	// ---------------------------------------------
	switch r.Method {
	case http.MethodGet:
		handler.handleGet(w, r)
	case http.MethodPost:
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		handler.handlePost(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

/**
 * フォームパス判定。
 *
 * リクエストパスが設定済みフォームHTMLに対応するかを返す処理。
 */
func HasFormPath(configSet *config.ConfigSet, requestPath string) bool {
	_, ok := findFormConfigByPath(configSet, requestPath)

	return ok
}

/**
 * GETリクエスト処理。
 *
 * F2M_FORMに対応するフォームHTMLをアプリ側で描画する処理。
 */
func (handler *Handler) handleGet(w http.ResponseWriter, r *http.Request) {
	formConfig, ok := findFormConfigByPath(handler.configSet, r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}

	handler.renderForm(w, formConfig, FieldValues{}, FormErrors{})
}

/**
 * POSTリクエスト処理。
 *
 * mode値に応じた入力、確認、完了画面への振り分け。
 */
func (handler *Handler) handlePost(w http.ResponseWriter, r *http.Request) {
	// ---------------------------------------------
	// フォーム値解析
	// ---------------------------------------------
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	formConfig, ok := handler.findFormConfig(r)
	if !ok {
		http.Error(w, "invalid F2M_ID", http.StatusBadRequest)
		return
	}

	if honeypotFilled(r, formConfig) {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	fieldValues := handler.collectFieldValues(r, formConfig)

	// ---------------------------------------------
	// 画面遷移
	// ---------------------------------------------
	switch r.PostFormValue("mode") {
	case "form":
		handler.renderForm(w, formConfig, fieldValues, FormErrors{})
	case "send":
		formErrors := validateFields(formConfig, fieldValues)
		if formErrors.HasErrors() {
			handler.renderForm(w, formConfig, fieldValues, formErrors)
			return
		}

		if err := handler.verifySubmitToken(r, formConfig, fieldValues); err != nil {
			http.Error(w, "invalid submit token", http.StatusBadRequest)
			return
		}

		submitMeta := newCSVSubmitMeta(r)
		if err := storage.AppendCSV(formConfig, fieldValues, submitMeta); err != nil {
			http.Error(w, "csv save error", http.StatusInternalServerError)
			return
		}

		if err := handler.mailService.SendAll(formConfig, fieldValues, newMailSubmitMeta(submitMeta)); err != nil {
			http.Error(w, "mail send error", http.StatusInternalServerError)
			return
		}

		handler.renderThanks(w, formConfig, fieldValues)
	default:
		formErrors := validateFields(formConfig, fieldValues)
		if formErrors.HasErrors() {
			handler.renderForm(w, formConfig, fieldValues, formErrors)
			return
		}

		handler.renderConfirm(w, formConfig, fieldValues)
	}
}

/**
 * 入力画面描画。
 *
 * 固定HTMLフォームへ入力値とエラーを反映する処理。
 */
func (handler *Handler) renderForm(w http.ResponseWriter, formConfig config.FormConfig, fieldValues FieldValues, formErrors FormErrors) {
	renderedHTML, err := renderFixedFormFile(formConfig.FormPath, formConfig, fieldValues, formErrors)
	if err != nil {
		http.Error(w, "form render error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, renderedHTML)
}

/**
 * 確認画面描画。
 *
 * 確認テンプレートへ画面表示用データを渡す処理。
 */
func (handler *Handler) renderConfirm(w http.ResponseWriter, formConfig config.FormConfig, fieldValues FieldValues) {
	pageView := handler.newPageView(formConfig, fieldValues)
	submitToken, err := handler.submitTokenSigner.Sign(formConfig.ID, fieldValues, tokenExpire(formConfig))
	if err != nil {
		http.Error(w, "submit token error", http.StatusInternalServerError)
		return
	}

	pageView.SubmitToken = submitToken

	handler.render(w, formConfig.ConfirmPath, pageView)
}

/**
 * 完了画面描画。
 *
 * 完了テンプレートへ画面表示用データを渡す処理。
 */
func (handler *Handler) renderThanks(w http.ResponseWriter, formConfig config.FormConfig, fieldValues FieldValues) {
	handler.render(w, formConfig.ThanksPath, handler.newPageView(formConfig, fieldValues))
}

/**
 * テンプレート描画。
 *
 * 指定テンプレートファイルを解析し、HTTPレスポンスへ出力する処理。
 */
func (handler *Handler) render(w http.ResponseWriter, templatePath string, pageView PageView) {
	// ---------------------------------------------
	// テンプレート解析
	// ---------------------------------------------
	pageTemplate, err := template.ParseFiles(templatePath)
	if err != nil {
		http.Error(w, "template parse error", http.StatusInternalServerError)
		return
	}

	// ---------------------------------------------
	// レスポンス出力
	// ---------------------------------------------
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := pageTemplate.Execute(w, pageView); err != nil {
		http.Error(w, "template execute error", http.StatusInternalServerError)
	}
}

/**
 * フォーム値収集。
 *
 * 設定上の表示順に対応する入力値だけを取得する処理。
 */
func (handler *Handler) collectFieldValues(r *http.Request, formConfig config.FormConfig) FieldValues {
	fieldValues := make(FieldValues)

	for _, fieldName := range configuredFieldNames(formConfig) {
		fieldValues[fieldName] = append([]string(nil), r.PostForm[fieldName]...)
	}

	return fieldValues
}

/**
 * 送信トークン検証。
 *
 * 確認画面で発行した署名付き送信トークンとPOST値の一致を検証する処理。
 */
func (handler *Handler) verifySubmitToken(r *http.Request, formConfig config.FormConfig, fieldValues FieldValues) error {
	return handler.submitTokenSigner.Verify(r.PostFormValue(submitTokenFieldName), formConfig.ID, fieldValues)
}

/**
 * honeypot入力判定。
 *
 * bot検知用項目に空白以外の値がPOSTされたかを返す処理。
 */
func honeypotFilled(r *http.Request, formConfig config.FormConfig) bool {
	honeypotField := strings.TrimSpace(formConfig.HoneypotField)
	if !formConfig.HoneypotEnabled || honeypotField == "" {
		return false
	}

	for _, fieldValue := range r.PostForm[honeypotField] {
		if strings.TrimSpace(fieldValue) != "" {
			return true
		}
	}

	return false
}

/**
 * CSV送信メタ情報生成。
 *
 * HTTPリクエストからCSV保存用の送信日時と参考IP情報を生成する処理。
 */
func newCSVSubmitMeta(r *http.Request) storage.CSVSubmitMeta {
	return storage.CSVSubmitMeta{
		SentAt:        time.Now(),
		RemoteIP:      remoteIP(r),
		XForwardedFor: r.Header.Get("X-Forwarded-For"),
		XRealIP:       r.Header.Get("X-Real-IP"),
	}
}

/**
 * メール送信メタ情報生成。
 *
 * CSV保存と同じ送信付加情報をメール送信用の値へ変換する処理。
 */
func newMailSubmitMeta(submitMeta storage.CSVSubmitMeta) mailer.SubmitMeta {
	return mailer.SubmitMeta{
		SentAt:        submitMeta.SentAt,
		RemoteIP:      submitMeta.RemoteIP,
		XForwardedFor: submitMeta.XForwardedFor,
		XRealIP:       submitMeta.XRealIP,
	}
}

/**
 * 接続元IP取得。
 *
 * RemoteAddrからポートを除いた接続元IPを取得する処理。
 */
func remoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}

	return r.RemoteAddr
}

/**
 * 送信トークン有効期限取得。
 *
 * 設定値がない場合は初期有効期限を返す処理。
 */
func tokenExpire(formConfig config.FormConfig) time.Duration {
	if formConfig.TokenExpire > 0 {
		return formConfig.TokenExpire
	}

	return defaultTokenExpire
}

/**
 * 設定済み項目名生成。
 *
 * 表示順と検証対象から入力値収集に必要な項目名を生成する処理。
 */
func configuredFieldNames(formConfig config.FormConfig) []string {
	fieldNames := make([]string, 0)
	seen := make(map[string]bool)

	appendFieldName := func(fieldName string) {
		fieldName = strings.TrimSpace(fieldName)
		if fieldName == "" || seen[fieldName] {
			return
		}

		fieldNames = append(fieldNames, fieldName)
		seen[fieldName] = true
	}

	for _, fieldName := range formConfig.FieldOrder {
		appendFieldName(fieldName)
	}
	for _, fieldName := range orderedFieldNames(nil, formConfig.RequiredFields) {
		appendFieldName(fieldName)
	}
	for _, fieldName := range orderedFieldNames(nil, formConfig.EmailFields) {
		appendFieldName(fieldName)
	}
	for _, equalField := range formConfig.EqualFields {
		appendFieldName(equalField.Left)
		appendFieldName(equalField.Right)
	}

	return fieldNames
}

/**
 * 画面表示データ生成。
 *
 * テンプレートから扱いやすい表示用データへの変換処理。
 */
func (handler *Handler) newPageView(formConfig config.FormConfig, fieldValues FieldValues) PageView {
	fields := make([]FieldView, 0, len(formConfig.FieldOrder))

	for _, fieldName := range formConfig.FieldOrder {
		fields = append(fields, FieldView{
			Name:      fieldName,
			Label:     formConfig.FieldLabels[fieldName],
			Value:     fieldValues.Joined(fieldName),
			Values:    fieldValues.CloneValues(fieldName),
			Type:      fieldType(formConfig, fieldName),
			Multiline: isMultilineField(fieldName),
		})
	}

	return PageView{
		FormID: formConfig.ID,
		Title:  formConfig.Subject,
		Fields: fields,
	}
}

/**
 * フォーム設定選択。
 *
 * POSTされたF2M_IDに対応するフォーム設定を取得する処理。
 */
func (handler *Handler) findFormConfig(r *http.Request) (config.FormConfig, bool) {
	formID := strings.TrimSpace(r.PostFormValue("F2M_ID"))
	if formID == "" {
		return config.FormConfig{}, false
	}

	formConfig, ok := handler.configSet.Forms[formID]

	return formConfig, ok
}

/**
 * フォームパス指定による設定選択。
 *
 * F2M_FORMの設定パスとリクエストパスを照合する処理。
 */
func findFormConfigByPath(configSet *config.ConfigSet, requestPath string) (config.FormConfig, bool) {
	if configSet == nil {
		return config.FormConfig{}, false
	}

	normalizedRequestPath := normalizeRequestPath(requestPath)
	if normalizedRequestPath == "" {
		return config.FormConfig{}, false
	}

	formIDs := make([]string, 0, len(configSet.Forms))
	for formID := range configSet.Forms {
		formIDs = append(formIDs, formID)
	}
	sort.Strings(formIDs)

	for _, formID := range formIDs {
		formConfig := configSet.Forms[formID]
		if formPathRequestPath(formConfig.FormPath) == normalizedRequestPath {
			return formConfig, true
		}
	}

	return config.FormConfig{}, false
}

/**
 * リクエストパス正規化。
 *
 * URLパスを先頭スラッシュ付きの比較用パスへ変換する処理。
 */
func normalizeRequestPath(requestPath string) string {
	requestPath = strings.TrimSpace(requestPath)
	if requestPath == "" {
		return ""
	}

	return path.Clean("/" + strings.TrimPrefix(requestPath, "/"))
}

/**
 * フォーム設定パス正規化。
 *
 * F2M_FORMのファイルパスをURL比較用パスへ変換する処理。
 */
func formPathRequestPath(formPath string) string {
	trimmedFormPath := strings.TrimSpace(formPath)
	var normalizedFormPath string
	if filepath.IsAbs(trimmedFormPath) {
		normalizedFormPath = filepath.Base(trimmedFormPath)
	} else {
		normalizedFormPath = filepath.ToSlash(trimmedFormPath)
	}

	normalizedFormPath = strings.TrimPrefix(normalizedFormPath, "./")
	normalizedFormPath = strings.TrimPrefix(normalizedFormPath, "/")
	if normalizedFormPath == "" || normalizedFormPath == "." {
		return ""
	}

	return path.Clean("/" + normalizedFormPath)
}

/**
 * 入力種別判定。
 *
 * 設定上のメールチェック対象をemail入力として扱う処理。
 */
func fieldType(formConfig config.FormConfig, fieldName string) string {
	if formConfig.EmailFields[fieldName] {
		return "email"
	}

	return "text"
}

/**
 * 複数行項目判定。
 *
 * 本文系フィールドをtextareaとして扱うための簡易判定。
 */
func isMultilineField(fieldName string) bool {
	switch strings.ToLower(fieldName) {
	case "body", "comment", "contact", "content", "inquiry", "message":
		return true
	default:
		return false
	}
}

/**
 * 画面表示データ。
 *
 * テンプレートに渡すページ単位の値。
 */
type PageView struct {
	FormID      string
	Title       string
	Fields      []FieldView
	SubmitToken string
}

/**
 * 項目表示データ。
 *
 * テンプレートに渡すフォーム項目単位の値。
 */
type FieldView struct {
	Name      string
	Label     string
	Value     string
	Values    []string
	Type      string
	Multiline bool
}
