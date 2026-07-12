<div align="center">

![uk72-api](/web/default/public/logo.png)

# uk72-api

面向 UK72 AI API 閘道部署的開源修改發行版。

<p align="center">
  <a href="./README.zh_CN.md">简体中文</a> |
  繁體中文 |
  <a href="./README.md">English</a> |
  <a href="./README.fr.md">Français</a> |
  <a href="./README.ja.md">日本語</a>
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

## 專案說明

`uk72-api` 是基於
[QuantumNous/new-api](https://github.com/QuantumNous/new-api)
的修改發行版。

本倉庫保留 GNU Affero 通用公共許可證 v3.0、上游版權聲明和原始專案可見歸屬，
並在 <https://github.com/lll368/uk72-api> 發布 UK72 部署對應源碼。

本專案不隸屬於 QuantumNous，不代表 QuantumNous，也不暗示獲得其背書或上游程式碼
商業授權例外。

## 本倉庫包含什麼

- UK72 修改發行版的源碼和部署相關檔案。
- 本倉庫目前實作的閘道、鑑權、額度、帳單、模型和渠道管理程式碼。
- 面向本發行版的本地建置和部署說明。

由於本專案基於 `new-api`，程式碼中仍可能包含上游模組和能力。實際可用能力取決
於你的配置、上游服務商和部署環境。

本 README 只保留本發行版的源碼、歸屬、部署和法律說明，不再複述不由 UK72 維護
的上游生態聲明。

## 源碼與歸屬

| 項目 | 連結 |
| --- | --- |
| UK72 源碼 | <https://github.com/lll368/uk72-api> |
| 上游專案 | <https://github.com/QuantumNous/new-api> |
| 原始基礎專案 | <https://github.com/songquanpeng/one-api> |

## 快速開始

### Docker Compose

```bash
git clone https://github.com/lll368/uk72-api.git
cd uk72-api

# 執行前請先檢查 docker-compose.yml 和環境變數。
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

## 配置說明

- 生產部署前請檢查 `docker-compose.yml`。
- 執行時配置可以透過環境變數或管理後台設定。
- API Key、支付密鑰、OAuth Secret、資料庫帳號等敏感資訊不要提交到公開倉庫。
- 當行為與上游文件不一致時，以本倉庫源碼和部署配置作為 `uk72-api` 的準確資訊。

## 法律與合規說明

- 只使用你合法取得授權的 API Key、帳號、模型和上游服務。
- 如果你面向公眾提供生成式人工智慧服務，需要自行承擔所在地要求的備案、許可、
  內容安全、日誌留存、實名、稅務和上游授權等合規義務。
- 透過網路提供 AGPLv3 修改版服務時，可能需要向使用者提供對應源碼。

## 許可證

本專案採用 [GNU Affero 通用公共許可證 v3.0](./LICENSE) 授權。

AGPLv3 第 7 條下的附加條款適用。帶使用者介面的修改版本必須在合適的法律聲明、
關於、頁腳或歸屬位置保留作者歸屬聲明
`Frontend design and development by New API contributors.`，並保留指向原始專案
的可見連結：<https://github.com/QuantumNous/new-api>。

## 致謝

`uk72-api` 基於 `QuantumNous/new-api`，而 `QuantumNous/new-api` 基於
`songquanpeng/one-api`。本倉庫保留必要的許可證和歸屬聲明，同時將 UK72 的修改
發行版與上游專案自身聲明區分開。

## 支援

與本修改發行版相關的問題，請使用：
<https://github.com/lll368/uk72-api/issues>
