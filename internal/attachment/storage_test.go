package attachment

import (
	"bytes"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"f2m-golang/internal/config"
)

/**
 * 添付一時保存の確認。
 *
 * 拡張子とMIMEが未指定でも保存、取得できることを検証する。
 */
func TestStoreSaveUploadedAllowsUnrestrictedExtensionAndMIME(t *testing.T) {
	store := NewStore(t.TempDir())
	fileHeader := newAttachmentTestFileHeader(t, `C:\fakepath\document.txt`, []byte("hello"))

	attachmentFile, err := store.SaveUploaded(config.FormConfig{AttachMax: "1K"}, "attachment", fileHeader)
	if err != nil {
		t.Fatal(err)
	}

	if attachmentFile.OriginalName != "document.txt" {
		t.Fatalf("OriginalName = %q", attachmentFile.OriginalName)
	}
	if attachmentFile.Size != 5 {
		t.Fatalf("Size = %d", attachmentFile.Size)
	}

	loadedFile, err := store.Load(attachmentFile.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if loadedFile.ID != attachmentFile.ID {
		t.Fatalf("loaded ID = %q", loadedFile.ID)
	}

	storedBytes, err := os.ReadFile(loadedFile.StoredPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(storedBytes) != "hello" {
		t.Fatalf("stored content = %q", string(storedBytes))
	}
}

/**
 * 添付拡張子拒否の確認。
 *
 * F2M_ATTACH_EXT指定時に未許可拡張子が拒否されることを検証する。
 */
func TestStoreSaveUploadedRejectsDisallowedExtension(t *testing.T) {
	store := NewStore(t.TempDir())
	fileHeader := newAttachmentTestFileHeader(t, "document.txt", []byte("hello"))

	_, err := store.SaveUploaded(config.FormConfig{
		AttachMax:  "1K",
		AttachExts: []string{"pdf"},
	}, "attachment", fileHeader)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "拡張子") {
		t.Fatalf("error = %q", err.Error())
	}
}

/**
 * 添付MIME拒否の確認。
 *
 * F2M_ATTACH_MIME指定時に未許可MIMEが拒否されることを検証する。
 */
func TestStoreSaveUploadedRejectsDisallowedMIME(t *testing.T) {
	store := NewStore(t.TempDir())
	fileHeader := newAttachmentTestFileHeader(t, "document.txt", []byte("hello"))

	_, err := store.SaveUploaded(config.FormConfig{
		AttachMax:   "1K",
		AttachMIMEs: []string{"application/pdf"},
	}, "attachment", fileHeader)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "MIME") {
		t.Fatalf("error = %q", err.Error())
	}
}

/**
 * 添付TTL期限切れの確認。
 *
 * TTLを超えた一時保存ファイルが取得不可になることを検証する。
 */
func TestStoreLoadRejectsExpiredAttachment(t *testing.T) {
	now := time.Date(2026, 5, 7, 1, 2, 3, 0, time.Local)
	store := NewStore(t.TempDir())
	store.Now = func() time.Time { return now }
	fileHeader := newAttachmentTestFileHeader(t, "document.txt", []byte("hello"))

	attachmentFile, err := store.SaveUploaded(config.FormConfig{AttachMax: "1K"}, "attachment", fileHeader)
	if err != nil {
		t.Fatal(err)
	}

	store.Now = func() time.Time { return now.Add(2 * time.Hour) }
	_, err = store.Load(attachmentFile.ID, time.Hour)
	if !errors.Is(err, ErrExpired) {
		t.Fatalf("error = %v, want ErrExpired", err)
	}
	if _, err := os.Stat(attachmentFile.StoredPath); !os.IsNotExist(err) {
		t.Fatalf("stored attachment still exists or stat failed unexpectedly: %v", err)
	}
}

/**
 * 添付テスト用ファイルヘッダー生成。
 *
 * multipartフォームからテスト用FileHeaderを生成する処理。
 */
func newAttachmentTestFileHeader(t *testing.T, fileName string, content []byte) *multipart.FileHeader {
	t.Helper()

	var requestBody bytes.Buffer
	multipartWriter := multipart.NewWriter(&requestBody)
	fileWriter, err := multipartWriter.CreateFormFile("attachment", fileName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fileWriter.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := multipartWriter.Close(); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "/", &requestBody)
	request.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	if err := request.ParseMultipartForm(1024); err != nil {
		t.Fatal(err)
	}
	defer request.MultipartForm.RemoveAll()

	return request.MultipartForm.File["attachment"][0]
}
