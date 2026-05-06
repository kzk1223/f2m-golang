package config

// このファイルは、config package のテストを担当する。
// *_test.go は通常ビルドに含まれず、go test 実行時のみ利用される検証コードである。
// 主な検証対象:
//   Parseによる設定文字列の正規化。
//   F2M_ID単位の複数フォーム分離。
//   不正設定に対する行番号付きエラー。
//   LoadFileによる実設定ファイルの文字コード判定込み読み込み。
// テスト実行例:
//   $env:GOCACHE = (Join-Path (Get-Location) ".gocache"); go test ./...
//   $env:GOCACHE = (Join-Path (Get-Location) ".gocache"); go test ./internal/config

import (
	"path/filepath"
	"strings"
	"testing"
)

/**
 * 設定解析の基本動作検証。
 *
 * 旧f2m互換キーの正規化確認。
 */
func TestParseBasicConfig(t *testing.T) {
	configText := `
# comment
F2M_ID : contact
F2M_FROM : form@example.com
F2M_TO : admin@example.com, support@example.com
F2M_SUBJECT : お問い合わせ
F2M_CHK : name,mail
F2M_JPNAME : name:お名前,mail:メールアドレス
F2M_CHK_EMAIL : mail
F2M_CHK_EQ : mail:mail2
F2M_CSV_CHARSET : CP932
F2M_TOKEN_EXPIRE : 15m
F2M_HONEYPOT : on
F2M_FORMERR : old-error.html
`

	configSet, err := Parse(configText)
	if err != nil {
		t.Fatal(err)
	}

	formConfig := configSet.Forms["contact"]

	if formConfig.ID != "contact" {
		t.Fatalf("ID = %q", formConfig.ID)
	}
	if len(formConfig.To) != 2 {
		t.Fatalf("To length = %d", len(formConfig.To))
	}
	if !formConfig.RequiredFields["name"] {
		t.Fatal("name required field is false")
	}
	if formConfig.FieldLabels["mail"] != "メールアドレス" {
		t.Fatalf("mail label = %q", formConfig.FieldLabels["mail"])
	}
	if len(formConfig.FieldOrder) != 2 {
		t.Fatalf("FieldOrder length = %d", len(formConfig.FieldOrder))
	}
	if len(formConfig.EqualFields) != 1 {
		t.Fatalf("EqualFields length = %d", len(formConfig.EqualFields))
	}
	if formConfig.CSVCharset != "CP932" {
		t.Fatalf("CSVCharset = %q", formConfig.CSVCharset)
	}
	if !formConfig.HoneypotEnabled {
		t.Fatal("HoneypotEnabled is false")
	}
	if !strings.HasPrefix(formConfig.HoneypotField, "f2m_hp_") {
		t.Fatalf("HoneypotField = %q", formConfig.HoneypotField)
	}
}

/**
 * 複数フォーム設定の検証。
 *
 * F2M_ID単位の設定分離確認。
 */
func TestParseMultipleForms(t *testing.T) {
	configText := `
F2M_ID : contact
F2M_FROM : contact@example.com
F2M_ID : recruit
F2M_FROM : recruit@example.com
`

	configSet, err := Parse(configText)
	if err != nil {
		t.Fatal(err)
	}

	if configSet.Forms["contact"].From != "contact@example.com" {
		t.Fatalf("contact From = %q", configSet.Forms["contact"].From)
	}
	if configSet.Forms["recruit"].From != "recruit@example.com" {
		t.Fatalf("recruit From = %q", configSet.Forms["recruit"].From)
	}
}

/**
 * F2M_ID未定義時の検証。
 *
 * フォーム識別子なし設定の拒否。
 */
func TestParseRejectsMissingFormID(t *testing.T) {
	_, err := Parse(`F2M_FROM : form@example.com`)
	if err == nil {
		t.Fatal("expected error")
	}
}

/**
 * 設定形式エラーの検証。
 *
 * 行番号付きエラーメッセージの確認。
 */
func TestParseReturnsLineNumber(t *testing.T) {
	_, err := Parse(`
F2M_ID : contact
F2M_CHK_EQ : mail
`)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "3 行目") {
		t.Fatalf("error = %q", err.Error())
	}
}

/**
 * 実設定ファイル読み込みの検証。
 *
 * f2m_conf.txtをLoadFile経由で解析する確認。
 */
func TestLoadFileSampleConfig(t *testing.T) {
	configPath := filepath.Join("..", "..", "f2m_conf.txt")

	configSet, err := LoadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	formConfig, exists := configSet.Forms["contact"]
	if !exists {
		t.Fatal("contact config not found")
	}

	t.Logf("ID = %s", formConfig.ID)
	t.Logf("From = %s", formConfig.From)
	t.Logf("To = %v", formConfig.To)
	t.Logf("Subject = %s", formConfig.Subject)
	t.Logf("FieldOrder = %v", formConfig.FieldOrder)
	t.Logf("FieldLabels = %v", formConfig.FieldLabels)
	t.Logf("RequiredFields = %v", formConfig.RequiredFields)
	t.Logf("EmailFields = %v", formConfig.EmailFields)
	t.Logf("EqualFields = %+v", formConfig.EqualFields)
	t.Logf("ReplyToField = %s", formConfig.ReplyToField)
	t.Logf("ReplySubject = %s", formConfig.ReplySubject)
	t.Logf("ReplyTemplate = %s", formConfig.ReplyTemplate)
	t.Logf("MailTemplate = %s", formConfig.MailTemplate)
	t.Logf("FormPath = %s", formConfig.FormPath)
	t.Logf("ConfirmPath = %s", formConfig.ConfirmPath)
	t.Logf("ThanksPath = %s", formConfig.ThanksPath)
	t.Logf("CSVPath = %s", formConfig.CSVPath)
	t.Logf("CSVCharset = %s", formConfig.CSVCharset)
	t.Logf("AttachFields = %v", formConfig.AttachFields)
	t.Logf("TokenExpire = %s", formConfig.TokenExpire)
	t.Logf("HoneypotEnabled = %t", formConfig.HoneypotEnabled)
	t.Logf("HoneypotField = %s", formConfig.HoneypotField)

	if formConfig.FormPath != "./form.html" {
		t.Fatalf("FormPath = %q", formConfig.FormPath)
	}
	if formConfig.ConfirmPath != "./templates/confirm.html" {
		t.Fatalf("ConfirmPath = %q", formConfig.ConfirmPath)
	}
	if formConfig.ThanksPath != "./templates/thanks.html" {
		t.Fatalf("ThanksPath = %q", formConfig.ThanksPath)
	}
	if !formConfig.RequiredFields["contact"] {
		t.Fatal("contact required field is false")
	}
	if !formConfig.EmailFields["mail"] {
		t.Fatal("mail email field is false")
	}
	if len(formConfig.EqualFields) != 1 {
		t.Fatalf("EqualFields length = %d", len(formConfig.EqualFields))
	}
	if formConfig.CSVCharset != "SJIS" {
		t.Fatalf("CSVCharset = %q", formConfig.CSVCharset)
	}
	if len(formConfig.AttachFields) != 1 || formConfig.AttachFields[0] != "attachment" {
		t.Fatalf("AttachFields = %#v", formConfig.AttachFields)
	}
	if !formConfig.HoneypotEnabled {
		t.Fatal("HoneypotEnabled is false")
	}
	if !strings.HasPrefix(formConfig.HoneypotField, "f2m_hp_") {
		t.Fatalf("HoneypotField = %q", formConfig.HoneypotField)
	}
}
