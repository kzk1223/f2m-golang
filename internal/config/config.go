// Package config は、f2m_conf.txt の設定構造と解析処理を扱うpackageである。
//
// Goでは同じディレクトリ内の .go ファイルが同一packageとして扱われる。
// このpackageは設定値の型定義、設定ファイル解析、既定値管理を担当する。
package config

import "time"

/**
 * 設定集合。
 *
 * F2M_IDをキーにしたフォーム設定群。
 */
type ConfigSet struct {
	Forms map[string]FormConfig
}

/**
 * フォーム設定。
 *
 * f2m_conf.txtの1フォーム分を正規化した構造。
 */
type FormConfig struct {
	ID string

	From    string
	To      []string
	Subject string

	RequiredFields map[string]bool
	EmailFields    map[string]bool
	EqualFields    []EqualField

	FieldLabels map[string]string
	FieldOrder  []string

	ReplyToField  string
	ReplySubject  string
	ReplyTemplate string
	MailTemplate  string

	CSVPath    string
	CSVCharset string

	FormPath    string
	ConfirmPath string
	ThanksPath  string

	AttachFields  []string
	AttachMax     string
	AttachExts    []string
	AttachMIMEs   []string
	AttachTempTTL time.Duration

	MailSender   string
	SMTPHost     string
	SMTPPort     string
	SMTPAuth     bool
	SMTPUser     string
	SMTPPassword string

	InputCharset  string
	OutputCharset string

	TokenExpire     time.Duration
	HoneypotEnabled bool
	HoneypotField   string
	RateLimit       time.Duration

	LogPath string
}

/**
 * 一致チェック定義。
 *
 * 左右フィールドの入力値一致条件。
 */
type EqualField struct {
	Left  string
	Right string
}
