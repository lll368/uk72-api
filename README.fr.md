<div align="center">

![uk72-api](/web/default/public/logo.png)

# uk72-api

Distribution open source modifiee pour le deploiement de la passerelle AI API
UK72.

<p align="center">
  <a href="./README.md">English</a> |
  <a href="./README.zh_CN.md">简体中文</a> |
  <a href="./README.zh_TW.md">繁體中文</a> |
  Français |
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

## Presentation du projet

`uk72-api` est une distribution modifiee basee sur
[QuantumNous/new-api](https://github.com/QuantumNous/new-api).

Ce depot conserve la licence GNU Affero General Public License v3.0, les avis
de copyright upstream et l'attribution visible au projet original. Il publie le
code source correspondant au deploiement UK72 sur
<https://github.com/lll368/uk72-api>.

Ce projet n'est pas affilie a QuantumNous et n'est pas approuve par
QuantumNous. Il ne peut pas accorder d'exception de licence commerciale pour le
code upstream.

## Contenu de ce depot

- Code source et fichiers de deploiement de la distribution modifiee UK72.
- Code de passerelle, autorisation, quotas, facturation, modeles et canaux tel
  qu'il est implemente dans ce depot.
- Instructions locales de construction et de deploiement pour cette
  distribution.

Comme ce projet est base sur `new-api`, certains modules et capacites upstream
peuvent encore exister dans le code. Leur disponibilite depend de votre
configuration, des fournisseurs upstream et de l'environnement de deploiement.

Ce README se limite a la source, l'attribution, le deploiement et les notes
legales de cette distribution. Il ne reprend pas les declarations de
l'ecosysteme upstream qui ne sont pas maintenues par UK72.

## Source et attribution

| Element | Lien |
| --- | --- |
| Code source UK72 | <https://github.com/lll368/uk72-api> |
| Projet upstream | <https://github.com/QuantumNous/new-api> |
| Projet de base original | <https://github.com/songquanpeng/one-api> |

## Demarrage rapide

### Docker Compose

```bash
git clone https://github.com/lll368/uk72-api.git
cd uk72-api

# Verifiez docker-compose.yml et les variables d'environnement avant execution.
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

## Notes de configuration

- Verifiez `docker-compose.yml` avant un deploiement en production.
- Utilisez les variables d'environnement ou la console d'administration pour la
  configuration d'execution.
- Ne publiez pas les API keys, identifiants de paiement, secrets OAuth ou
  identifiants de base de donnees dans un depot public.
- Si le comportement differe de la documentation upstream, le code source et la
  configuration de ce depot font autorite pour `uk72-api`.

## Notes legales et conformite

- Utilisez uniquement les API keys, comptes, modeles et services upstream pour
  lesquels vous disposez d'une autorisation.
- Si vous fournissez un service public d'IA generative, vous etes responsable
  des obligations applicables dans votre juridiction.
- Le deploiement reseau d'une application AGPLv3 modifiee peut vous obliger a
  fournir le code source correspondant aux utilisateurs.

## Licence

Ce projet est sous licence
[GNU Affero General Public License v3.0](./LICENSE).

Les conditions supplementaires de la section 7 de l'AGPLv3 s'appliquent. Les
versions modifiees avec interface utilisateur doivent conserver l'avis
d'attribution `Frontend design and development by New API contributors.` dans
un emplacement juridique, a propos, pied de page ou attribution approprie, et
conserver un lien visible vers le projet original :
<https://github.com/QuantumNous/new-api>.

## Remerciements

`uk72-api` est base sur `QuantumNous/new-api`, lui-meme base sur
`songquanpeng/one-api`. Ce depot conserve les avis de licence et d'attribution
necessaires tout en separant la distribution modifiee UK72 des declarations du
projet upstream.

## Support

Pour les problemes propres a cette distribution modifiee, utilisez :
<https://github.com/lll368/uk72-api/issues>
