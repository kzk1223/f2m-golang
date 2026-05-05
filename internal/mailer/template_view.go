package mailer

import (
	"strings"

	"f2m-golang/internal/config"
)

/**
 * メールテンプレート表示データ。
 *
 * メール本文テンプレートに渡すフォーム単位の値。
 */
type TemplateView struct {
	FormID  string
	Subject string
	Fields  []FieldView
	Values  map[string][]string
}

/**
 * メール項目表示データ。
 *
 * メール本文テンプレートに渡す項目単位の値。
 */
type FieldView struct {
	Name   string
	Label  string
	Value  string
	Values []string
}

/**
 * 先頭入力値取得。
 *
 * テンプレートから単一入力値を参照する処理。
 */
func (view TemplateView) First(fieldName string) string {
	values := view.Values[fieldName]
	if len(values) == 0 {
		return ""
	}

	return values[0]
}

/**
 * 入力値結合。
 *
 * テンプレートから複数入力値を日本語読点区切りで参照する処理。
 */
func (view TemplateView) Joined(fieldName string) string {
	return joinFieldValues(view.Values[fieldName])
}

/**
 * メールテンプレート表示データ生成。
 *
 * 設定上の項目順に入力値を整形する処理。
 */
func newTemplateView(formConfig config.FormConfig, fieldValues map[string][]string) TemplateView {
	fields := make([]FieldView, 0, len(formConfig.FieldOrder))
	for _, fieldName := range formConfig.FieldOrder {
		label := formConfig.FieldLabels[fieldName]
		if label == "" {
			label = fieldName
		}

		fields = append(fields, FieldView{
			Name:   fieldName,
			Label:  label,
			Value:  joinFieldValues(fieldValues[fieldName]),
			Values: cloneValues(fieldValues[fieldName]),
		})
	}

	return TemplateView{
		FormID:  formConfig.ID,
		Subject: formConfig.Subject,
		Fields:  fields,
		Values:  cloneFieldValues(fieldValues),
	}
}

/**
 * 入力値集合複製。
 *
 * テンプレート側で参照する入力値を複製する処理。
 */
func cloneFieldValues(fieldValues map[string][]string) map[string][]string {
	clonedFieldValues := make(map[string][]string, len(fieldValues))
	for fieldName, values := range fieldValues {
		clonedFieldValues[fieldName] = cloneValues(values)
	}

	return clonedFieldValues
}

/**
 * 入力値スライス複製。
 *
 * 入力値スライスを外部変更から分離する処理。
 */
func cloneValues(values []string) []string {
	clonedValues := make([]string, len(values))
	copy(clonedValues, values)

	return clonedValues
}

/**
 * 入力値結合。
 *
 * 複数値を日本語読点区切りで1つの文字列に変換する処理。
 */
func joinFieldValues(values []string) string {
	return strings.Join(values, "、")
}
