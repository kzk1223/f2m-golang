package mailer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"f2m-golang/internal/attachment"
	"f2m-golang/internal/config"
)

/**
 * 管理者通知と自動返信の送信確認。
 *
 * 設定と入力値から2通のメールが生成されることを検証する。
 */
func TestServiceSendsAdminAndReplyMessages(t *testing.T) {
	templateDir := t.TempDir()
	mailTemplatePath := writeMailerTestTemplate(t, templateDir, "mail.txt", `MAIL {{.FormID}}
{{range .Fields}}{{.Label}}={{.Value}}
{{end}}`)
	replyTemplatePath := writeMailerTestTemplate(t, templateDir, "reply.txt", `REPLY {{.First "name"}}
{{range .Fields}}{{.Name}}={{.Value}}
{{end}}`)

	formConfig := config.FormConfig{
		ID:            "contact",
		From:          "form@example.com",
		To:            []string{"admin@example.com"},
		Subject:       "管理者通知",
		FieldOrder:    []string{"name", "mail", "interest"},
		FieldLabels:   map[string]string{"name": "お名前", "mail": "メールアドレス", "interest": "興味のある内容", "attachment": "添付ファイル"},
		ReplyToField:  "mail",
		ReplySubject:  "自動返信",
		ReplyTemplate: replyTemplatePath,
		MailTemplate:  mailTemplatePath,
		AttachFields:  []string{"attachment"},
	}
	fieldValues := map[string][]string{
		"name":     {"山田太郎"},
		"mail":     {"taro@example.com"},
		"interest": {"資料請求", "見積依頼"},
	}
	submitMeta := SubmitMeta{
		SentAt:        time.Date(2026, 5, 6, 1, 2, 3, 0, time.Local),
		RemoteIP:      "203.0.113.10",
		XForwardedFor: "198.51.100.1, 198.51.100.2",
		XRealIP:       "198.51.100.1",
	}
	attachmentFiles := []attachment.File{
		{ID: "f2m_att_test", FieldName: "attachment", OriginalName: "document.txt", StoredPath: filepath.Join(templateDir, "document.txt"), Size: 5},
	}
	sender := &recordingSender{}
	service := NewService(sender)

	if err := service.SendAll(formConfig, fieldValues, attachmentFiles, submitMeta); err != nil {
		t.Fatal(err)
	}

	if len(sender.messages) != 2 {
		t.Fatalf("messages length = %d, want 2", len(sender.messages))
	}
	assertMailerMessage(t, sender.messages[0], messageTypeAdmin, []string{"admin@example.com"}, "管理者通知", "興味のある内容=資料請求、見積依頼", "添付ファイル=document.txt", "送信日時: 2026-05-06 01:02:03", "送信元IP: 203.0.113.10", "X-Forwarded-For: 198.51.100.1, 198.51.100.2", "X-Real-IP: 198.51.100.1")
	assertMailerMessage(t, sender.messages[1], messageTypeReply, []string{"taro@example.com"}, "自動返信", "REPLY 山田太郎", "attachment=document.txt")
	if len(sender.messages[0].Attachments) != 1 {
		t.Fatalf("admin attachments length = %d, want 1", len(sender.messages[0].Attachments))
	}
	if len(sender.messages[1].Attachments) != 0 {
		t.Fatalf("reply attachments length = %d, want 0", len(sender.messages[1].Attachments))
	}
	if strings.Contains(sender.messages[1].Body, "送信元IP") {
		t.Fatalf("reply body = %q, want no submit meta", sender.messages[1].Body)
	}
}

/**
 * メールヘッダー改行拒否の確認。
 *
 * Subjectに改行が含まれる場合に送信を拒否することを検証する。
 */
func TestServiceRejectsHeaderLineBreak(t *testing.T) {
	service := NewService(&recordingSender{})
	formConfig := config.FormConfig{
		From:    "form@example.com",
		To:      []string{"admin@example.com"},
		Subject: "件名\r\nBcc: attacker@example.com",
	}

	err := service.SendAll(formConfig, map[string][]string{}, nil, SubmitMeta{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Subject contains line break") {
		t.Fatalf("error = %q", err.Error())
	}
}

/**
 * テスト用テンプレート書き込み。
 *
 * 一時ディレクトリへメールテンプレートを配置する処理。
 */
func writeMailerTestTemplate(t *testing.T, templateDir string, fileName string, templateText string) string {
	t.Helper()

	templatePath := filepath.Join(templateDir, fileName)
	if err := os.WriteFile(templatePath, []byte(templateText), 0644); err != nil {
		t.Fatal(err)
	}

	return templatePath
}

/**
 * テスト用送信処理。
 *
 * 送信されたメールをメモリ上に記録する処理。
 */
type recordingSender struct {
	messages []Message
}

/**
 * テスト用メール記録。
 *
 * 送信メールを順序付きで保持する処理。
 */
func (sender *recordingSender) Send(_ config.FormConfig, message Message) error {
	sender.messages = append(sender.messages, message)

	return nil
}

/**
 * 送信メール検証。
 *
 * 種別、宛先、件名、本文を確認する処理。
 */
func assertMailerMessage(t *testing.T, message Message, expectedType MessageType, expectedTo []string, expectedSubject string, expectedBodies ...string) {
	t.Helper()

	if message.Type != expectedType {
		t.Fatalf("message type = %q, want %q", message.Type, expectedType)
	}
	if strings.Join(message.To, "\x00") != strings.Join(expectedTo, "\x00") {
		t.Fatalf("message to = %#v, want %#v", message.To, expectedTo)
	}
	if message.Subject != expectedSubject {
		t.Fatalf("message subject = %q, want %q", message.Subject, expectedSubject)
	}
	for _, expectedBody := range expectedBodies {
		if !strings.Contains(message.Body, expectedBody) {
			t.Fatalf("message body = %q, want contains %q", message.Body, expectedBody)
		}
	}
}
