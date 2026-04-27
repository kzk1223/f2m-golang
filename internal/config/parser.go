package config

// このファイルは、config package の設定解析処理を担当する。
// config.go に定義されたstructへ、テキスト設定を正規化して詰め替える処理群である。

import (
	"bufio"
	"fmt"
	"regexp"
	"strings"
	"time"

	"f2m-golang/internal/charset"
)

var configKeyPattern = regexp.MustCompile(`^[A-Z0-9_]+$`)

/**
 * 設定ファイル読み込み。
 *
 * 文字コード判定後、フォーム設定集合として解析する処理。
 */
func LoadFile(path string) (*ConfigSet, error) {
	configText, _, err := charset.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return Parse(configText)
}

/**
 * 設定テキスト解析。
 *
 * KEY : VALUE形式をF2M_ID単位の設定に変換する処理。
 */
func Parse(configText string) (*ConfigSet, error) {
	configSet := &ConfigSet{
		Forms: make(map[string]FormConfig),
	}

	var current *FormConfig

	scanner := bufio.NewScanner(strings.NewReader(configText))
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		rawLine := scanner.Text()
		line := strings.TrimSpace(rawLine)

		// ---------------------------------------------
		// 空行・コメント行
		// ---------------------------------------------
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, ":")
		if !ok {
			return nil, parseError(lineNumber, "区切り文字 ':' がありません")
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		if !configKeyPattern.MatchString(key) {
			return nil, parseError(lineNumber, "設定キーの形式が不正です")
		}

		if key == "F2M_ID" {
			if value == "" {
				return nil, parseError(lineNumber, "F2M_ID が空です")
			}

			formConfig := newDefaultFormConfig(value)
			configSet.Forms[value] = formConfig
			current = &formConfig
			continue
		}

		if current == nil {
			return nil, parseError(lineNumber, "F2M_ID より前に設定項目があります")
		}

		if err := applyValue(current, key, value); err != nil {
			return nil, parseError(lineNumber, err.Error())
		}

		configSet.Forms[current.ID] = *current
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if len(configSet.Forms) == 0 {
		return nil, fmt.Errorf("設定に F2M_ID がありません")
	}

	return configSet, nil
}

/**
 * フォーム設定の初期値生成。
 *
 * Go版で追加された既定値の集約。
 */
func newDefaultFormConfig(id string) FormConfig {
	return FormConfig{
		ID: id,

		RequiredFields: make(map[string]bool),
		EmailFields:    make(map[string]bool),
		FieldLabels:    make(map[string]string),

		CSVCharset: "SJIS",

		AttachMax:     "3M",
		AttachTempTTL: time.Hour,

		InputCharset:  "auto",
		OutputCharset: "UTF-8",

		CSRF:        true,
		TokenExpire: 30 * time.Minute,
		RateLimit:   time.Minute,
	}
}

/**
 * 設定値反映。
 *
 * キー別の型変換と正規化処理。
 */
func applyValue(formConfig *FormConfig, key string, value string) error {
	switch key {
	case "F2M_FROM":
		formConfig.From = value
	case "F2M_TO":
		formConfig.To = splitList(value)
	case "F2M_SUBJECT":
		formConfig.Subject = value
	case "F2M_CHK":
		formConfig.RequiredFields = splitSet(value)
	case "F2M_CHK_EMAIL":
		formConfig.EmailFields = splitSet(value)
	case "F2M_CHK_EQ":
		equalFields, err := parseEqualFields(value)
		if err != nil {
			return err
		}
		formConfig.EqualFields = equalFields
	case "F2M_JPNAME":
		if err := parseFieldLabels(formConfig, value); err != nil {
			return err
		}
	case "F2M_RESV_TO_FLD":
		formConfig.ReplyToField = value
	case "F2M_RESV_SUBJECT":
		formConfig.ReplySubject = value
	case "F2M_RESV_TMPL":
		formConfig.ReplyTemplate = value
	case "F2M_MAIL_TMPL":
		formConfig.MailTemplate = value
	case "F2M_CSV":
		formConfig.CSVPath = value
	case "F2M_CSV_CHARSET":
		formConfig.CSVCharset = strings.ToUpper(value)
	case "F2M_FORM":
		formConfig.FormPath = value
	case "F2M_CONFIRM":
		formConfig.ConfirmPath = value
	case "F2M_THANKS":
		formConfig.ThanksPath = value
	case "F2M_ATTACH_FLD":
		formConfig.AttachFields = splitList(value)
	case "F2M_ATTACH_MAX":
		formConfig.AttachMax = value
	case "F2M_ATTACH_EXT":
		formConfig.AttachExts = splitList(value)
	case "F2M_ATTACH_MIME":
		formConfig.AttachMIMEs = splitList(value)
	case "F2M_ATTACH_TMP_TTL":
		attachTempTTL, err := time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("F2M_ATTACH_TMP_TTL の形式が不正です")
		}
		formConfig.AttachTempTTL = attachTempTTL
	case "F2M_MAIL_SENDER":
		formConfig.MailSender = value
	case "F2M_SMTP_HOST":
		formConfig.SMTPHost = value
	case "F2M_SMTP_PORT":
		formConfig.SMTPPort = value
	case "F2M_SMTP_AUTH":
		formConfig.SMTPAuth = parseBool(value)
	case "F2M_SMTP_USER":
		formConfig.SMTPUser = value
	case "F2M_SMTP_PASSWORD":
		formConfig.SMTPPassword = value
	case "F2M_INPUT_CHARSET":
		formConfig.InputCharset = value
	case "F2M_OUTPUT_CHARSET":
		formConfig.OutputCharset = value
	case "F2M_CSRF":
		formConfig.CSRF = parseBool(value)
	case "F2M_TOKEN_EXPIRE":
		tokenExpire, err := time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("F2M_TOKEN_EXPIRE の形式が不正です")
		}
		formConfig.TokenExpire = tokenExpire
	case "F2M_HONEYPOT":
		formConfig.HoneypotField = value
	case "F2M_RATE_LIMIT":
		rateLimit, err := time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("F2M_RATE_LIMIT の形式が不正です")
		}
		formConfig.RateLimit = rateLimit
	case "F2M_LOG":
		formConfig.LogPath = value
	case "F2M_FORMERR":
		// Go版では廃止。旧設定との互換読み飛ばし。
	default:
		// 未知キーは旧設定の余地を残すため、初期版では許容。
	}

	return nil
}

/**
 * カンマ区切りリスト解析。
 *
 * 空要素を除去した順序付き配列。
 */
func splitList(value string) []string {
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))

	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			items = append(items, item)
		}
	}

	return items
}

/**
 * カンマ区切り集合解析。
 *
 * フィールド存在確認用の集合。
 */
func splitSet(value string) map[string]bool {
	items := make(map[string]bool)

	for _, item := range splitList(value) {
		items[item] = true
	}

	return items
}

/**
 * 表示名定義解析。
 *
 * F2M_JPNAMEの順序と名称を保持する処理。
 */
func parseFieldLabels(formConfig *FormConfig, value string) error {
	for _, item := range splitList(value) {
		name, label, ok := strings.Cut(item, ":")
		if !ok {
			return fmt.Errorf("F2M_JPNAME の形式が不正です")
		}

		name = strings.TrimSpace(name)
		label = strings.TrimSpace(label)

		if name == "" {
			return fmt.Errorf("F2M_JPNAME の項目名が空です")
		}

		if _, exists := formConfig.FieldLabels[name]; !exists {
			formConfig.FieldOrder = append(formConfig.FieldOrder, name)
		}

		formConfig.FieldLabels[name] = label
	}

	return nil
}

/**
 * 一致チェック定義解析。
 *
 * field1:field2形式の配列変換処理。
 */
func parseEqualFields(value string) ([]EqualField, error) {
	items := make([]EqualField, 0)

	for _, item := range splitList(value) {
		left, right, ok := strings.Cut(item, ":")
		if !ok {
			return nil, fmt.Errorf("F2M_CHK_EQ の形式が不正です")
		}

		left = strings.TrimSpace(left)
		right = strings.TrimSpace(right)

		if left == "" || right == "" {
			return nil, fmt.Errorf("F2M_CHK_EQ の項目名が空です")
		}

		items = append(items, EqualField{Left: left, Right: right})
	}

	return items, nil
}

/**
 * 真偽値解析。
 *
 * 旧PHP版のtrue判定に近い表現を許容する処理。
 */
func parseBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "on", "true", "yes", "1":
		return true
	default:
		return false
	}
}

/**
 * 解析エラー生成。
 *
 * 設定ファイル行番号付きエラー。
 */
func parseError(lineNumber int, message string) error {
	return fmt.Errorf("設定ファイル %d 行目: %s", lineNumber, message)
}
