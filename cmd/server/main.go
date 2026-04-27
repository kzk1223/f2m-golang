package main

import (
"fmt"
"log"
"net/http"
)

/**
 * 開発確認用HTTPサーバー。
 *
 * ローカル開発環境でGoの起動確認を行う。
 */
func main() {
mux := http.NewServeMux()

// ---------------------------------------------
// ルーティング
// ---------------------------------------------
mux.HandleFunc("/", handleIndex)

addr := "127.0.0.1:8088"
log.Printf("start server: http://%s", addr)

if err := http.ListenAndServe(addr, mux); err != nil {
log.Fatal(err)
}
}

/**
 * 動作確認用レスポンスを返す。
 *
 * Goサーバーが起動していることを確認する。
 */
func handleIndex(w http.ResponseWriter, r *http.Request) {
// ---------------------------------------------
// レスポンス
// ---------------------------------------------
w.Header().Set("Content-Type", "text/plain; charset=utf-8")
fmt.Fprintln(w, "f2m-golang OK")
}
