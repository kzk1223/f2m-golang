# f2m-golang

f2m-golang は、通常の HTML フォームと `f2m_conf.txt` によって構成できる Go 製の汎用メールフォームである。

旧 PHP 版 f2m の設定キーと基本的な運用思想を引き継ぎつつ、Go の標準機能を中心に再設計する。

## 目的
非PGがHTMLへの簡単な記載＋設定ファイルの作成でメールフォームを設定できることを目指している

## 基本設計

- フォームごとの設定は `F2M_ID` を単位として管理する
- 設定ファイルは `KEY : VALUE` 形式とする
- 処理本体は `internal` 配下に集約する
- CGI 版と常駐 HTTP サーバー版で共通の `http.Handler` を利用する
- 内部処理の文字コードは UTF-8 に統一する
- 入力画面、確認画面、完了画面は HTML テンプレートで差し替え可能とする

## 実行方式

常駐 HTTP サーバー版では、`net/http` により HTTP サーバーとして起動する。

```text
cmd/server/main.go
```

CGI 版では、`net/http/cgi` により共通 Handler を CGI として実行する設計である。

```text
cmd/cgi/main.go
```

## 画面遷移

```text
入力画面
  ↓
入力チェック
  ↓
確認画面
  ↓
送信処理
  ↓
管理者通知メール送信
  ↓
自動返信メール送信
  ↓
CSV保存
  ↓
完了画面
```

入力チェックでエラーがある場合は、入力画面を再表示し、入力値、エラー一覧、項目別エラーを表示する。

## 設定ファイル

設定ファイルは `f2m_conf.txt` を想定する。

```text
F2M_ID      : contact
F2M_FROM    : form@example.com
F2M_TO      : admin@example.com
F2M_SUBJECT : お問い合わせがありました
F2M_CHK     : name,mail
F2M_JPNAME  : name:お名前,mail:メールアドレス
```

主な設定項目は以下である。

```text
F2M_ID
F2M_FROM
F2M_TO
F2M_SUBJECT
F2M_CHK
F2M_JPNAME
F2M_CHK_EMAIL
F2M_CHK_EQ
F2M_RESV_TO_FLD
F2M_RESV_SUBJECT
F2M_RESV_TMPL
F2M_MAIL_TMPL
F2M_CSV
F2M_FORM
F2M_CONFIRM
F2M_THANKS
```

## 文字コード

設定ファイル、HTML テンプレート、メールテンプレートは以下を読み込み対象とする。

```text
UTF-8
UTF-8 BOM
Shift_JIS
CP932
```

読み込み後は UTF-8 文字列として内部処理する。CSV 出力文字コードは `F2M_CSV_CHARSET` に従う。

## テンプレート

テンプレートは Go 標準の `html/template` を利用する。

```text
form.html
confirm.html
thanks.html
mail.txt
reply.txt
```

フォーム HTML には、エラー表示、入力値復元、CSRF トークンをシステム側で補完する。

## 入力チェック

以下の検証を行う。

```text
必須チェック
メール形式チェック
一致チェック
添付ファイルサイズチェック
添付ファイル拡張子チェック
添付ファイルMIMEチェック
```

## メール送信

`F2M_TO` が設定されている場合、管理者通知メールを送信する。

`F2M_RESV_TO_FLD` が設定されている場合、指定フィールドのメールアドレス宛に自動返信メールを送信する。

## CSV 保存

`F2M_CSV` が設定されている場合、送信内容を CSV に保存する。

初回作成時は `F2M_JPNAME` の表示名をヘッダーとして出力する。checkbox の複数値は `、` 区切りで 1 セルに保存する。

## セキュリティ

- CSRF トークンを検証する
- 確認画面からの送信値は署名付きトークンで検証する
- HTML 出力はエスケープする
- メールヘッダへの改行混入を禁止する
- 添付ファイルは公開ディレクトリ外に一時保存する
- SMTP パスワードなどの秘密情報はログに出力しない
- 設定ファイルの解析エラーは行番号付きで返す

## 内部構成

```text
cmd/server       常駐 HTTP サーバー入口
cmd/cgi          CGI 入口
internal/config  設定ファイル解析
internal/charset 文字コード判定と UTF-8 正規化
internal/form    入力値、検証、画面遷移
internal/mailer  メール送信
internal/storage CSV 保存と添付ファイル管理
internal/security CSRF、署名、送信制御
```
