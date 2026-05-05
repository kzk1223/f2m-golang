// Package mailer は、フォーム送信メールの生成と送信を扱うpackageである。
package mailer

import (
	"bytes"
	"fmt"
	"net/mail"
	"strings"
	"text/template"
	"time"

	"f2m-golang/internal/config"
)

const (
	messageTypeAdmin       MessageType = "admin"
	messageTypeReply       MessageType = "reply"
	submitMetaSentAtLayout             = "2006-01-02 15:04:05"
)

/**
 * メール種別。
 *
 * 管理者通知と自動返信を区別する値。
 */
type MessageType string

/**
 * 送信メール。
 *
 * SMTP送信前に正規化されたメール内容。
 */
type Message struct {
	Type    MessageType
	From    string
	To      []string
	Subject string
	Body    string
}

/**
 * 送信メタ情報。
 *
 * 管理者通知メールへ自動追記する送信時の付加情報。
 */
type SubmitMeta struct {
	SentAt        time.Time
	RemoteIP      string
	XForwardedFor string
	XRealIP       string
}

/**
 * メール送信処理。
 *
 * SMTP実装やテスト用実装を差し替えるための境界。
 */
type Sender interface {
	Send(formConfig config.FormConfig, message Message) error
}

/**
 * メール送信サービス。
 *
 * 設定と入力値から管理者通知と自動返信を生成し送信する処理。
 */
type Service struct {
	sender Sender
}

/**
 * メール送信サービス生成。
 *
 * 指定された送信処理を保持するサービスを返す処理。
 */
func NewService(sender Sender) *Service {
	return &Service{
		sender: sender,
	}
}

/**
 * フォームメール送信。
 *
 * 設定されている管理者通知と自動返信を順番に送信する処理。
 */
func (service *Service) SendAll(formConfig config.FormConfig, fieldValues map[string][]string, submitMeta SubmitMeta) error {
	if service == nil || service.sender == nil {
		return nil
	}
	if !hasConfiguredMail(formConfig) {
		return nil
	}
	if err := validateMailSender(formConfig); err != nil {
		return err
	}

	// ---------------------------------------------
	// 管理者通知
	// ---------------------------------------------
	if hasAdminMail(formConfig) {
		adminMessage, err := newAdminMessage(formConfig, fieldValues, submitMeta)
		if err != nil {
			return err
		}
		if err := service.sender.Send(formConfig, adminMessage); err != nil {
			return fmt.Errorf("send admin mail: %w", err)
		}
	}

	// ---------------------------------------------
	// 自動返信
	// ---------------------------------------------
	if hasReplyMail(formConfig) {
		replyMessage, err := newReplyMessage(formConfig, fieldValues)
		if err != nil {
			return err
		}
		if replyMessage.To != nil {
			if err := service.sender.Send(formConfig, replyMessage); err != nil {
				return fmt.Errorf("send reply mail: %w", err)
			}
		}
	}

	return nil
}

/**
 * 管理者通知メール生成。
 *
 * F2M_TO宛ての通知メールを生成する処理。
 */
func newAdminMessage(formConfig config.FormConfig, fieldValues map[string][]string, submitMeta SubmitMeta) (Message, error) {
	body, err := renderMailBody(formConfig.MailTemplate, formConfig, fieldValues)
	if err != nil {
		return Message{}, fmt.Errorf("render admin mail: %w", err)
	}
	body = appendAdminSubmitMeta(body, submitMeta)

	return newMessage(messageTypeAdmin, formConfig.From, formConfig.To, formConfig.Subject, body)
}

/**
 * 自動返信メール生成。
 *
 * F2M_RESV_TO_FLDで指定された入力値宛ての返信メールを生成する処理。
 */
func newReplyMessage(formConfig config.FormConfig, fieldValues map[string][]string) (Message, error) {
	replyTo := strings.TrimSpace(firstFieldValue(fieldValues, formConfig.ReplyToField))
	if replyTo == "" {
		return Message{Type: messageTypeReply}, nil
	}

	body, err := renderMailBody(formConfig.ReplyTemplate, formConfig, fieldValues)
	if err != nil {
		return Message{}, fmt.Errorf("render reply mail: %w", err)
	}

	return newMessage(messageTypeReply, formConfig.From, []string{replyTo}, formConfig.ReplySubject, body)
}

/**
 * メール内容生成。
 *
 * ヘッダー値と宛先を検証した送信メールを返す処理。
 */
func newMessage(messageType MessageType, from string, recipients []string, subject string, body string) (Message, error) {
	normalizedFrom, err := normalizeAddress("From", from)
	if err != nil {
		return Message{}, err
	}

	normalizedRecipients, err := normalizeAddressList("To", recipients)
	if err != nil {
		return Message{}, err
	}

	normalizedSubject, err := normalizeHeaderValue("Subject", subject)
	if err != nil {
		return Message{}, err
	}

	return Message{
		Type:    messageType,
		From:    normalizedFrom,
		To:      normalizedRecipients,
		Subject: normalizedSubject,
		Body:    body,
	}, nil
}

/**
 * メール本文描画。
 *
 * text/templateでメール本文を生成する処理。
 */
func renderMailBody(templatePath string, formConfig config.FormConfig, fieldValues map[string][]string) (string, error) {
	view := newTemplateView(formConfig, fieldValues)
	if strings.TrimSpace(templatePath) == "" {
		return defaultMailBody(view), nil
	}

	mailTemplate, err := template.ParseFiles(templatePath)
	if err != nil {
		return "", err
	}

	var bodyBuffer bytes.Buffer
	if err := mailTemplate.Execute(&bodyBuffer, view); err != nil {
		return "", err
	}

	return bodyBuffer.String(), nil
}

/**
 * 既定メール本文生成。
 *
 * テンプレート未指定時に項目一覧の本文を生成する処理。
 */
func defaultMailBody(view TemplateView) string {
	var bodyBuilder strings.Builder
	for _, field := range view.Fields {
		bodyBuilder.WriteString(field.Label)
		bodyBuilder.WriteString(": ")
		bodyBuilder.WriteString(field.Value)
		bodyBuilder.WriteString("\n")
	}

	return bodyBuilder.String()
}

/**
 * 管理者通知メタ情報追記。
 *
 * 管理者通知メール本文の末尾へ送信付加情報を自動追加する処理。
 */
func appendAdminSubmitMeta(body string, submitMeta SubmitMeta) string {
	var bodyBuilder strings.Builder
	bodyBuilder.WriteString(strings.TrimRight(body, "\r\n"))
	bodyBuilder.WriteString("\n\n---\n")
	bodyBuilder.WriteString("送信日時: ")
	bodyBuilder.WriteString(formatSubmitMetaSentAt(submitMeta.SentAt))
	bodyBuilder.WriteString("\n")
	bodyBuilder.WriteString("送信元IP: ")
	bodyBuilder.WriteString(submitMeta.RemoteIP)
	bodyBuilder.WriteString("\n")
	bodyBuilder.WriteString("X-Forwarded-For: ")
	bodyBuilder.WriteString(submitMeta.XForwardedFor)
	bodyBuilder.WriteString("\n")
	bodyBuilder.WriteString("X-Real-IP: ")
	bodyBuilder.WriteString(submitMeta.XRealIP)
	bodyBuilder.WriteString("\n")

	return bodyBuilder.String()
}

/**
 * 送信日時整形。
 *
 * 送信日時を管理者通知メール用の固定形式へ変換する処理。
 */
func formatSubmitMetaSentAt(sentAt time.Time) string {
	if sentAt.IsZero() {
		return ""
	}

	return sentAt.Format(submitMetaSentAtLayout)
}

/**
 * メール設定有無判定。
 *
 * 管理者通知または自動返信が設定されているかを返す処理。
 */
func hasConfiguredMail(formConfig config.FormConfig) bool {
	return hasAdminMail(formConfig) || hasReplyMail(formConfig)
}

/**
 * 管理者通知設定判定。
 *
 * F2M_TOに宛先が設定されているかを返す処理。
 */
func hasAdminMail(formConfig config.FormConfig) bool {
	for _, recipient := range formConfig.To {
		if strings.TrimSpace(recipient) != "" {
			return true
		}
	}

	return false
}

/**
 * 自動返信設定判定。
 *
 * F2M_RESV_TO_FLDが設定されているかを返す処理。
 */
func hasReplyMail(formConfig config.FormConfig) bool {
	return strings.TrimSpace(formConfig.ReplyToField) != ""
}

/**
 * 送信方式検証。
 *
 * 現在対応しているメール送信方式かを検証する処理。
 */
func validateMailSender(formConfig config.FormConfig) error {
	mailSender := strings.TrimSpace(formConfig.MailSender)
	if mailSender == "" || strings.EqualFold(mailSender, "smtp") {
		return nil
	}

	return fmt.Errorf("unsupported mail sender: %s", formConfig.MailSender)
}

/**
 * メールアドレスリスト正規化。
 *
 * 空要素を除外し、各宛先の形式を検証する処理。
 */
func normalizeAddressList(headerName string, addresses []string) ([]string, error) {
	normalizedAddresses := make([]string, 0, len(addresses))
	for _, address := range addresses {
		normalizedAddress, err := normalizeAddress(headerName, address)
		if err != nil {
			if strings.TrimSpace(address) == "" {
				continue
			}

			return nil, err
		}
		normalizedAddresses = append(normalizedAddresses, normalizedAddress)
	}
	if len(normalizedAddresses) == 0 {
		return nil, fmt.Errorf("%s is empty", headerName)
	}

	return normalizedAddresses, nil
}

/**
 * メールアドレス正規化。
 *
 * ヘッダー改行混入とメールアドレス形式を検証する処理。
 */
func normalizeAddress(headerName string, address string) (string, error) {
	normalizedAddress, err := normalizeHeaderValue(headerName, address)
	if err != nil {
		return "", err
	}
	if normalizedAddress == "" {
		return "", fmt.Errorf("%s is empty", headerName)
	}
	if _, err := mail.ParseAddress(normalizedAddress); err != nil {
		return "", fmt.Errorf("%s is invalid: %w", headerName, err)
	}

	return normalizedAddress, nil
}

/**
 * ヘッダー値正規化。
 *
 * メールヘッダーへ設定する文字列の改行混入を拒否する処理。
 */
func normalizeHeaderValue(headerName string, headerValue string) (string, error) {
	normalizedHeaderValue := strings.TrimSpace(headerValue)
	if strings.ContainsAny(normalizedHeaderValue, "\r\n") {
		return "", fmt.Errorf("%s contains line break", headerName)
	}

	return normalizedHeaderValue, nil
}

/**
 * 先頭入力値取得。
 *
 * 指定項目の先頭入力値を返す処理。
 */
func firstFieldValue(fieldValues map[string][]string, fieldName string) string {
	values := fieldValues[fieldName]
	if len(values) == 0 {
		return ""
	}

	return values[0]
}
