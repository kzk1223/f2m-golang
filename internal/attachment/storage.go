// Package attachment は、フォーム添付ファイルの一時保存と検証を扱うpackageである。
package attachment

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"f2m-golang/internal/config"
)

const (
	defaultTempRoot = "storage/tmp/attachments"
	defaultMaxBytes = 3 * 1024 * 1024
	defaultTempTTL  = time.Hour
	idPrefix        = "f2m_att_"
	idRandomBytes   = 16
	metaExt         = ".json"
	fileExt         = ".bin"
)

var (
	ErrNotFound = errors.New("attachment not found")
	ErrExpired  = errors.New("attachment expired")
)

/**
 * 添付ファイル情報。
 *
 * 一時保存済みファイルの表示名、保存先、検証済みMIMEを保持する値。
 */
type File struct {
	FieldName    string    `json:"field_name"`
	ID           string    `json:"id"`
	OriginalName string    `json:"original_name"`
	StoredPath   string    `json:"-"`
	ContentType  string    `json:"content_type"`
	Size         int64     `json:"size"`
	CreatedAt    time.Time `json:"created_at"`
}

/**
 * 添付ファイル一時保存。
 *
 * 公開ディレクトリ外の一時領域へ添付ファイルを保存する処理。
 */
type Store struct {
	Root string
	Now  func() time.Time
}

/**
 * 添付ファイル一時保存の生成。
 *
 * 保存先未指定時は既定の一時保存先を使う処理。
 */
func NewStore(root string) *Store {
	if strings.TrimSpace(root) == "" {
		root = defaultTempRoot
	}

	return &Store{
		Root: root,
		Now:  time.Now,
	}
}

/**
 * 添付機能有効判定。
 *
 * 添付対象フィールドが設定されているかを返す処理。
 */
func Enabled(formConfig config.FormConfig) bool {
	return len(normalizedFieldNames(formConfig.AttachFields)) > 0
}

/**
 * 添付対象フィールド名。
 *
 * 重複と空白を除外した添付対象フィールド名を返す処理。
 */
func FieldNames(formConfig config.FormConfig) []string {
	return normalizedFieldNames(formConfig.AttachFields)
}

/**
 * 添付最大バイト数。
 *
 * F2M_ATTACH_MAXのサイズ表記をバイト数へ変換する処理。
 */
func MaxSizeBytes(maxSizeText string) (int64, error) {
	maxSizeText = strings.TrimSpace(maxSizeText)
	if maxSizeText == "" {
		return defaultMaxBytes, nil
	}

	unit := int64(1)
	numberText := maxSizeText
	upperSizeText := strings.ToUpper(maxSizeText)

	switch {
	case strings.HasSuffix(upperSizeText, "KB"):
		unit = 1024
		numberText = maxSizeText[:len(maxSizeText)-2]
	case strings.HasSuffix(upperSizeText, "K"):
		unit = 1024
		numberText = maxSizeText[:len(maxSizeText)-1]
	case strings.HasSuffix(upperSizeText, "MB"):
		unit = 1024 * 1024
		numberText = maxSizeText[:len(maxSizeText)-2]
	case strings.HasSuffix(upperSizeText, "M"):
		unit = 1024 * 1024
		numberText = maxSizeText[:len(maxSizeText)-1]
	case strings.HasSuffix(upperSizeText, "GB"):
		unit = 1024 * 1024 * 1024
		numberText = maxSizeText[:len(maxSizeText)-2]
	case strings.HasSuffix(upperSizeText, "G"):
		unit = 1024 * 1024 * 1024
		numberText = maxSizeText[:len(maxSizeText)-1]
	}

	sizeNumber, err := strconv.ParseFloat(strings.TrimSpace(numberText), 64)
	if err != nil || sizeNumber <= 0 {
		return 0, fmt.Errorf("F2M_ATTACH_MAX の形式が不正です")
	}

	return int64(sizeNumber * float64(unit)), nil
}

/**
 * 添付一時保存TTL。
 *
 * 設定値がない場合は既定TTLを返す処理。
 */
func TempTTL(formConfig config.FormConfig) time.Duration {
	if formConfig.AttachTempTTL > 0 {
		return formConfig.AttachTempTTL
	}

	return defaultTempTTL
}

/**
 * アップロードファイル保存。
 *
 * サイズ、拡張子、MIMEを検証して一時保存する処理。
 */
func (store *Store) SaveUploaded(formConfig config.FormConfig, fieldName string, fileHeader *multipart.FileHeader) (File, error) {
	if fileHeader == nil || strings.TrimSpace(fileHeader.Filename) == "" {
		return File{}, ErrNotFound
	}

	maxBytes, err := MaxSizeBytes(formConfig.AttachMax)
	if err != nil {
		return File{}, err
	}
	if fileHeader.Size > maxBytes {
		return File{}, fmt.Errorf("ファイルサイズが上限を超えています。")
	}

	originalName := sanitizeOriginalName(fileHeader.Filename)
	if err := validateExtension(originalName, formConfig.AttachExts); err != nil {
		return File{}, err
	}

	sourceFile, err := fileHeader.Open()
	if err != nil {
		return File{}, err
	}
	defer sourceFile.Close()

	fileID, err := newFileID()
	if err != nil {
		return File{}, err
	}
	if err := os.MkdirAll(store.Root, 0755); err != nil {
		return File{}, err
	}

	storedPath := store.filePath(fileID)
	destinationFile, err := os.OpenFile(storedPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return File{}, err
	}

	contentType, writtenSize, copyErr := copyWithDetection(destinationFile, sourceFile, maxBytes)
	closeErr := destinationFile.Close()
	if copyErr != nil {
		_ = os.Remove(storedPath)
		return File{}, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(storedPath)
		return File{}, closeErr
	}
	if err := validateMIME(contentType, formConfig.AttachMIMEs); err != nil {
		_ = os.Remove(storedPath)
		return File{}, err
	}

	attachmentFile := File{
		FieldName:    fieldName,
		ID:           fileID,
		OriginalName: originalName,
		StoredPath:   storedPath,
		ContentType:  contentType,
		Size:         writtenSize,
		CreatedAt:    store.now(),
	}
	if err := store.writeMeta(attachmentFile); err != nil {
		_ = os.Remove(storedPath)
		return File{}, err
	}

	return attachmentFile, nil
}

/**
 * 添付ファイル取得。
 *
 * 署名済みIDに対応する一時保存ファイルを取得する処理。
 */
func (store *Store) Load(fileID string, ttl time.Duration) (File, error) {
	if !validFileID(fileID) {
		return File{}, ErrNotFound
	}

	metaBytes, err := os.ReadFile(store.metaPath(fileID))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return File{}, ErrNotFound
		}

		return File{}, err
	}

	var attachmentFile File
	if err := json.Unmarshal(metaBytes, &attachmentFile); err != nil {
		return File{}, err
	}
	if attachmentFile.ID != fileID {
		return File{}, ErrNotFound
	}

	attachmentFile.StoredPath = store.filePath(fileID)
	if store.expired(attachmentFile, ttl) {
		_ = store.Delete(attachmentFile)
		return File{}, ErrExpired
	}
	if _, err := os.Stat(attachmentFile.StoredPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return File{}, ErrNotFound
		}

		return File{}, err
	}

	return attachmentFile, nil
}

/**
 * 添付ファイル群取得。
 *
 * 複数の署名済みIDに対応する一時保存ファイルを順序通り取得する処理。
 */
func (store *Store) LoadMany(fileIDs []string, ttl time.Duration) ([]File, error) {
	attachmentFiles := make([]File, 0, len(fileIDs))
	for _, fileID := range fileIDs {
		fileID = strings.TrimSpace(fileID)
		if fileID == "" {
			continue
		}

		attachmentFile, err := store.Load(fileID, ttl)
		if err != nil {
			return nil, err
		}
		attachmentFiles = append(attachmentFiles, attachmentFile)
	}

	return attachmentFiles, nil
}

/**
 * 添付ファイル削除。
 *
 * 一時保存ファイルとメタ情報を削除する処理。
 */
func (store *Store) Delete(attachmentFile File) error {
	if !validFileID(attachmentFile.ID) {
		return nil
	}

	var deleteErr error
	if err := os.Remove(store.filePath(attachmentFile.ID)); err != nil && !errors.Is(err, os.ErrNotExist) {
		deleteErr = err
	}
	if err := os.Remove(store.metaPath(attachmentFile.ID)); err != nil && !errors.Is(err, os.ErrNotExist) && deleteErr == nil {
		deleteErr = err
	}

	return deleteErr
}

/**
 * 添付ファイル群削除。
 *
 * 複数の一時保存ファイルを削除する処理。
 */
func (store *Store) DeleteMany(attachmentFiles []File) error {
	var deleteErr error
	for _, attachmentFile := range attachmentFiles {
		if err := store.Delete(attachmentFile); err != nil && deleteErr == nil {
			deleteErr = err
		}
	}

	return deleteErr
}

/**
 * 期限切れ添付ファイル削除。
 *
 * TTLを超えた一時保存ファイルを削除する処理。
 */
func (store *Store) CleanupExpired(ttl time.Duration) error {
	if err := os.MkdirAll(store.Root, 0755); err != nil {
		return err
	}

	metaPaths, err := filepath.Glob(filepath.Join(store.Root, "*"+metaExt))
	if err != nil {
		return err
	}

	for _, metaPath := range metaPaths {
		fileID := strings.TrimSuffix(filepath.Base(metaPath), metaExt)
		attachmentFile, err := store.Load(fileID, 0)
		if err != nil {
			continue
		}
		if store.expired(attachmentFile, ttl) {
			_ = store.Delete(attachmentFile)
		}
	}

	return nil
}

/**
 * 添付ID一覧生成。
 *
 * 添付ファイル情報から署名用ID一覧を生成する処理。
 */
func IDs(attachmentFiles []File) []string {
	fileIDs := make([]string, 0, len(attachmentFiles))
	for _, attachmentFile := range attachmentFiles {
		fileIDs = append(fileIDs, attachmentFile.ID)
	}

	return fileIDs
}

/**
 * 添付サイズ表示。
 *
 * バイト数を画面表示用の短い文字列へ変換する処理。
 */
func FormatSize(size int64) string {
	switch {
	case size >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
	case size >= 1024:
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	default:
		return fmt.Sprintf("%d B", size)
	}
}

/**
 * 一時保存メタ情報書き込み。
 *
 * 添付ファイル情報をJSONで保存する処理。
 */
func (store *Store) writeMeta(attachmentFile File) error {
	metaBytes, err := json.MarshalIndent(attachmentFile, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(store.metaPath(attachmentFile.ID), metaBytes, 0600)
}

/**
 * 一時保存ファイルパス。
 *
 * 添付IDから実ファイルパスを生成する処理。
 */
func (store *Store) filePath(fileID string) string {
	return filepath.Join(store.Root, fileID+fileExt)
}

/**
 * 一時保存メタ情報パス。
 *
 * 添付IDからメタ情報パスを生成する処理。
 */
func (store *Store) metaPath(fileID string) string {
	return filepath.Join(store.Root, fileID+metaExt)
}

/**
 * 現在時刻取得。
 *
 * テスト差し替え可能な時刻を返す処理。
 */
func (store *Store) now() time.Time {
	if store.Now != nil {
		return store.Now()
	}

	return time.Now()
}

/**
 * 期限切れ判定。
 *
 * 作成時刻とTTLから一時保存期限切れかを返す処理。
 */
func (store *Store) expired(attachmentFile File, ttl time.Duration) bool {
	if ttl <= 0 {
		return false
	}

	return store.now().After(attachmentFile.CreatedAt.Add(ttl))
}

/**
 * 添付ファイルコピー。
 *
 * 先頭バイトでMIMEを判定し、サイズ上限内で保存先へコピーする処理。
 */
func copyWithDetection(destination io.Writer, source io.Reader, maxBytes int64) (string, int64, error) {
	limitedSource := io.LimitReader(source, maxBytes+1)
	detectionBytes := make([]byte, 512)
	detectionSize, err := io.ReadFull(limitedSource, detectionBytes)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return "", 0, err
	}

	contentType := http.DetectContentType(detectionBytes[:detectionSize])
	writtenSize, err := destination.Write(detectionBytes[:detectionSize])
	if err != nil {
		return "", int64(writtenSize), err
	}

	remainingSize, err := io.Copy(destination, limitedSource)
	totalSize := int64(writtenSize) + remainingSize
	if err != nil {
		return "", totalSize, err
	}
	if totalSize > maxBytes {
		return "", totalSize, fmt.Errorf("ファイルサイズが上限を超えています。")
	}

	return contentType, totalSize, nil
}

/**
 * 拡張子検証。
 *
 * 許可拡張子が指定されている場合のみ拡張子を検証する処理。
 */
func validateExtension(originalName string, allowedExts []string) error {
	normalizedAllowedExts := normalizedExts(allowedExts)
	if len(normalizedAllowedExts) == 0 {
		return nil
	}

	fileExtName := strings.TrimPrefix(strings.ToLower(filepath.Ext(originalName)), ".")
	if fileExtName == "" || !normalizedAllowedExts[fileExtName] {
		return fmt.Errorf("添付ファイルの拡張子が許可されていません。")
	}

	return nil
}

/**
 * MIME検証。
 *
 * 許可MIMEが指定されている場合のみMIMEを検証する処理。
 */
func validateMIME(contentType string, allowedMIMEs []string) error {
	normalizedAllowedMIMEs := normalizedMIMEs(allowedMIMEs)
	if len(normalizedAllowedMIMEs) == 0 {
		return nil
	}

	normalizedContentType := normalizeMIME(contentType)
	if !normalizedAllowedMIMEs[normalizedContentType] {
		return fmt.Errorf("添付ファイルのMIMEタイプが許可されていません。")
	}

	return nil
}

/**
 * 元ファイル名正規化。
 *
 * パス要素と制御文字を除去した表示用ファイル名を返す処理。
 */
func sanitizeOriginalName(originalName string) string {
	normalizedName := strings.ReplaceAll(originalName, "\\", "/")
	normalizedName = filepath.Base(normalizedName)
	normalizedName = strings.Map(func(value rune) rune {
		if unicode.IsControl(value) {
			return -1
		}

		return value
	}, normalizedName)
	normalizedName = strings.TrimSpace(normalizedName)
	if normalizedName == "" || normalizedName == "." || normalizedName == string(filepath.Separator) {
		return "attachment"
	}

	return normalizedName
}

/**
 * 添付ID生成。
 *
 * 一時保存ファイル用の推測困難なIDを生成する処理。
 */
func newFileID() (string, error) {
	randomBytes := make([]byte, idRandomBytes)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}

	return idPrefix + hex.EncodeToString(randomBytes), nil
}

/**
 * 添付ID検証。
 *
 * 一時保存ファイル名として扱えるIDかを返す処理。
 */
func validFileID(fileID string) bool {
	if !strings.HasPrefix(fileID, idPrefix) {
		return false
	}

	hexID := strings.TrimPrefix(fileID, idPrefix)
	if len(hexID) != idRandomBytes*2 {
		return false
	}

	_, err := hex.DecodeString(hexID)
	return err == nil
}

/**
 * 添付フィールド名正規化。
 *
 * 空白と重複を除外したフィールド名一覧を生成する処理。
 */
func normalizedFieldNames(fieldNames []string) []string {
	normalizedNames := make([]string, 0, len(fieldNames))
	seen := make(map[string]bool)
	for _, fieldName := range fieldNames {
		fieldName = strings.TrimSpace(fieldName)
		if fieldName == "" || seen[fieldName] {
			continue
		}

		normalizedNames = append(normalizedNames, fieldName)
		seen[fieldName] = true
	}

	return normalizedNames
}

/**
 * 許可拡張子正規化。
 *
 * 比較用の小文字拡張子集合を生成する処理。
 */
func normalizedExts(exts []string) map[string]bool {
	normalizedExts := make(map[string]bool)
	for _, extName := range exts {
		extName = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(extName)), ".")
		if extName != "" {
			normalizedExts[extName] = true
		}
	}

	return normalizedExts
}

/**
 * 許可MIME正規化。
 *
 * 比較用の小文字MIME集合を生成する処理。
 */
func normalizedMIMEs(mimes []string) map[string]bool {
	normalizedMIMEs := make(map[string]bool)
	for _, mimeName := range mimes {
		mimeName = normalizeMIME(mimeName)
		if mimeName != "" {
			normalizedMIMEs[mimeName] = true
		}
	}

	return normalizedMIMEs
}

/**
 * MIME正規化。
 *
 * charset等のパラメータを除外した比較用MIMEを返す処理。
 */
func normalizeMIME(mimeName string) string {
	mimeName = strings.ToLower(strings.TrimSpace(mimeName))
	if semicolonIndex := strings.Index(mimeName, ";"); semicolonIndex >= 0 {
		mimeName = strings.TrimSpace(mimeName[:semicolonIndex])
	}

	return mimeName
}
