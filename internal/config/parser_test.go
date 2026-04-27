package config

// このファイルは、config package のテストを担当する。
// *_test.go は通常ビルドに含まれず、go test 実行時のみ利用される検証コードである。

import (
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
