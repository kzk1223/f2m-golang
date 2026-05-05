package charset

import (
	"fmt"
	"strings"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

/**
 * 文字列エンコード。
 *
 * UTF-8文字列を指定文字コードのバイト列へ変換する処理。
 */
func Encode(text string, encodingName string) ([]byte, error) {
	switch normalizeEncodingName(encodingName) {
	case "", "UTF8":
		return []byte(text), nil
	case "SJIS", "SHIFTJIS", "CP932", "WINDOWS31J":
		encodedText, _, err := transform.String(japanese.ShiftJIS.NewEncoder(), text)
		if err != nil {
			return nil, fmt.Errorf("encode cp932: %w", err)
		}

		return []byte(encodedText), nil
	default:
		return nil, fmt.Errorf("unsupported encoding: %s", encodingName)
	}
}

/**
 * 文字コード名正規化。
 *
 * 表記ゆれを比較用の文字列へ変換する処理。
 */
func normalizeEncodingName(encodingName string) string {
	normalizedName := strings.ToUpper(strings.TrimSpace(encodingName))
	normalizedName = strings.ReplaceAll(normalizedName, "-", "")
	normalizedName = strings.ReplaceAll(normalizedName, "_", "")

	return normalizedName
}
