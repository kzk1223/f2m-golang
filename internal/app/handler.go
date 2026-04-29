// Package app は、フォーム画面のHTTP制御を扱うpackageである。
package app

import (
	"html/template"
	"net/http"
	"strings"

	"f2m-golang/internal/config"
)

/**
 * フォーム画面制御。
 *
 * 入力、確認、完了の画面遷移を扱うHTTP handler。
 */
type Handler struct {
	formConfig config.FormConfig
}

/**
 * フォーム画面制御の生成。
 *
 * フォーム設定を保持したHTTP handlerを返す。
 */
func New(formConfig config.FormConfig) http.Handler {
	return &Handler{
		formConfig: formConfig,
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
		handler.renderForm(w, nil)
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

	fieldValues := handler.collectFieldValues(r)

	// ---------------------------------------------
	// 画面遷移
	// ---------------------------------------------
	switch r.PostFormValue("mode") {
	case "form":
		handler.renderForm(w, fieldValues)
	case "send":
		handler.renderThanks(w, fieldValues)
	default:
		handler.renderConfirm(w, fieldValues)
	}
}

/**
 * 入力画面描画。
 *
 * フォームテンプレートへ画面表示用データを渡す処理。
 */
func (handler *Handler) renderForm(w http.ResponseWriter, fieldValues map[string]string) {
	handler.render(w, handler.formConfig.FormPath, handler.newPageView(fieldValues))
}

/**
 * 確認画面描画。
 *
 * 確認テンプレートへ画面表示用データを渡す処理。
 */
func (handler *Handler) renderConfirm(w http.ResponseWriter, fieldValues map[string]string) {
	handler.render(w, handler.formConfig.ConfirmPath, handler.newPageView(fieldValues))
}

/**
 * 完了画面描画。
 *
 * 完了テンプレートへ画面表示用データを渡す処理。
 */
func (handler *Handler) renderThanks(w http.ResponseWriter, fieldValues map[string]string) {
	handler.render(w, handler.formConfig.ThanksPath, handler.newPageView(fieldValues))
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
func (handler *Handler) collectFieldValues(r *http.Request) map[string]string {
	fieldValues := make(map[string]string)

	for _, fieldName := range handler.formConfig.FieldOrder {
		fieldValues[fieldName] = r.PostFormValue(fieldName)
	}

	return fieldValues
}

/**
 * 画面表示データ生成。
 *
 * テンプレートから扱いやすい表示用データへの変換処理。
 */
func (handler *Handler) newPageView(fieldValues map[string]string) PageView {
	fields := make([]FieldView, 0, len(handler.formConfig.FieldOrder))

	for _, fieldName := range handler.formConfig.FieldOrder {
		fields = append(fields, FieldView{
			Name:      fieldName,
			Label:     handler.formConfig.FieldLabels[fieldName],
			Value:     fieldValues[fieldName],
			Type:      handler.fieldType(fieldName),
			Multiline: isMultilineField(fieldName),
		})
	}

	return PageView{
		Title:  handler.formConfig.Subject,
		Fields: fields,
	}
}

/**
 * 入力種別判定。
 *
 * 設定上のメールチェック対象をemail入力として扱う処理。
 */
func (handler *Handler) fieldType(fieldName string) string {
	if handler.formConfig.EmailFields[fieldName] {
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
