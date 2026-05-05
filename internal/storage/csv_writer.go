// Package storage は、フォーム送信内容の永続化処理を扱うpackageである。
package storage

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"f2m-golang/internal/charset"
	"f2m-golang/internal/config"

	"github.com/gofrs/flock"
)

const (
	csvLockTimeout    = 10 * time.Second
	csvLockRetryDelay = 50 * time.Millisecond
	csvSentAtLayout   = "2006-01-02 15:04:05"
)

var csvWriteMu sync.Mutex

/**
 * CSV追記保存。
 *
 * 設定されたCSVパスへ入力値を項目順で追記する処理。
 */
func AppendCSV(formConfig config.FormConfig, fieldValues map[string][]string, submitMeta CSVSubmitMeta) (returnErr error) {
	csvPath := strings.TrimSpace(formConfig.CSVPath)
	if csvPath == "" {
		return nil
	}

	csvWriteMu.Lock()
	defer csvWriteMu.Unlock()

	// ---------------------------------------------
	// 保存先準備
	// ---------------------------------------------
	if err := os.MkdirAll(filepath.Dir(csvPath), 0755); err != nil {
		return fmt.Errorf("create csv directory: %w", err)
	}

	csvLock, err := lockCSVFile(csvPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := csvLock.Unlock(); err != nil && returnErr == nil {
			returnErr = fmt.Errorf("unlock csv file: %w", err)
		}
	}()

	needsHeader, err := needsCSVHeader(csvPath)
	if err != nil {
		return err
	}

	records := make([][]string, 0, 2)
	if needsHeader {
		records = append(records, csvHeaderRecord(formConfig))
	}
	records = append(records, csvValueRecord(formConfig, fieldValues, submitMeta))

	csvBytes, err := renderCSVRecords(records, formConfig.CSVCharset)
	if err != nil {
		return err
	}

	// ---------------------------------------------
	// 追記保存
	// ---------------------------------------------
	csvFile, err := os.OpenFile(csvPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open csv file: %w", err)
	}

	if _, err := csvFile.Write(csvBytes); err != nil {
		csvFile.Close()
		return fmt.Errorf("write csv file: %w", err)
	}

	if err := csvFile.Close(); err != nil {
		return fmt.Errorf("close csv file: %w", err)
	}

	return nil
}

/**
 * CSVファイルロック取得。
 *
 * CSV横のロックファイルを使い、プロセス間排他を取得する処理。
 */
func lockCSVFile(csvPath string) (*flock.Flock, error) {
	lockPath := csvPath + ".lock"
	csvLock := flock.New(lockPath)

	lockContext, cancel := context.WithTimeout(context.Background(), csvLockTimeout)
	defer cancel()

	locked, err := csvLock.TryLockContext(lockContext, csvLockRetryDelay)
	if err != nil {
		return nil, fmt.Errorf("lock csv file: %w", err)
	}
	if !locked {
		return nil, fmt.Errorf("lock csv file timeout: %s", lockPath)
	}

	return csvLock, nil
}

/**
 * CSV送信メタ情報。
 *
 * 入力項目以外にCSVへ保存する送信時の付加情報。
 */
type CSVSubmitMeta struct {
	SentAt        time.Time
	RemoteIP      string
	XForwardedFor string
	XRealIP       string
}

/**
 * CSVヘッダー要否判定。
 *
 * ファイル未作成または空ファイルの場合にヘッダー出力を必要とする処理。
 */
func needsCSVHeader(csvPath string) (bool, error) {
	fileInfo, err := os.Stat(csvPath)
	if err == nil {
		return fileInfo.Size() == 0, nil
	}

	if os.IsNotExist(err) {
		return true, nil
	}

	return false, fmt.Errorf("stat csv file: %w", err)
}

/**
 * CSVヘッダー行生成。
 *
 * F2M_JPNAMEの表示順と表示名からヘッダー行を生成する処理。
 */
func csvHeaderRecord(formConfig config.FormConfig) []string {
	headerRecord := make([]string, 0, len(formConfig.FieldOrder))

	for _, fieldName := range formConfig.FieldOrder {
		fieldLabel := strings.TrimSpace(formConfig.FieldLabels[fieldName])
		if fieldLabel == "" {
			fieldLabel = fieldName
		}

		headerRecord = append(headerRecord, fieldLabel)
	}

	headerRecord = append(headerRecord, "送信日時", "送信元IP", "X-Forwarded-For", "X-Real-IP")

	return headerRecord
}

/**
 * CSV値行生成。
 *
 * F2M_JPNAMEの表示順で入力値と送信メタ情報の行を生成する処理。
 */
func csvValueRecord(formConfig config.FormConfig, fieldValues map[string][]string, submitMeta CSVSubmitMeta) []string {
	valueRecord := make([]string, 0, len(formConfig.FieldOrder)+4)

	for _, fieldName := range formConfig.FieldOrder {
		valueRecord = append(valueRecord, joinCSVFieldValues(fieldValues[fieldName]))
	}

	valueRecord = append(
		valueRecord,
		formatCSVSentAt(submitMeta.SentAt),
		submitMeta.RemoteIP,
		submitMeta.XForwardedFor,
		submitMeta.XRealIP,
	)

	return valueRecord
}

/**
 * CSV入力値結合。
 *
 * 複数選択値を1セル保存用の文字列へ変換する処理。
 */
func joinCSVFieldValues(values []string) string {
	return strings.Join(values, "、")
}

/**
 * CSV送信日時整形。
 *
 * 送信日時をCSV保存用の固定形式へ変換する処理。
 */
func formatCSVSentAt(sentAt time.Time) string {
	if sentAt.IsZero() {
		return ""
	}

	return sentAt.Format(csvSentAtLayout)
}

/**
 * CSVレコード描画。
 *
 * レコード群をCSV文字列へ変換し、設定文字コードへエンコードする処理。
 */
func renderCSVRecords(records [][]string, encodingName string) ([]byte, error) {
	var csvBuffer bytes.Buffer
	csvWriter := csv.NewWriter(&csvBuffer)
	csvWriter.UseCRLF = true

	for _, record := range records {
		if err := csvWriter.Write(record); err != nil {
			return nil, fmt.Errorf("write csv record: %w", err)
		}
	}

	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		return nil, fmt.Errorf("flush csv record: %w", err)
	}

	encodedCSVBytes, err := charset.Encode(csvBuffer.String(), encodingName)
	if err != nil {
		return nil, err
	}

	return encodedCSVBytes, nil
}
