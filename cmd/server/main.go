package main

import (
	"log"
	"net/http"

	"f2m-golang/internal/app"
	"f2m-golang/internal/config"
)

const (
	configPath    = "./f2m_conf.txt"
	serverAddress = "127.0.0.1:8088"
)

/**
 * 開発確認用HTTPサーバー。
 *
 * 設定ファイルを読み込み、フォーム画面制御を起動する。
 */
func main() {
	configSet, err := config.LoadFile(configPath)
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()

	// ---------------------------------------------
	// ルーティング
	// ---------------------------------------------
	mux.Handle("/", app.New(configSet))

	log.Printf("start server: http://%s", serverAddress)

	if err := http.ListenAndServe(serverAddress, mux); err != nil {
		log.Fatal(err)
	}
}
