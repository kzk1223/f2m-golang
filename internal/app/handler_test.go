package app

// このファイルは、app package のHTTP画面遷移を確認するテストである。
// 実行方法: go test ./internal/app

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"f2m-golang/internal/config"
)

/**
 * フォーム画面遷移の確認。
 *
 * GET、確認POST、戻るPOST、完了POSTの分岐を検証する。
 */
func TestHandlerTransitions(t *testing.T) {
	handler := New(newTestFormConfig(t))

	getResponse := performRequest(handler, http.MethodGet, "/", nil)
	assertResponse(t, getResponse, http.StatusOK, "FORM name= mail= contact=")

	confirmResponse := performRequest(handler, http.MethodPost, "/", url.Values{
		"name":    {"山田太郎"},
		"mail":    {"taro@example.com"},
		"contact": {"確認テスト"},
	})
	assertResponse(t, confirmResponse, http.StatusOK, "CONFIRM name=山田太郎 mail=taro@example.com contact=確認テスト")

	backResponse := performRequest(handler, http.MethodPost, "/", url.Values{
		"mode":    {"form"},
		"name":    {"山田太郎"},
		"mail":    {"taro@example.com"},
		"contact": {"確認テスト"},
	})
	assertResponse(t, backResponse, http.StatusOK, "FORM name=山田太郎 mail=taro@example.com contact=確認テスト")

	thanksResponse := performRequest(handler, http.MethodPost, "/", url.Values{
		"mode":    {"send"},
		"name":    {"山田太郎"},
		"mail":    {"taro@example.com"},
		"contact": {"確認テスト"},
	})
	assertResponse(t, thanksResponse, http.StatusOK, "THANKS お問い合わせ")
}

/**
 * テスト用フォーム設定生成。
 *
 * 一時テンプレートを持つフォーム設定を返す処理。
 */
func newTestFormConfig(t *testing.T) config.FormConfig {
	t.Helper()

	templateDir := t.TempDir()
	writeTestTemplate(t, templateDir, "form.html", `FORM{{range .Fields}} {{.Name}}={{.Value}}{{end}}`)
	writeTestTemplate(t, templateDir, "confirm.html", `CONFIRM{{range .Fields}} {{.Name}}={{.Value}}{{end}}`)
	writeTestTemplate(t, templateDir, "thanks.html", `THANKS {{.Title}}`)

	return config.FormConfig{
		ID:             "contact",
		Subject:        "お問い合わせ",
		FieldOrder:     []string{"name", "mail", "contact"},
		FieldLabels:    map[string]string{"name": "お名前", "mail": "メールアドレス", "contact": "お問い合わせ内容"},
		EmailFields:    map[string]bool{"mail": true},
		FormPath:       filepath.Join(templateDir, "form.html"),
		ConfirmPath:    filepath.Join(templateDir, "confirm.html"),
		ThanksPath:     filepath.Join(templateDir, "thanks.html"),
		RequiredFields: map[string]bool{},
	}
}

/**
 * テスト用テンプレート書き込み。
 *
 * 一時ディレクトリへ最小テンプレートを配置する処理。
 */
func writeTestTemplate(t *testing.T, templateDir string, fileName string, templateText string) {
	t.Helper()

	templatePath := filepath.Join(templateDir, fileName)
	if err := os.WriteFile(templatePath, []byte(templateText), 0644); err != nil {
		t.Fatal(err)
	}
}

/**
 * テスト用HTTPリクエスト実行。
 *
 * url.ValuesをフォームPOSTとして送信する処理。
 */
func performRequest(handler http.Handler, method string, path string, formValues url.Values) *httptest.ResponseRecorder {
	var requestBody *strings.Reader
	if formValues == nil {
		requestBody = strings.NewReader("")
	} else {
		requestBody = strings.NewReader(formValues.Encode())
	}

	request := httptest.NewRequest(method, path, requestBody)
	if formValues != nil {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	return response
}

/**
 * HTTPレスポンス検証。
 *
 * ステータスコードと本文の期待文字列を確認する処理。
 */
func assertResponse(t *testing.T, response *httptest.ResponseRecorder, expectedStatus int, expectedBody string) {
	t.Helper()

	if response.Code != expectedStatus {
		t.Fatalf("status = %d, want %d", response.Code, expectedStatus)
	}

	if !strings.Contains(response.Body.String(), expectedBody) {
		t.Fatalf("body = %q, want contains %q", response.Body.String(), expectedBody)
	}
}
