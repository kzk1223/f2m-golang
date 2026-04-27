// Package charset は、外部ファイルの文字コードをアプリ内部のUTF-8文字列へ正規化するpackageである。
//
// Goではディレクトリ単位でpackageを構成するため、このファイルは internal/charset package の一部である。
// 設定ファイル、HTMLテンプレート、メールテンプレートなど、文字コード判定を必要とする処理から利用される。
package charset

import (
	"bytes"
	"fmt"
	"os"
	"unicode/utf8"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

/**
 * 入力文字コード種別。
 *
 * 読み込み時に判定した文字コード名。
 */
type Encoding string

const (
	EncodingUTF8    Encoding = "UTF-8"
	EncodingUTF8BOM Encoding = "UTF-8-BOM"
	EncodingCP932   Encoding = "CP932"
)

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

/**
 * ファイル内容のUTF-8文字列化。
 *
 * UTF-8 BOM、UTF-8、CP932の順に判定する処理。
 */
func ReadFile(path string) (string, Encoding, error) {
	rawBytes, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}

	return Decode(rawBytes)
}

/**
 * バイト列のUTF-8文字列化。
 *
 * 内部処理用文字列への正規化処理。
 */
func Decode(rawBytes []byte) (string, Encoding, error) {
	// ---------------------------------------------
	// UTF-8 BOM
	// ---------------------------------------------
	if bytes.HasPrefix(rawBytes, utf8BOM) {
		return string(rawBytes[len(utf8BOM):]), EncodingUTF8BOM, nil
	}

	// ---------------------------------------------
	// UTF-8
	// ---------------------------------------------
	if utf8.Valid(rawBytes) {
		return string(rawBytes), EncodingUTF8, nil
	}

	// ---------------------------------------------
	// CP932
	// ---------------------------------------------
	decodedBytes, _, err := transform.Bytes(japanese.ShiftJIS.NewDecoder(), rawBytes)
	if err != nil {
		return "", "", fmt.Errorf("decode cp932: %w", err)
	}

	return string(decodedBytes), EncodingCP932, nil
}
