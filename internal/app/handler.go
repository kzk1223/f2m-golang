// Package app は、フォーム画面のHTTP制御を扱うpackageである。
package app

import (
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strings"

	"f2m-golang/internal/config"
)

/**
 * フォーム画面制御。
 *
 * 入力、確認、完了の画面遷移を扱うHTTP handler。
 */
type Handler struct {
	configSet *config.ConfigSet
}

/**
 * フォーム画面制御の生成。
 *
 * 設定集合を保持したHTTP handlerを返す。
 */
func New(configSet *config.ConfigSet) http.Handler {
	return &Handler{
		configSet: configSet,
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
	case http.MethodGet:
		formConfig, ok := handler.defaultFormConfig()
		if !ok {
			http.Error(w, "form config not found", http.StatusInternalServerError)
			return
		}

		handler.renderForm(w, formConfig, nil, FormErrors{})
	case http.MethodPost:
		handler.handlePost(w, r)
	default:
		w.Header().Set("Allow", http.MethodGet+", "+http.MethodPost)
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
	handler.render(w, formConfig.ConfirmPath, handler.newPageView(formConfig, fieldValues))
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
 * 既定フォーム設定選択。
 *
 * GET表示用に安定した順序で先頭のフォーム設定を取得する処理。
 */
func (handler *Handler) defaultFormConfig() (config.FormConfig, bool) {
	if handler.configSet == nil || len(handler.configSet.Forms) == 0 {
		return config.FormConfig{}, false
	}

	formIDs := make([]string, 0, len(handler.configSet.Forms))
	for formID := range handler.configSet.Forms {
		formIDs = append(formIDs, formID)
	}

	sort.Strings(formIDs)

	return handler.configSet.Forms[formIDs[0]], true
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
	FormID string
	Title  string
	Fields []FieldView
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
