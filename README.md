# sing-box-helper

Утилита для управления sing-box на роутерах Keenetic / Entware через простой веб-интерфейс.

**Статус:** v1.0 — полный 4-вкладочный UI, YAML-настройки, автодетект LAN-IP, `.ipk`-пакет, GitHub Actions release.

## Что это

`sing-box-helper` устанавливается рядом с уже работающим sing-box и берёт на себя:

- приём готовой ссылки на узел (`vless://`, `hysteria2://`, `socks5://`, …) и автоматическую генерацию корректного `config.json`;
- проверку, что туннель действительно работает (5-шаговая диагностика);
- откат к предыдущим версиям конфига (история бэкапов с авто-обрезкой);
- настройку переменных, специфичных для роутера, без правки JSON вручную (YAML + веб-форма).

Утилита **не** маршрутизирует трафик и не настраивает iptables — этим занимаются sing-box, ядро Linux и (на Keenetic) keen-pbr. Полное ТЗ — `docs/ТЗ — sing-box-helper.md` в родительском воркспейсе.

## Установка через opkg

На роутере (один раз):

```sh
# mipsel — старые Keenetic / Xiaomi на MT7621/MT7620 (Mi Router 3G и т.п.)
echo 'src/gz singbox-helper https://wolfam0108.github.io/sing-box-helper/mipsel-3.4' \
  >> /opt/etc/opkg.conf

# aarch64 — новые Keenetic Hopper / Hero / Giga
echo 'src/gz singbox-helper https://wolfam0108.github.io/sing-box-helper/aarch64-3.10' \
  >> /opt/etc/opkg.conf

opkg update
opkg install singbox-helper
```

После установки веб-UI доступен на `http://<LAN-IP-роутера>:8765`. Сервис управляется через `/opt/etc/init.d/S98singbox-helper {start|stop|restart|check}`. Конфигурация — `/opt/etc/singbox-helper/config.yaml` (создаётся при первом сохранении настроек), состояние — `/opt/etc/singbox-helper/state.json`. Оба файла переживают `opkg upgrade`.

Обновления:

```sh
opkg update
opkg upgrade singbox-helper
```

Удаление:

```sh
opkg remove singbox-helper
# runtime-данные в /opt/etc/singbox-helper/ остаются — удалите вручную при необходимости
```

## Поддерживаемые URI-схемы

| Схема | Что это | Замечания |
|---|---|---|
| `hysteria2://` / `hy2://` | Hysteria2 | sni / insecure / alpn / obfs |
| `vless://` (vanilla) | VLESS over tcp/ws/grpc/h2/httpupgrade | security=none/tls |
| `vless://` + REALITY | VLESS+REALITY (+Vision) | pbk / sid / fp / sni / flow |
| `vless://` + xhttp | VLESS+xhttp+REALITY | маппится в `httpupgrade` с предупреждением; sing-box 1.13 не имеет нативного xhttp |
| `socks://` / `socks4://` / `socks4a://` / `socks5://` / `socks5h://` | SOCKS4/4a/5 | сценарий «sing-box как обёртка над локальным naive-proxy / mieru / xray-core» |

Прочие протоколы (Trojan, VMess, Shadowsocks, TUIC, AnyTLS) — на v2.

## Веб-интерфейс

Четыре вкладки, всё embed'ом в один бинарь (~7.5 МБ под mipsle):

1. **Главная** — статус (sing-box / TUN / текущий узел), форма Apply (URI → Preview / Apply / Test), 5-шаговая проверка работы.
2. **Настройки** — mixed-proxy (auto/127/0.0.0.0/custom), TUN (iface/addr/MTU/stack), DNS, Clash API, log level. Сохранение пишет YAML и пересобирает `config.json` + рестартит sing-box, если есть активный узел.
3. **Логи** — `sing-box` (через `ndmc -c "show log once"` с фильтром) или `helper` (in-process ring buffer); 100/200/500/1000 строк; авто-обновление 3 сек.
4. **Бэкапы** — список snapshots с распознанным `protocol server:port`, кнопки «Откатиться» (создаёт пре-restore снимок) и «Удалить». Auto-trim до 10 штук.

## HTTP API

| Метод | Путь | Что делает |
|---|---|---|
| `GET`  | `/api/status` | Состояние sing-box, TUN, текущий узел (источник истины — `config.json` + метаданные из `state.json`). |
| `POST` | `/api/preview` | Парсит URI, возвращает `display` + готовый `config.json`. Без записи. |
| `POST` | `/api/apply` | Pre-reach-check → бэкап → запись `config.json` → restart sing-box → `state.json`. |
| `GET`  | `/api/test` | 5 диагностических шагов: доступность узла, sing-box процесс, TUN, прямой IP, IP через TUN. |
| `GET`  | `/api/settings` | Текущие настройки + `mixed_listen_effective` (резолв `auto`). |
| `POST` | `/api/settings` | Валидация → запись YAML → swap in-memory → при наличии активного узла — re-render + restart. |
| `GET`  | `/api/logs?source=&lines=` | `source=singbox` (через ndmc) или `source=helper` (ring buffer). |
| `GET`  | `/api/backups` | Список бэкапов newest-first с summary и size. |
| `POST` | `/api/backups/restore` | `{"file":"..."}` — снимает текущий конфиг, восстанавливает из бэкапа, рестартит sing-box. |
| `DELETE` | `/api/backups?file=...` | Удалить один бэкап. Только файлы из той же директории, что и `ConfigPath`. |

## Запуск CLI

```bash
# Печать готового config.json
go run ./cmd/singbox-helper --from-uri 'hysteria2://pw@host:443'

# Запись на диск (с автоматическим бэкапом старого файла)
go run ./cmd/singbox-helper \
  --from-uri 'vless://uuid@host:443?type=tcp&security=reality&pbk=...&fp=chrome&sni=ya.ru' \
  --apply

# Локальный SOCKS-клиент (naive-proxy / mieru / xray-core)
go run ./cmd/singbox-helper --from-uri 'socks5://127.0.0.1:1080#mieru-local'
```

## Запуск сервера

```bash
go run ./cmd/singbox-helper --serve --listen 0.0.0.0:8765
# опционально:
#   --settings /opt/etc/singbox-helper/config.yaml
#   --out /opt/etc/sing-box/config.json   (только для CLI-режима)
```

UI откроется по `http://<IP-роутера>:8765`.

## Сборка из исходников

Кросс-сборка бинаря (для разработки / запуска без `.ipk`):

```bash
GOOS=linux GOARCH=mipsle GOMIPS=softfloat \
  go build -trimpath -ldflags='-s -w' -o singbox-helper-mipsle ./cmd/singbox-helper

# Заливка на роутер (sftp-server в Entware-OpenSSH нет, нужен -O)
scp -O singbox-helper-mipsle keen01:/opt/bin/singbox-helper
```

Размер: ~7.5 МБ.

Сборка `.ipk` локально (требует bash, tar, sed — git-bash / WSL / Linux):

```bash
BINARY=./singbox-helper-mipsle PKGVER=0.0.0-dev PKGARCH=mipsel-3.4 \
  ./scripts/build-ipk.sh
# → dist/singbox-helper_0.0.0-dev_mipsel-3.4.ipk
```

Релизный билд под все архитектуры делает CI (`.github/workflows/release.yml`):

```bash
git tag v1.0.0
git push --tags
```

После пуша тега GitHub Actions собирает `.ipk` под каждую целевую архитектуру, создаёт GitHub Release с прицепленными файлами, и публикует обновлённый opkg-фид в ветку `gh-pages`. Через ~2 минуты пользователи могут сделать `opkg update && opkg upgrade singbox-helper`.

## YAML-настройки

`/opt/etc/singbox-helper/config.yaml`:

```yaml
log_level: info
log_timestamp: true
upstream_dns: 1.1.1.1
dns_strategy: ipv4_only
tun_interface_name: singtun
tun_address: 198.18.0.1/30
tun_mtu: 1500
tun_stack: gvisor
enable_mixed: true
mixed_listen: auto            # auto | 0.0.0.0 | 127.0.0.1 | <IP>
mixed_listen_port: 7891
enable_clash_api: true
clash_api_listen: 0.0.0.0:9090
clash_api_ui_dir: /opt/share/dashboard
```

`mixed_listen: auto` резолвится при каждом рендере через `ip -4 addr show br0` (на Entware ищется в `/opt/sbin/ip` поверх busybox). Если детект не сработал — fallback в `0.0.0.0`.

Файл можно либо править руками, либо через вкладку «Настройки». Отсутствие файла = встроенные defaults. Частичный файл сливается с defaults — добавление новых полей в код обратносовместимо.

## State-файл

`/opt/etc/singbox-helper/state.json` хранит метаданные, которые сам sing-box не знает:

```json
{
  "uri":   "vless://...#users-wolframM1",
  "label": "users-wolframM1",
  "applied_at": "2026-05-12T12:47:59Z"
}
```

`/api/status` сверяет `server:port` из `state.json` с тем, что реально в `config.json`. Если совпадает — `managed: true` (можно показать URI + label + время применения), иначе `managed: false` («конфиг не управляется через эту утилиту»).

## Архитектура

```
cmd/singbox-helper/main.go          — entrypoint, flag-парсинг, settings load, stderr→ring buffer
internal/parser/                    — парсеры URI → Outbound + Display
  hysteria2.go vless.go socks.go parser.go (dispatch)
internal/config/                    — рендер config.json + YAML Settings
internal/probe/                     — reach / tunnel-ip / LAN-IP / sing-box status
internal/state/                     — load/save state.json (URI + label + applied_at)
internal/backup/                    — list/create/restore/delete/trim для config.json.bak-*
internal/logbuf/                    — thread-safe ring buffer для /api/logs?source=helper
internal/web/                       — HTTP API + embed.FS веб-UI
  assets/   index.html style.css app.js
```

Принципы:
- **Один бинарь** — CLI и HTTP-сервер в одной программе (`--serve` флаг).
- **Источник истины — реальные файлы.** `/api/status` парсит `config.json` на каждый запрос, не доверяет in-memory кэшу.
- **Парсер строгий.** Невалидный URI → отказ с понятной ошибкой, никаких «как-нибудь да соберём».
- **TUN всегда `auto_route:false`, `strict_route:false`** — keen-pbr сам решает маршрутизацию, конфликт недопустим.
- **Тег outbound всегда `proxy`** — `route.final = "proxy"`, единая модель.

## Запуск тестов

```bash
go test ./...
```

~93 теста по всем пакетам, все зелёные.

## Лицензия

[MIT](LICENSE)
