<div align="center">

![uk72-api](/web/default/public/logo.png)

# uk72-api

Modified open source distribution for the UK72 AI API gateway deployment.

<p align="center">
  English |
  <a href="./README.zh_CN.md">简体中文</a> |
  <a href="./README.zh_TW.md">繁體中文</a> |
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

## Project Overview

`uk72-api` is a modified distribution based on
[QuantumNous/new-api](https://github.com/QuantumNous/new-api).

This repository preserves the GNU Affero General Public License v3.0,
upstream copyright notices, and visible attribution to the original project.
It publishes the corresponding source code for the UK72 deployment at
<https://github.com/lll368/uk72-api>.

This project is not affiliated with or endorsed by QuantumNous. It does not
grant commercial license exceptions for upstream code.

## What This Repository Contains

- Source code and deployment assets for the UK72 modified distribution.
- Gateway, authorization, quota, billing, model, and channel management code as
  implemented in this repository.
- Local build and deployment instructions for this distribution.

Because this project is based on `new-api`, some upstream modules and
capabilities may still exist in the codebase. Actual availability depends on
your configuration, upstream providers, and deployment environment.

This README is limited to this distribution's source, attribution, deployment,
and legal notes. It does not reproduce upstream ecosystem claims that are not
maintained by UK72.

## Source And Attribution

| Item | Link |
| --- | --- |
| UK72 source code | <https://github.com/lll368/uk72-api> |
| Upstream project | <https://github.com/QuantumNous/new-api> |
| Original base project | <https://github.com/songquanpeng/one-api> |

## Quick Start

### Docker Compose

```bash
git clone https://github.com/lll368/uk72-api.git
cd uk72-api

# Review docker-compose.yml and environment variables before running.
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

## Configuration Notes

- Review `docker-compose.yml` before production deployment.
- Use environment variables or the admin console for runtime configuration.
- Keep API keys, payment credentials, OAuth secrets, and database credentials
  outside public repositories.
- When behavior differs from upstream documentation, this repository's source
  code and deployment configuration are authoritative for `uk72-api`.

## Legal And Compliance Notes

- Use only API keys, accounts, models, and upstream services that you are
  authorized to use.
- If you provide a public generative AI service, you are responsible for
  applicable registration, content safety, logging, real-name, tax, and
  upstream authorization obligations in your jurisdiction.
- Network deployment of a modified AGPLv3 application may require providing
  corresponding source code to users.

## License

This project is licensed under the
[GNU Affero General Public License v3.0](./LICENSE).

Additional terms under AGPLv3 Section 7 apply. Modified versions with a user
interface must preserve the author attribution notice
`Frontend design and development by New API contributors.` in an appropriate
legal, about, footer, or attribution location, and must keep a visible link to
the original project: <https://github.com/QuantumNous/new-api>.

## Acknowledgements

`uk72-api` is based on `QuantumNous/new-api`, which is based on
`songquanpeng/one-api`. This repository keeps the required license and
attribution notices while separating UK72's modified distribution from upstream
project claims.

## Support

For issues specific to this modified distribution, use:
<https://github.com/lll368/uk72-api/issues>
