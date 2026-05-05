package mailer

import (
	"fmt"
	"strconv"
	"strings"

	gomail "github.com/wneessen/go-mail"

	"f2m-golang/internal/config"
)

const (
	defaultSMTPHost = "localhost"
	defaultSMTPPort = 25
)

/**
 * SMTPメール送信処理。
 *
 * go-mailを使ってSMTPサーバーへメールを送信する処理。
 */
type GoMailSender struct{}

/**
 * SMTPメール送信処理生成。
 *
 * go-mailを使う送信処理を返す。
 */
func NewGoMailSender() *GoMailSender {
	return &GoMailSender{}
}

/**
 * SMTPメール送信。
 *
 * フォーム設定のSMTP情報を使ってメールを送信する処理。
 */
func (sender *GoMailSender) Send(formConfig config.FormConfig, message Message) error {
	client, err := newGoMailClient(formConfig)
	if err != nil {
		return err
	}

	goMailMessage := gomail.NewMsg()
	if err := goMailMessage.From(message.From); err != nil {
		return fmt.Errorf("set from: %w", err)
	}
	if err := goMailMessage.To(message.To...); err != nil {
		return fmt.Errorf("set to: %w", err)
	}
	goMailMessage.Subject(message.Subject)
	goMailMessage.SetBodyString(gomail.TypeTextPlain, message.Body)

	if err := client.DialAndSend(goMailMessage); err != nil {
		return fmt.Errorf("smtp send: %w", err)
	}

	return nil
}

/**
 * SMTPクライアント生成。
 *
 * フォーム設定からgo-mailクライアントを生成する処理。
 */
func newGoMailClient(formConfig config.FormConfig) (*gomail.Client, error) {
	smtpHost := strings.TrimSpace(formConfig.SMTPHost)
	if smtpHost == "" {
		smtpHost = defaultSMTPHost
	}

	smtpPort, err := smtpPort(formConfig.SMTPPort)
	if err != nil {
		return nil, err
	}

	clientOptions := []gomail.Option{
		gomail.WithPort(smtpPort),
		gomail.WithTLSPolicy(gomail.TLSOpportunistic),
	}

	if formConfig.SMTPAuth {
		if strings.TrimSpace(formConfig.SMTPUser) == "" {
			return nil, fmt.Errorf("SMTP user is empty")
		}

		clientOptions = append(
			clientOptions,
			gomail.WithSMTPAuth(gomail.SMTPAuthAutoDiscover),
			gomail.WithUsername(formConfig.SMTPUser),
			gomail.WithPassword(formConfig.SMTPPassword),
		)
	}

	client, err := gomail.NewClient(smtpHost, clientOptions...)
	if err != nil {
		return nil, fmt.Errorf("new smtp client: %w", err)
	}

	return client, nil
}

/**
 * SMTPポート取得。
 *
 * 設定値が空の場合は既定ポートを返す処理。
 */
func smtpPort(portText string) (int, error) {
	trimmedPortText := strings.TrimSpace(portText)
	if trimmedPortText == "" {
		return defaultSMTPPort, nil
	}

	port, err := strconv.Atoi(trimmedPortText)
	if err != nil {
		return 0, fmt.Errorf("SMTP port is invalid: %w", err)
	}
	if port <= 0 {
		return 0, fmt.Errorf("SMTP port is invalid: %d", port)
	}

	return port, nil
}
