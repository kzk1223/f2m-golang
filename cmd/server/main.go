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

	mux := newServerMux(".", configSet)

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
func newServerMux(documentRoot string, configSet *config.ConfigSet) http.Handler {
	mux := http.NewServeMux()
	fileServer := http.FileServer(http.Dir(documentRoot))
	formHandler := app.New(configSet)

	// ---------------------------------------------
	// ルーティング
	// ---------------------------------------------
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if shouldHandleByApp(configSet, r) {
			formHandler.ServeHTTP(w, r)
			return
		}

		fileServer.ServeHTTP(w, r)
	})

	return mux
}

/**
 * アプリ側処理判定。
 *
 * POST送信とF2M_FORM対象GETをフォーム画面制御へ渡すかを返す処理。
 */
func shouldHandleByApp(configSet *config.ConfigSet, r *http.Request) bool {
	if r.Method == http.MethodPost && r.URL.Path == "/" {
		return true
	}

	return r.Method == http.MethodGet && app.HasFormPath(configSet, r.URL.Path)
}
