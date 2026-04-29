package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"f2m-golang/internal/app"
	"f2m-golang/internal/config"
)

/**
 * 開発サーバールーティングの確認。
 *
 * GETは静的配信、POST / はフォーム処理へ委譲することを検証する。
 */
func TestServerRouting(t *testing.T) {
	documentRoot := t.TempDir()
	writeServerTestFile(t, documentRoot, "form.html", `
<!doctype html>
<html lang="ja">
<body>
<form method="post" action="/">
<input type="hidden" name="F2M_ID" value="contact">
<input name="name">
<input name="mail">
<textarea name="contact"></textarea>
</form>
</body>
</html>
`)
	writeServerTestFile(t, documentRoot, "confirm.html", `CONFIRM{{range .Fields}} {{.Name}}={{.Value}}{{end}}`)

	mux := newServerMux(documentRoot, app.New(&config.ConfigSet{
		Forms: map[string]config.FormConfig{
			"contact": {
				ID:          "contact",
				Subject:     "お問い合わせ",
				FieldOrder:  []string{"name", "mail", "contact"},
				FieldLabels: map[string]string{"name": "お名前", "mail": "メールアドレス", "contact": "お問い合わせ内容"},
				FormPath:    filepath.Join(documentRoot, "form.html"),
				ConfirmPath: filepath.Join(documentRoot, "confirm.html"),
			},
		},
	}))

	getResponse := performServerRequest(mux, http.MethodGet, "/form.html", nil)
	assertServerResponseContains(t, getResponse, http.StatusOK, `name="F2M_ID"`, `name="contact"`)

	postResponse := performServerRequest(mux, http.MethodPost, "/", url.Values{
		"F2M_ID":  {"contact"},
		"name":    {"山田太郎"},
		"mail":    {"taro@example.com"},
		"contact": {"確認テスト"},
	})
	assertServerResponseContains(t, postResponse, http.StatusOK, "CONFIRM name=山田太郎 mail=taro@example.com contact=確認テスト")
}

/**
 * テスト用ファイル書き込み。
 *
 * 一時ドキュメントルートへ検証用ファイルを配置する処理。
 */
func writeServerTestFile(t *testing.T, documentRoot string, fileName string, content string) {
	t.Helper()

	filePath := filepath.Join(documentRoot, fileName)
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

/**
 * テスト用HTTPリクエスト実行。
 *
 * url.ValuesをフォームPOSTとして送信する処理。
 */
func performServerRequest(handler http.Handler, method string, path string, formValues url.Values) *httptest.ResponseRecorder {
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
 * ステータスコードと本文の期待文字列群を確認する処理。
 */
func assertServerResponseContains(t *testing.T, response *httptest.ResponseRecorder, expectedStatus int, expectedBodies ...string) {
	t.Helper()

	if response.Code != expectedStatus {
		t.Fatalf("status = %d, want %d", response.Code, expectedStatus)
	}

	for _, expectedBody := range expectedBodies {
		if !strings.Contains(response.Body.String(), expectedBody) {
			t.Fatalf("body = %q, want contains %q", response.Body.String(), expectedBody)
		}
	}
}
