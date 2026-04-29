package app

import (
	"fmt"
	"net/mail"
	"sort"
	"strings"

	"f2m-golang/internal/config"
)

/**
 * 入力エラー集合。
 *
 * 項目別エラーと概要表示用メッセージを保持する値。
 */
type FormErrors struct {
	Summary []string
	Fields  map[string][]string
}

/**
 * エラー有無判定。
 *
 * 入力エラーが存在するかを返す処理。
 */
func (formErrors FormErrors) HasErrors() bool {
	return len(formErrors.Summary) > 0
}

/**
 * 入力エラー追加。
 *
 * 項目別エラーと概要メッセージを同時に追加する処理。
 */
func (formErrors *FormErrors) Add(fieldName string, message string) {
	if formErrors.Fields == nil {
		formErrors.Fields = make(map[string][]string)
	}

	formErrors.Fields[fieldName] = append(formErrors.Fields[fieldName], message)
	formErrors.Summary = append(formErrors.Summary, message)
}

/**
 * 入力値検証。
 *
 * フォーム設定に基づく必須、メール形式、一致チェックの処理。
 */
func validateFields(formConfig config.FormConfig, fieldValues map[string]string) FormErrors {
	formErrors := FormErrors{
		Fields: make(map[string][]string),
	}

	// ---------------------------------------------
	// 必須チェック
	// ---------------------------------------------
	for _, fieldName := range orderedFieldNames(formConfig.FieldOrder, formConfig.RequiredFields) {
		if !formConfig.RequiredFields[fieldName] {
			continue
		}

		if strings.TrimSpace(fieldValues[fieldName]) == "" {
			formErrors.Add(fieldName, fmt.Sprintf("%sを入力してください。", fieldLabel(formConfig, fieldName)))
		}
	}

	// ---------------------------------------------
	// メール形式チェック
	// ---------------------------------------------
	for _, fieldName := range orderedFieldNames(formConfig.FieldOrder, formConfig.EmailFields) {
		if !formConfig.EmailFields[fieldName] {
			continue
		}

		fieldValue := strings.TrimSpace(fieldValues[fieldName])
		if fieldValue != "" && !isEmailAddress(fieldValue) {
			formErrors.Add(fieldName, fmt.Sprintf("%sの形式が不正です。", fieldLabel(formConfig, fieldName)))
		}
	}

	// ---------------------------------------------
	// 一致チェック
	// ---------------------------------------------
	for _, equalField := range formConfig.EqualFields {
		if fieldValues[equalField.Left] != fieldValues[equalField.Right] {
			message := fmt.Sprintf(
				"%sと%sが一致しません。",
				fieldLabel(formConfig, equalField.Left),
				fieldLabel(formConfig, equalField.Right),
			)
			formErrors.Add(equalField.Right, message)
		}
	}

	return formErrors
}

/**
 * 表示名取得。
 *
 * 設定された日本語項目名を優先して返す処理。
 */
func fieldLabel(formConfig config.FormConfig, fieldName string) string {
	if label := strings.TrimSpace(formConfig.FieldLabels[fieldName]); label != "" {
		return label
	}

	return fieldName
}

/**
 * メールアドレス形式判定。
 *
 * 単独メールアドレスとして解析できるかを返す処理。
 */
func isEmailAddress(value string) bool {
	address, err := mail.ParseAddress(value)
	if err != nil {
		return false
	}

	return address.Address == value
}

/**
 * 項目順序生成。
 *
 * F2M_JPNAMEの順序を優先し、残りの項目を安定順で追加する処理。
 */
func orderedFieldNames(fieldOrder []string, fieldSet map[string]bool) []string {
	fieldNames := make([]string, 0, len(fieldSet))
	seen := make(map[string]bool)

	for _, fieldName := range fieldOrder {
		if fieldSet[fieldName] {
			fieldNames = append(fieldNames, fieldName)
			seen[fieldName] = true
		}
	}

	remainingFieldNames := make([]string, 0)
	for fieldName := range fieldSet {
		if !seen[fieldName] {
			remainingFieldNames = append(remainingFieldNames, fieldName)
		}
	}

	sort.Strings(remainingFieldNames)
	fieldNames = append(fieldNames, remainingFieldNames...)

	return fieldNames
}
