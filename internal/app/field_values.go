package app

import "strings"

/**
 * フォーム入力値集合。
 *
 * 項目名ごとにPOSTされた値を順序付きで保持する値。
 */
type FieldValues map[string][]string

/**
 * 先頭入力値取得。
 *
 * 単一値として扱う項目の先頭値を返す処理。
 */
func (fieldValues FieldValues) First(fieldName string) string {
	values := fieldValues[fieldName]
	if len(values) == 0 {
		return ""
	}

	return values[0]
}

/**
 * 入力値結合。
 *
 * 複数入力値を画面表示とCSV保存用の文字列へ変換する処理。
 */
func (fieldValues FieldValues) Joined(fieldName string) string {
	return joinFieldValues(fieldValues[fieldName])
}

/**
 * 入力値存在判定。
 *
 * 空白以外の入力値が1つ以上あるかを返す処理。
 */
func (fieldValues FieldValues) HasAny(fieldName string) bool {
	for _, fieldValue := range fieldValues[fieldName] {
		if strings.TrimSpace(fieldValue) != "" {
			return true
		}
	}

	return false
}

/**
 * 入力値含有判定。
 *
 * 指定値が入力値集合に含まれるかを返す処理。
 */
func (fieldValues FieldValues) Contains(fieldName string, expectedValue string) bool {
	for _, fieldValue := range fieldValues[fieldName] {
		if fieldValue == expectedValue {
			return true
		}
	}

	return false
}

/**
 * 入力値スライス複製。
 *
 * 外部変更を避けるために入力値のスライスを複製する処理。
 */
func (fieldValues FieldValues) CloneValues(fieldName string) []string {
	values := fieldValues[fieldName]
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
