package app

// このファイルは、app package のHTTP画面遷移を確認するテストである。
// 実行方法: go test ./internal/app

import (
	"encoding/csv"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"f2m-golang/internal/config"
)

/**
 * フォーム画面遷移の確認。
 *
 * 確認POST、戻るPOST、完了POSTの分岐を検証する。
 */
func TestHandlerTransitions(t *testing.T) {
	handler := New(newTestConfigSet(t))

	confirmResponse := performRequest(handler, http.MethodPost, "/", url.Values{
		"F2M_ID":  {"contact"},
		"name":    {"山田太郎"},
		"mail":    {"taro@example.com"},
		"mail2":   {"taro@example.com"},
		"contact": {"確認テスト"},
	})
	assertResponseContains(
		t,
		confirmResponse,
		http.StatusOK,
		"CONFIRM name=山田太郎 mail=taro@example.com mail2=taro@example.com contact=確認テスト",
		`name="f2m_submit_token"`,
	)
	submitToken := extractSubmitToken(t, confirmResponse)

	backResponse := performRequest(handler, http.MethodPost, "/", url.Values{
		"F2M_ID":  {"contact"},
		"mode":    {"form"},
		"name":    {"山田太郎"},
		"mail":    {"taro@example.com"},
		"mail2":   {"taro@example.com"},
		"contact": {"確認テスト"},
	})
	assertResponseContains(t, backResponse, http.StatusOK, `value="山田太郎"`, `value="taro@example.com"`, `確認テスト`)

	thanksResponse := performRequest(handler, http.MethodPost, "/", url.Values{
		"F2M_ID":           {"contact"},
		"mode":             {"send"},
		"f2m_submit_token": {submitToken},
		"name":             {"山田太郎"},
		"mail":             {"taro@example.com"},
		"mail2":            {"taro@example.com"},
		"contact":          {"確認テスト"},
	})
	assertResponse(t, thanksResponse, http.StatusOK, "THANKS お問い合わせ")
}

/**
 * 送信トークンなし送信の拒否確認。
 *
 * 確認画面を経由しないmode=sendが拒否されることを検証する。
 */
func TestHandlerRejectsSendWithoutSubmitToken(t *testing.T) {
	handler := New(newTestConfigSet(t))

	response := performRequest(handler, http.MethodPost, "/", url.Values{
		"F2M_ID":  {"contact"},
		"mode":    {"send"},
		"name":    {"山田太郎"},
		"mail":    {"taro@example.com"},
		"mail2":   {"taro@example.com"},
		"contact": {"確認テスト"},
	})

	assertResponse(t, response, http.StatusBadRequest, "invalid submit token")
}

/**
 * 送信値改ざんの拒否確認。
 *
 * 確認画面で署名した入力値と異なるmode=sendが拒否されることを検証する。
 */
func TestHandlerRejectsTamperedSendValues(t *testing.T) {
	handler := New(newTestConfigSet(t))

	confirmResponse := performRequest(handler, http.MethodPost, "/", url.Values{
		"F2M_ID":  {"contact"},
		"name":    {"山田太郎"},
		"mail":    {"taro@example.com"},
		"mail2":   {"taro@example.com"},
		"contact": {"確認テスト"},
	})
	submitToken := extractSubmitToken(t, confirmResponse)

	response := performRequest(handler, http.MethodPost, "/", url.Values{
		"F2M_ID":           {"contact"},
		"mode":             {"send"},
		"f2m_submit_token": {submitToken},
		"name":             {"山田太郎"},
		"mail":             {"taro@example.com"},
		"mail2":            {"taro@example.com"},
		"contact":          {"改ざんテスト"},
	})

	assertResponse(t, response, http.StatusBadRequest, "invalid submit token")
}

/**
 * CSV保存付き送信の確認。
 *
 * mode=send成功時に設定されたCSVへ送信内容が保存されることを検証する。
 */
func TestHandlerSavesCSVOnSend(t *testing.T) {
	configSet := newTestConfigSet(t)
	csvPath := filepath.Join(t.TempDir(), "contact.csv")
	formConfig := configSet.Forms["contact"]
	formConfig.CSVPath = csvPath
	formConfig.CSVCharset = "UTF-8"
	configSet.Forms["contact"] = formConfig
	handler := New(configSet)

	confirmResponse := performRequest(handler, http.MethodPost, "/", url.Values{
		"F2M_ID":  {"contact"},
		"name":    {"山田太郎"},
		"mail":    {"taro@example.com"},
		"mail2":   {"taro@example.com"},
		"contact": {"CSV保存テスト"},
	})
	submitToken := extractSubmitToken(t, confirmResponse)

	response := performRequestWithMeta(handler, http.MethodPost, "/", url.Values{
		"F2M_ID":           {"contact"},
		"mode":             {"send"},
		"f2m_submit_token": {submitToken},
		"name":             {"山田太郎"},
		"mail":             {"taro@example.com"},
		"mail2":            {"taro@example.com"},
		"contact":          {"CSV保存テスト"},
	}, "203.0.113.10:54321", http.Header{
		"X-Forwarded-For": {"198.51.100.1, 198.51.100.2"},
		"X-Real-Ip":       {"198.51.100.1"},
	})

	assertResponse(t, response, http.StatusOK, "THANKS お問い合わせ")
	assertSavedCSVWithSubmitMeta(t, csvPath)
}

/**
 * radioとcheckbox入力の確認。
 *
 * 複数値の確認表示、戻る復元、送信遷移を検証する。
 */
func TestHandlerHandlesRadioAndCheckboxValues(t *testing.T) {
	handler := New(newTestConfigSet(t))

	confirmResponse := performRequest(handler, http.MethodPost, "/", url.Values{
		"F2M_ID":   {"survey"},
		"name":     {"山田太郎"},
		"interest": {"資料請求", "見積依頼"},
		"gender":   {"女性"},
	})
	assertResponseContains(
		t,
		confirmResponse,
		http.StatusOK,
		"name=山田太郎 interest=資料請求、見積依頼 gender=女性",
		`name="interest" value="資料請求"`,
		`name="interest" value="見積依頼"`,
		`name="gender" value="女性"`,
	)
	submitToken := extractSubmitToken(t, confirmResponse)

	backResponse := performRequest(handler, http.MethodPost, "/", url.Values{
		"F2M_ID":   {"survey"},
		"mode":     {"form"},
		"name":     {"山田太郎"},
		"interest": {"資料請求", "見積依頼"},
		"gender":   {"女性"},
	})
	assertResponseContains(
		t,
		backResponse,
		http.StatusOK,
		`value="資料請求" checked="checked"`,
		`value="見積依頼" checked="checked"`,
		`value="女性" checked="checked"`,
	)

	thanksResponse := performRequest(handler, http.MethodPost, "/", url.Values{
		"F2M_ID":           {"survey"},
		"mode":             {"send"},
		"f2m_submit_token": {submitToken},
		"name":             {"山田太郎"},
		"interest":         {"資料請求", "見積依頼"},
		"gender":           {"女性"},
	})
	assertResponse(t, thanksResponse, http.StatusOK, "THANKS アンケート")
}

/**
 * GETリクエスト拒否の確認。
 *
 * フォーム処理handlerがPOST専用であることを検証する。
 */
func TestHandlerRejectsGetRequest(t *testing.T) {
	handler := New(newTestConfigSet(t))

	response := performRequest(handler, http.MethodGet, "/", nil)

	assertResponse(t, response, http.StatusMethodNotAllowed, "method not allowed")
}

/**
 * 固定HTMLフォームへのエラー反映確認。
 *
 * 入力エラー、入力値復元、項目別エラーの自動挿入を検証する。
 */
func TestHandlerRendersFixedFormWithErrors(t *testing.T) {
	handler := New(newTestConfigSet(t))

	response := performRequest(handler, http.MethodPost, "/", url.Values{
		"F2M_ID":  {"contact"},
		"name":    {"山田太郎"},
		"mail":    {"invalid-mail"},
		"mail2":   {"other@example.com"},
		"contact": {"確認テスト"},
	})

	assertResponseContains(
		t,
		response,
		http.StatusOK,
		`class="f2m-error-summary"`,
		"メールアドレスの形式が不正です。",
		"メールアドレスとメールアドレス確認用が一致しません。",
		`value="山田太郎"`,
		`value="invalid-mail"`,
		`aria-invalid="true"`,
		`data-f2m-error-for="mail"`,
		"確認テスト",
	)
}

/**
 * 必須エラーの確認。
 *
 * F2M_CHKで指定された未入力項目が拒否されることを検証する。
 */
func TestHandlerRendersRequiredErrors(t *testing.T) {
	handler := New(newTestConfigSet(t))

	response := performRequest(handler, http.MethodPost, "/", url.Values{
		"F2M_ID": {"contact"},
		"mail":   {"taro@example.com"},
		"mail2":  {"taro@example.com"},
	})

	assertResponseContains(
		t,
		response,
		http.StatusOK,
		"お名前を入力してください。",
		"お問い合わせ内容を入力してください。",
		`data-f2m-error-for="name"`,
		`data-f2m-error-for="contact"`,
	)
}

/**
 * F2M_ID指定による設定選択の確認。
 *
 * POST値のF2M_IDに対応するフォーム設定が使われることを検証する。
 */
func TestHandlerSelectsFormConfigByF2MID(t *testing.T) {
	handler := New(newTestConfigSet(t))

	response := performRequest(handler, http.MethodPost, "/", url.Values{
		"F2M_ID":  {"recruit"},
		"name":    {"山田花子"},
		"company": {"サンプル株式会社"},
	})

	assertResponse(t, response, http.StatusOK, "CONFIRM name=山田花子 company=サンプル株式会社")
}

/**
 * 不正F2M_IDの確認。
 *
 * 未定義フォームIDが拒否されることを検証する。
 */
func TestHandlerRejectsUnknownF2MID(t *testing.T) {
	handler := New(newTestConfigSet(t))

	response := performRequest(handler, http.MethodPost, "/", url.Values{
		"F2M_ID": {"unknown"},
		"name":   {"山田太郎"},
	})

	assertResponse(t, response, http.StatusBadRequest, "invalid F2M_ID")
}

/**
 * テスト用設定集合生成。
 *
 * 一時テンプレートを持つ設定集合を返す処理。
 */
func newTestConfigSet(t *testing.T) *config.ConfigSet {
	t.Helper()

	templateDir := t.TempDir()
	writeTestTemplate(t, templateDir, "form.html", `
<!doctype html>
<html lang="ja">
<body>
<form method="post" action="/">
<input type="hidden" name="F2M_ID" value="contact">
<input name="name">
<input name="mail">
<input name="mail2">
<textarea name="contact"></textarea>
</form>
</body>
</html>
`)
	writeTestTemplate(t, templateDir, "survey_form.html", `
<!doctype html>
<html lang="ja">
<body>
<form method="post" action="/">
<input type="hidden" name="F2M_ID" value="survey">
<input name="name">
<label><input type="checkbox" name="interest" value="資料請求">資料請求</label>
<label><input type="checkbox" name="interest" value="見積依頼">見積依頼</label>
<label><input type="checkbox" name="interest" value="相談したい">相談したい</label>
<label><input type="radio" name="gender" value="男性">男性</label>
<label><input type="radio" name="gender" value="女性">女性</label>
</form>
</body>
</html>
`)
	writeTestTemplate(t, templateDir, "confirm.html", `CONFIRM{{range .Fields}} {{.Name}}={{.Value}}{{end}} <input type="hidden" name="f2m_submit_token" value="{{.SubmitToken}}">{{range .Fields}}{{$field := .}}{{range .Values}}<input type="hidden" name="{{$field.Name}}" value="{{.}}">{{end}}{{end}}`)
	writeTestTemplate(t, templateDir, "thanks.html", `THANKS {{.Title}}`)

	return &config.ConfigSet{
		Forms: map[string]config.FormConfig{
			"contact": {
				ID:             "contact",
				Subject:        "お問い合わせ",
				FieldOrder:     []string{"name", "mail", "mail2", "contact"},
				FieldLabels:    map[string]string{"name": "お名前", "mail": "メールアドレス", "mail2": "メールアドレス確認用", "contact": "お問い合わせ内容"},
				EmailFields:    map[string]bool{"mail": true},
				EqualFields:    []config.EqualField{{Left: "mail", Right: "mail2"}},
				FormPath:       filepath.Join(templateDir, "form.html"),
				ConfirmPath:    filepath.Join(templateDir, "confirm.html"),
				ThanksPath:     filepath.Join(templateDir, "thanks.html"),
				RequiredFields: map[string]bool{"name": true, "mail": true, "mail2": true, "contact": true},
			},
			"recruit": {
				ID:             "recruit",
				Subject:        "採用応募",
				FieldOrder:     []string{"name", "company"},
				FieldLabels:    map[string]string{"name": "お名前", "company": "会社名"},
				EmailFields:    map[string]bool{},
				FormPath:       filepath.Join(templateDir, "form.html"),
				ConfirmPath:    filepath.Join(templateDir, "confirm.html"),
				ThanksPath:     filepath.Join(templateDir, "thanks.html"),
				RequiredFields: map[string]bool{},
			},
			"survey": {
				ID:             "survey",
				Subject:        "アンケート",
				FieldOrder:     []string{"name", "interest", "gender"},
				FieldLabels:    map[string]string{"name": "お名前", "interest": "興味のある内容", "gender": "性別"},
				EmailFields:    map[string]bool{},
				FormPath:       filepath.Join(templateDir, "survey_form.html"),
				ConfirmPath:    filepath.Join(templateDir, "confirm.html"),
				ThanksPath:     filepath.Join(templateDir, "thanks.html"),
				RequiredFields: map[string]bool{"name": true, "interest": true, "gender": true},
			},
		},
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
	return performRequestWithMeta(handler, method, path, formValues, "", nil)
}

/**
 * テスト用HTTPリクエスト実行。
 *
 * 接続元情報とHTTPヘッダーを指定してフォームPOSTを送信する処理。
 */
func performRequestWithMeta(handler http.Handler, method string, path string, formValues url.Values, remoteAddr string, headers http.Header) *httptest.ResponseRecorder {
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
	if remoteAddr != "" {
		request.RemoteAddr = remoteAddr
	}
	for headerName, headerValues := range headers {
		for _, headerValue := range headerValues {
			request.Header.Add(headerName, headerValue)
		}
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

	assertResponseContains(t, response, expectedStatus, expectedBody)
}

/**
 * HTTPレスポンス複数文字列検証。
 *
 * ステータスコードと本文の期待文字列群を確認する処理。
 */
func assertResponseContains(t *testing.T, response *httptest.ResponseRecorder, expectedStatus int, expectedBodies ...string) {
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

/**
 * 送信トークン抽出。
 *
 * 確認画面HTMLからhiddenの送信トークン値を取り出す処理。
 */
func extractSubmitToken(t *testing.T, response *httptest.ResponseRecorder) string {
	t.Helper()

	tokenPrefix := `name="f2m_submit_token" value="`
	body := response.Body.String()
	tokenStart := strings.Index(body, tokenPrefix)
	if tokenStart < 0 {
		t.Fatalf("body = %q, want submit token", body)
	}

	tokenStart += len(tokenPrefix)
	tokenEnd := strings.Index(body[tokenStart:], `"`)
	if tokenEnd < 0 {
		t.Fatalf("body = %q, want submit token closing quote", body)
	}

	return body[tokenStart : tokenStart+tokenEnd]
}

/**
 * メタ情報付き保存済みCSV検証。
 *
 * CSVファイルを読み込み、入力値と送信付加情報が保存されていることを検証する。
 */
func assertSavedCSVWithSubmitMeta(t *testing.T, csvPath string) {
	t.Helper()

	csvBytes, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatal(err)
	}

	csvReader := csv.NewReader(strings.NewReader(string(csvBytes)))
	actualRecords, err := csvReader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}

	expectedHeader := []string{"お名前", "メールアドレス", "メールアドレス確認用", "お問い合わせ内容", "送信日時", "送信元IP", "X-Forwarded-For", "X-Real-IP"}
	if len(actualRecords) != 2 {
		t.Fatalf("records = %#v, want 2 records", actualRecords)
	}
	if strings.Join(actualRecords[0], "\x00") != strings.Join(expectedHeader, "\x00") {
		t.Fatalf("header = %#v, want %#v", actualRecords[0], expectedHeader)
	}

	valueRecord := actualRecords[1]
	if len(valueRecord) != len(expectedHeader) {
		t.Fatalf("value record = %#v, want %d columns", valueRecord, len(expectedHeader))
	}

	expectedPrefix := []string{"山田太郎", "taro@example.com", "taro@example.com", "CSV保存テスト"}
	if strings.Join(valueRecord[:4], "\x00") != strings.Join(expectedPrefix, "\x00") {
		t.Fatalf("value record prefix = %#v, want %#v", valueRecord[:4], expectedPrefix)
	}
	if _, err := time.Parse("2006-01-02 15:04:05", valueRecord[4]); err != nil {
		t.Fatalf("sent at = %q, want timestamp layout: %v", valueRecord[4], err)
	}
	if valueRecord[5] != "203.0.113.10" {
		t.Fatalf("remote ip = %q, want %q", valueRecord[5], "203.0.113.10")
	}
	if valueRecord[6] != "198.51.100.1, 198.51.100.2" {
		t.Fatalf("x-forwarded-for = %q, want %q", valueRecord[6], "198.51.100.1, 198.51.100.2")
	}
	if valueRecord[7] != "198.51.100.1" {
		t.Fatalf("x-real-ip = %q, want %q", valueRecord[7], "198.51.100.1")
	}
}

/**
 * 接続元IP取得の確認。
 *
 * RemoteAddrからポート番号を除いたIPが取得されることを検証する。
 */
func TestRemoteIPRemovesPort(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
	request.RemoteAddr = net.JoinHostPort("203.0.113.10", "54321")

	if actualIP := remoteIP(request); actualIP != "203.0.113.10" {
		t.Fatalf("remoteIP = %q, want %q", actualIP, "203.0.113.10")
	}
}
