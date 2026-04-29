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

	mux := newServerMux(".", app.New(configSet))

	log.Printf("start server: http://%s", serverAddress)

	if err := http.ListenAndServe(serverAddress, mux); err != nil {
		log.Fatal(err)
	}
}

/**
 * 開発サーバールーター生成。
 *
 * GETを静的配信へ、POST / をフォーム処理へ振り分ける処理。
 */
func newServerMux(documentRoot string, formHandler http.Handler) http.Handler {
	mux := http.NewServeMux()
	fileServer := http.FileServer(http.Dir(documentRoot))

	// ---------------------------------------------
	// ルーティング
	// ---------------------------------------------
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/" {
			formHandler.ServeHTTP(w, r)
			return
		}

		fileServer.ServeHTTP(w, r)
	})

	return mux
}
