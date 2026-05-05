// Package app は、フォーム画面のHTTP制御を扱うpackageである。
package app

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"f2m-golang/internal/config"
	"f2m-golang/internal/security"
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
	submitTokenSigner *security.SubmitTokenSigner
}

/**
 * フォーム画面制御の生成。
 *
 * 設定集合を保持したHTTP handlerを返す。
 */
func New(configSet *config.ConfigSet) http.Handler {
	submitTokenSigner, err := security.NewSubmitTokenSigner()
	if err != nil {
		panic(err)
	}

	return &Handler{
		configSet:         configSet,
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
	// パス制御
	// ---------------------------------------------
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// ---------------------------------------------
	// メソッド制御
	// ---------------------------------------------
	switch r.Method {
	case http.MethodPost:
		handler.handlePost(w, r)
	default:
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
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
func (handler *Handler) renderForm(w http.ResponseWriter, formConfig config.FormConfig, fieldValues map[string]string, formErrors FormErrors) {
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
func (handler *Handler) renderConfirm(w http.ResponseWriter, formConfig config.FormConfig, fieldValues map[string]string) {
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
func (handler *Handler) renderThanks(w http.ResponseWriter, formConfig config.FormConfig, fieldValues map[string]string) {
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
func (handler *Handler) collectFieldValues(r *http.Request, formConfig config.FormConfig) map[string]string {
	fieldValues := make(map[string]string)

	for _, fieldName := range configuredFieldNames(formConfig) {
		fieldValues[fieldName] = r.PostFormValue(fieldName)
	}

	return fieldValues
}

/**
 * 送信トークン検証。
 *
 * 確認画面で発行した署名付き送信トークンとPOST値の一致を検証する処理。
 */
func (handler *Handler) verifySubmitToken(r *http.Request, formConfig config.FormConfig, fieldValues map[string]string) error {
	return handler.submitTokenSigner.Verify(r.PostFormValue(submitTokenFieldName), formConfig.ID, fieldValues)
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
func (handler *Handler) newPageView(formConfig config.FormConfig, fieldValues map[string]string) PageView {
	fields := make([]FieldView, 0, len(formConfig.FieldOrder))

	for _, fieldName := range formConfig.FieldOrder {
		fields = append(fields, FieldView{
			Name:      fieldName,
			Label:     formConfig.FieldLabels[fieldName],
			Value:     fieldValues[fieldName],
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
	Type      string
	Multiline bool
}
