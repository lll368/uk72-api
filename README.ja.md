<div align="center">

![uk72-api](/web/default/public/logo.png)

# uk72-api

UK72 AI API ゲートウェイ配信用のオープンソース改変ディストリビューション。

<p align="center">
  <a href="./README.md">English</a> |
  <a href="./README.zh_CN.md">简体中文</a> |
  <a href="./README.zh_TW.md">繁體中文</a> |
  <a href="./README.fr.md">Français</a> |
  日本語
</p>

<p align="center">
  <a href="https://github.com/lll368/uk72-api/blob/main/LICENSE">
    <img src="https://img.shields.io/github/license/lll368/uk72-api?color=brightgreen" alt="license">
  </a><!--
  --><a href="https://github.com/lll368/uk72-api">
    <img src="https://img.shields.io/badge/source-uk72--api-blue" alt="source">
  </a><!--
  --><a href="https://github.com/QuantumNous/new-api">
    <img src="https://img.shields.io/badge/upstream-QuantumNous%2Fnew--api-lightgrey" alt="upstream">
  </a>
</p>

</div>

## プロジェクト概要

`uk72-api` は
[QuantumNous/new-api](https://github.com/QuantumNous/new-api)
をベースにした改変ディストリビューションです。

このリポジトリは GNU Affero General Public License v3.0、upstream の著作権表示、
および元プロジェクトへの可視の帰属表示を保持しています。UK72 配信に対応する
ソースコードは <https://github.com/lll368/uk72-api> で公開しています。

本プロジェクトは QuantumNous に所属せず、QuantumNous による承認または推奨を
意味しません。また、upstream コードの商用ライセンス例外を付与するものでは
ありません。

## このリポジトリに含まれるもの

- UK72 改変ディストリビューションのソースコードとデプロイ関連ファイル。
- このリポジトリで実装されているゲートウェイ、認可、クォータ、課金、モデル、
  チャネル管理コード。
- このディストリビューション向けのローカルビルドおよびデプロイ手順。

本プロジェクトは `new-api` をベースにしているため、コード内には upstream の
モジュールや機能が残っている場合があります。実際に利用できる機能は、設定、
upstream プロバイダー、およびデプロイ環境に依存します。

この README は、このディストリビューションのソースコード、帰属、デプロイ、
法務情報に限定しています。UK72 が維持していない upstream エコシステムの主張は
繰り返しません。

## ソースコードと帰属

| 項目 | リンク |
| --- | --- |
| UK72 ソースコード | <https://github.com/lll368/uk72-api> |
| Upstream プロジェクト | <https://github.com/QuantumNous/new-api> |
| 元のベースプロジェクト | <https://github.com/songquanpeng/one-api> |

## クイックスタート

### Docker Compose

```bash
git clone https://github.com/lll368/uk72-api.git
cd uk72-api

# 実行前に docker-compose.yml と環境変数を確認してください。
docker compose up -d
```

### Docker

```bash
docker build -t uk72-api:local .

docker run --name uk72-api -d --restart always \
  -p 3000:3000 \
  -e TZ=Asia/Shanghai \
  -v ./data:/data \
  uk72-api:local
```

## 設定メモ

- 本番デプロイ前に `docker-compose.yml` を確認してください。
- 実行時設定は環境変数または管理コンソールで設定できます。
- API Key、決済認証情報、OAuth Secret、データベース認証情報などの機密情報を
  公開リポジトリにコミットしないでください。
- 動作が upstream ドキュメントと異なる場合、`uk72-api` についてはこの
  リポジトリのソースコードとデプロイ設定を正とします。

## 法務およびコンプライアンス

- 利用権限を持つ API Key、アカウント、モデル、upstream サービスのみを使用して
  ください。
- 公開の生成 AI サービスを提供する場合、所在地で必要な登録、許可、コンテンツ
  安全、ログ保存、本人確認、税務、upstream 認可などの義務は利用者が負います。
- 改変した AGPLv3 アプリケーションをネットワーク経由で提供する場合、対応する
  ソースコードをユーザーへ提供する義務が生じることがあります。

## ライセンス

本プロジェクトは [GNU Affero General Public License v3.0](./LICENSE) の下で
ライセンスされています。

AGPLv3 第 7 条に基づく追加条件が適用されます。ユーザーインターフェースを持つ
改変版は、`Frontend design and development by New API contributors.` という
著作者帰属表示を、適切な法的表示、About、フッター、または帰属表示の場所に保持し、
元プロジェクトへの可視リンクを保持する必要があります：
<https://github.com/QuantumNous/new-api>。

## 謝辞

`uk72-api` は `QuantumNous/new-api` をベースにしており、
`QuantumNous/new-api` は `songquanpeng/one-api` をベースにしています。この
リポジトリは必要なライセンスと帰属表示を保持しつつ、UK72 の改変ディストリビューション
を upstream プロジェクト自身の主張から分離します。

## サポート

この改変ディストリビューション固有の問題は、以下を使用してください：
<https://github.com/lll368/uk72-api/issues>
