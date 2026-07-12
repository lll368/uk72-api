<div align="center">

![uk72-api](/web/default/public/logo.png)

# uk72-api

面向 UK72 AI API 网关部署的开源修改发行版。

<p align="center">
  简体中文 |
  <a href="./README.zh_TW.md">繁體中文</a> |
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

## 项目说明

`uk72-api` 是基于
[QuantumNous/new-api](https://github.com/QuantumNous/new-api)
的修改发行版。

本仓库保留 GNU Affero 通用公共许可证 v3.0、上游版权声明和原始项目可见归属，
并在 <https://github.com/lll368/uk72-api> 发布 UK72 部署对应源码。

本项目不隶属于 QuantumNous，不代表 QuantumNous，也不暗示获得其背书或上游代码
商业授权例外。

## 本仓库包含什么

- UK72 修改发行版的源码和部署相关文件。
- 本仓库当前实现的网关、鉴权、额度、账单、模型和渠道管理代码。
- 面向本发行版的本地构建和部署说明。

由于本项目基于 `new-api`，代码中仍可能包含上游模块和能力。实际可用能力取决
于你的配置、上游服务商和部署环境。

本 README 只保留本发行版的源码、归属、部署和法律说明，不再复述不由 UK72 维护
的上游生态声明。

## 源码与归属

| 项目 | 链接 |
| --- | --- |
| UK72 源码 | <https://github.com/lll368/uk72-api> |
| 上游项目 | <https://github.com/QuantumNous/new-api> |
| 原始基础项目 | <https://github.com/songquanpeng/one-api> |

## 快速开始

### Docker Compose

```bash
git clone https://github.com/lll368/uk72-api.git
cd uk72-api

# 运行前请先检查 docker-compose.yml 和环境变量。
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

## 配置说明

- 生产部署前请检查 `docker-compose.yml`。
- 运行时配置可以通过环境变量或管理后台设置。
- API Key、支付密钥、OAuth Secret、数据库账号等敏感信息不要提交到公开仓库。
- 当行为与上游文档不一致时，以本仓库源码和部署配置作为 `uk72-api` 的准确信息。

## 法律与合规说明

- 只使用你合法取得授权的 API Key、账号、模型和上游服务。
- 如果你面向公众提供生成式人工智能服务，需要自行承担所在地要求的备案、许可、
  内容安全、日志留存、实名、税务和上游授权等合规义务。
- 通过网络提供 AGPLv3 修改版服务时，可能需要向用户提供对应源码。

## 许可证

本项目采用 [GNU Affero 通用公共许可证 v3.0](./LICENSE) 授权。

AGPLv3 第 7 条下的附加条款适用。带用户界面的修改版本必须在合适的法律声明、
关于、页脚或归属位置保留作者归属声明
`Frontend design and development by New API contributors.`，并保留指向原始项目
的可见链接：<https://github.com/QuantumNous/new-api>。

## 致谢

`uk72-api` 基于 `QuantumNous/new-api`，而 `QuantumNous/new-api` 基于
`songquanpeng/one-api`。本仓库保留必要的许可证和归属声明，同时将 UK72 的修改
发行版与上游项目自身声明区分开。

## 支持

与本修改发行版相关的问题，请使用：
<https://github.com/lll368/uk72-api/issues>
