package mailer

import (
	"strings"

	"f2m-golang/internal/attachment"
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
 * 設定上の項目順に入力値と添付ファイル名を整形する処理。
 */
func newTemplateView(formConfig config.FormConfig, fieldValues map[string][]string, attachmentFiles []attachment.File) TemplateView {
	fieldOrder := mailFieldOrder(formConfig)
	templateValues := mailTemplateValues(fieldValues, attachmentFiles)
	fields := make([]FieldView, 0, len(fieldOrder))
	for _, fieldName := range fieldOrder {
		label := formConfig.FieldLabels[fieldName]
		if label == "" {
			label = fieldName
		}
		values := templateValues[fieldName]

		fields = append(fields, FieldView{
			Name:   fieldName,
			Label:  label,
			Value:  joinFieldValues(values),
			Values: cloneValues(values),
		})
	}

	return TemplateView{
		FormID:  formConfig.ID,
		Subject: formConfig.Subject,
		Fields:  fields,
		Values:  templateValues,
	}
}

/**
 * メール項目順序生成。
 *
 * F2M_JPNAMEの順序に添付項目を追加した表示順を生成する処理。
 */
func mailFieldOrder(formConfig config.FormConfig) []string {
	fieldOrder := make([]string, 0, len(formConfig.FieldOrder)+len(formConfig.AttachFields))
	seen := make(map[string]bool)

	appendFieldName := func(fieldName string) {
		fieldName = strings.TrimSpace(fieldName)
		if fieldName == "" || seen[fieldName] {
			return
		}

		fieldOrder = append(fieldOrder, fieldName)
		seen[fieldName] = true
	}

	for _, fieldName := range formConfig.FieldOrder {
		appendFieldName(fieldName)
	}
	for _, fieldName := range formConfig.AttachFields {
		appendFieldName(fieldName)
	}

	return fieldOrder
}

/**
 * メールテンプレート値生成。
 *
 * 通常入力値へ添付ファイル名を追加した値集合を生成する処理。
 */
func mailTemplateValues(fieldValues map[string][]string, attachmentFiles []attachment.File) map[string][]string {
	templateValues := cloneFieldValues(fieldValues)
	for _, attachmentFile := range attachmentFiles {
		fieldName := strings.TrimSpace(attachmentFile.FieldName)
		if fieldName == "" || strings.TrimSpace(attachmentFile.OriginalName) == "" {
			continue
		}

		templateValues[fieldName] = append(templateValues[fieldName], attachmentFile.OriginalName)
	}

	return templateValues
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
