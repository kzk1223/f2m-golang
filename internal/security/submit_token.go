// Package security は、フォーム送信に関する検証処理を扱うpackageである。
package security

import (
	"errors"
	"time"

	"github.com/gorilla/securecookie"
)

const (
	submitTokenName        = "f2m_submit_token"
	submitTokenHashKeySize = 64
)

var (
	errSubmitTokenInvalid = errors.New("submit token is invalid")
	errSubmitTokenExpired = errors.New("submit token is expired")
)

/**
 * 送信トークン署名器。
 *
 * 確認画面経由の送信値であることをHMAC署名で検証する値。
 */
type SubmitTokenSigner struct {
	secureCookie *securecookie.SecureCookie
	now          func() time.Time
}

/**
 * 送信トークン署名器生成。
 *
 * 暗号論的乱数の秘密鍵を持つ署名器を返す処理。
 */
func NewSubmitTokenSigner() (*SubmitTokenSigner, error) {
	hashKey := securecookie.GenerateRandomKey(submitTokenHashKeySize)
	if hashKey == nil {
		return nil, errors.New("submit token key generation failed")
	}

	secureCookie := securecookie.New(hashKey, nil)
	secureCookie.SetSerializer(securecookie.JSONEncoder{})

	return &SubmitTokenSigner{
		secureCookie: secureCookie,
		now:          time.Now,
	}, nil
}

/**
 * 送信トークン生成。
 *
 * フォームID、入力値、添付ID、有効期限を署名付き文字列へ変換する処理。
 */
func (signer *SubmitTokenSigner) Sign(formID string, fieldValues map[string][]string, attachmentIDs []string, expiresIn time.Duration) (string, error) {
	claims := SubmitTokenClaims{
		FormID:        formID,
		FieldValues:   cloneFieldValues(fieldValues),
		AttachmentIDs: cloneStringSlice(attachmentIDs),
		ExpiresAt:     signer.now().Add(expiresIn).Unix(),
	}

	return signer.secureCookie.Encode(submitTokenName, claims)
}

/**
 * 送信トークン検証。
 *
 * 改ざん、有効期限、フォームID、入力値、添付IDの一致を検証する処理。
 */
func (signer *SubmitTokenSigner) Verify(token string, formID string, fieldValues map[string][]string, attachmentIDs []string) error {
	var claims SubmitTokenClaims
	if err := signer.secureCookie.Decode(submitTokenName, token, &claims); err != nil {
		return errSubmitTokenInvalid
	}

	if signer.now().Unix() > claims.ExpiresAt {
		return errSubmitTokenExpired
	}

	if claims.FormID != formID ||
		!equalFieldValues(claims.FieldValues, fieldValues) ||
		!equalStringSlice(claims.AttachmentIDs, attachmentIDs) {
		return errSubmitTokenInvalid
	}

	return nil
}

/**
 * 送信トークンクレーム。
 *
 * 確認画面で確定したフォームID、入力値、添付ID、有効期限。
 */
type SubmitTokenClaims struct {
	FormID        string              `json:"form_id"`
	FieldValues   map[string][]string `json:"field_values"`
	AttachmentIDs []string            `json:"attachment_ids"`
	ExpiresAt     int64               `json:"expires_at"`
}

/**
 * 入力値複製。
 *
 * 署名後の外部変更を避けるためのmap複製処理。
 */
func cloneFieldValues(fieldValues map[string][]string) map[string][]string {
	clonedFieldValues := make(map[string][]string, len(fieldValues))
	for fieldName, fieldValue := range fieldValues {
		clonedFieldValue := make([]string, len(fieldValue))
		copy(clonedFieldValue, fieldValue)
		clonedFieldValues[fieldName] = clonedFieldValue
	}

	return clonedFieldValues
}

/**
 * 文字列スライス複製。
 *
 * 署名後の外部変更を避けるためのスライス複製処理。
 */
func cloneStringSlice(values []string) []string {
	clonedValues := make([]string, len(values))
	copy(clonedValues, values)

	return clonedValues
}

/**
 * 入力値一致判定。
 *
 * 署名済み入力値と送信入力値が完全一致するかを返す処理。
 */
func equalFieldValues(leftFieldValues map[string][]string, rightFieldValues map[string][]string) bool {
	if len(leftFieldValues) != len(rightFieldValues) {
		return false
	}

	for fieldName, leftFieldValue := range leftFieldValues {
		rightFieldValue, exists := rightFieldValues[fieldName]
		if !exists || !equalFieldValueSlice(leftFieldValue, rightFieldValue) {
			return false
		}
	}

	return true
}

/**
 * 入力値スライス一致判定。
 *
 * 署名済み入力値と送信入力値の順序付き値が完全一致するかを返す処理。
 */
func equalFieldValueSlice(leftValues []string, rightValues []string) bool {
	if len(leftValues) != len(rightValues) {
		return false
	}

	for valueIndex, leftValue := range leftValues {
		if rightValues[valueIndex] != leftValue {
			return false
		}
	}

	return true
}

/**
 * 文字列スライス一致判定。
 *
 * 署名済み値と送信値の順序付き文字列が完全一致するかを返す処理。
 */
func equalStringSlice(leftValues []string, rightValues []string) bool {
	if len(leftValues) != len(rightValues) {
		return false
	}

	for valueIndex, leftValue := range leftValues {
		if rightValues[valueIndex] != leftValue {
			return false
		}
	}

	return true
}
