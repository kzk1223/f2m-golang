package storage

// このファイルは、storage package のCSV保存処理を確認するテストである。
// 実行方法: go test ./internal/storage

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"f2m-golang/internal/charset"
	"f2m-golang/internal/config"
)

/**
 * CSV初回保存の確認。
 *
 * ヘッダー行と入力値行が項目順で保存されることを検証する。
 */
func TestAppendCSVWritesHeaderAndData(t *testing.T) {
	csvPath := filepath.Join(t.TempDir(), "contact.csv")
	formConfig := newCSVTestFormConfig(csvPath, "UTF-8")

	err := AppendCSV(formConfig, map[string][]string{
		"name":    {"山田太郎"},
		"mail":    {"taro@example.com"},
		"contact": {"改行\nカンマ,引用\""},
	}, newCSVTestSubmitMeta())
	if err != nil {
		t.Fatal(err)
	}

	assertCSVRecords(t, csvPath, [][]string{
		{"お名前", "メールアドレス", "お問い合わせ内容", "送信日時", "送信元IP", "X-Forwarded-For", "X-Real-IP"},
		{"山田太郎", "taro@example.com", "改行\nカンマ,引用\"", "2026-05-06 12:34:56", "203.0.113.10", "198.51.100.1, 198.51.100.2", "198.51.100.1"},
	})
}

/**
 * CSV追記保存の確認。
 *
 * 2回目以降の保存でヘッダー行が重複しないことを検証する。
 */
func TestAppendCSVAppendsWithoutDuplicatingHeader(t *testing.T) {
	csvPath := filepath.Join(t.TempDir(), "contact.csv")
	formConfig := newCSVTestFormConfig(csvPath, "UTF-8")

	if err := AppendCSV(formConfig, map[string][]string{"name": {"山田太郎"}, "mail": {"taro@example.com"}, "contact": {"初回"}}, newCSVTestSubmitMeta()); err != nil {
		t.Fatal(err)
	}
	if err := AppendCSV(formConfig, map[string][]string{"name": {"佐藤花子"}, "mail": {"hanako@example.com"}, "contact": {"追記"}}, newCSVTestSubmitMeta()); err != nil {
		t.Fatal(err)
	}

	assertCSVRecords(t, csvPath, [][]string{
		{"お名前", "メールアドレス", "お問い合わせ内容", "送信日時", "送信元IP", "X-Forwarded-For", "X-Real-IP"},
		{"山田太郎", "taro@example.com", "初回", "2026-05-06 12:34:56", "203.0.113.10", "198.51.100.1, 198.51.100.2", "198.51.100.1"},
		{"佐藤花子", "hanako@example.com", "追記", "2026-05-06 12:34:56", "203.0.113.10", "198.51.100.1, 198.51.100.2", "198.51.100.1"},
	})
}

/**
 * CSV未設定時の確認。
 *
 * CSVパスが空の場合に保存処理が何もしないことを検証する。
 */
func TestAppendCSVSkipsEmptyPath(t *testing.T) {
	err := AppendCSV(config.FormConfig{}, map[string][]string{"name": {"山田太郎"}}, CSVSubmitMeta{})
	if err != nil {
		t.Fatal(err)
	}
}

/**
 * Shift_JIS保存の確認。
 *
 * F2M_CSV_CHARSETがSJISの場合にUTF-8以外のバイト列で保存されることを検証する。
 */
func TestAppendCSVWritesShiftJIS(t *testing.T) {
	csvPath := filepath.Join(t.TempDir(), "contact.csv")
	formConfig := newCSVTestFormConfig(csvPath, "SJIS")

	if err := AppendCSV(formConfig, map[string][]string{"name": {"山田太郎"}, "mail": {"taro@example.com"}, "contact": {"確認"}}, newCSVTestSubmitMeta()); err != nil {
		t.Fatal(err)
	}

	rawCSVBytes, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatal(err)
	}
	if utf8.Valid(rawCSVBytes) {
		t.Fatalf("csv bytes are valid UTF-8, want Shift_JIS bytes")
	}

	decodedCSVText, _, err := charset.Decode(rawCSVBytes)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(decodedCSVText, "山田太郎") {
		t.Fatalf("decoded csv = %q, want contains %q", decodedCSVText, "山田太郎")
	}
}

/**
 * CSV複数値保存の確認。
 *
 * checkbox相当の複数値が読点区切りで1セルに保存されることを検証する。
 */
func TestAppendCSVJoinsMultipleValues(t *testing.T) {
	csvPath := filepath.Join(t.TempDir(), "survey.csv")
	formConfig := config.FormConfig{
		FieldOrder:  []string{"interest"},
		FieldLabels: map[string]string{"interest": "興味のある内容"},
		CSVPath:     csvPath,
		CSVCharset:  "UTF-8",
	}

	if err := AppendCSV(formConfig, map[string][]string{"interest": {"資料請求", "見積依頼"}}, newCSVTestSubmitMeta()); err != nil {
		t.Fatal(err)
	}

	assertCSVRecords(t, csvPath, [][]string{
		{"興味のある内容", "送信日時", "送信元IP", "X-Forwarded-For", "X-Real-IP"},
		{"資料請求、見積依頼", "2026-05-06 12:34:56", "203.0.113.10", "198.51.100.1, 198.51.100.2", "198.51.100.1"},
	})
}

/**
 * テスト用フォーム設定生成。
 *
 * CSV保存に必要な最小フォーム設定を返す処理。
 */
func newCSVTestFormConfig(csvPath string, csvCharset string) config.FormConfig {
	return config.FormConfig{
		FieldOrder:  []string{"name", "mail", "contact"},
		FieldLabels: map[string]string{"name": "お名前", "mail": "メールアドレス", "contact": "お問い合わせ内容"},
		CSVPath:     csvPath,
		CSVCharset:  csvCharset,
	}
}

/**
 * テスト用CSV送信メタ情報生成。
 *
 * CSVの付加情報列を固定値で検証するための値を返す処理。
 */
func newCSVTestSubmitMeta() CSVSubmitMeta {
	return CSVSubmitMeta{
		SentAt:        time.Date(2026, 5, 6, 12, 34, 56, 0, time.FixedZone("JST", 9*60*60)),
		RemoteIP:      "203.0.113.10",
		XForwardedFor: "198.51.100.1, 198.51.100.2",
		XRealIP:       "198.51.100.1",
	}
}

/**
 * CSVレコード検証。
 *
 * 保存済みCSVを読み込み、期待レコード群と一致することを検証する。
 */
func assertCSVRecords(t *testing.T, csvPath string, expectedRecords [][]string) {
	t.Helper()

	csvBytes, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatal(err)
	}

	csvReader := csv.NewReader(strings.NewReader(string(csvBytes)))
	actualRecords, err := csvReader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(actualRecords, expectedRecords) {
		t.Fatalf("records = %#v, want %#v", actualRecords, expectedRecords)
	}
}
