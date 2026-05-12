# sing-box-helper

Утилита для управления sing-box на роутерах Keenetic / Entware через простой веб-интерфейс.

**Статус: ранний этап разработки (v1.0-α: парсер + рендер + CLI + HTTP API без UI).**

## Что это

`sing-box-helper` устанавливается рядом с уже работающим sing-box и берёт на себя:

- приём готовой ссылки на узел (`vless://`, `hysteria2://`, …) и автоматическую генерацию корректного `config.json`;
- проверку, что туннель действительно работает (диагностика);
- откат к предыдущим версиям конфига;
- настройку переменных, специфичных для роутера, без правки JSON вручную.

Утилита **не** маршрутизирует трафик и не настраивает iptables — этим занимаются sing-box, ядро Linux и (на Keenetic) keen-pbr. См. полное ТЗ в `docs/ТЗ — sing-box-helper.md` родительского репозитория.

## План разработки

| Этап | Что | Статус |
|---|---|---|
| MVP-1 | Парсер `hysteria2://` + unit-тесты | ✅ |
| MVP-2 | Парсер `vless://` (vanilla TCP/TLS/WS/gRPC/h2/httpupgrade) + dispatch | ✅ |
| MVP-3 | Парсер `vless://` + REALITY (с uTLS fp и Vision flow) | ✅ |
| MVP-4 | Парсер `vless://` + REALITY + xhttp (маппинг в httpupgrade) | ✅ |
| v0.5 | Рендер полного `config.json` + CLI-команда `--from-uri` | ✅ |
| v1.0-α | HTTP API (`/api/status`, `/api/preview`, `/api/apply`, `/api/test`) без UI | ✅ |
| v1.0-β | Веб-UI (HTML/CSS/JS, embed-ассеты) | ⏳ |
| v1.0-γ | `.ipk`-пакет для Keenetic + GitHub Actions matrix-build | ⏳ |

## Запуск CLI

```bash
# Печать готового config.json
go run ./cmd/singbox-helper --from-uri 'hysteria2://pw@host:443'

# Запись на диск (с автоматическим бэкапом старого файла)
go run ./cmd/singbox-helper \
  --from-uri 'vless://uuid@host:443?type=tcp&security=reality&pbk=...&fp=chrome&sni=ya.ru' \
  --apply

# Кросс-сборка под Keenetic (mipsle, softfloat) — около 7.2 МБ
GOOS=linux GOARCH=mipsle GOMIPS=softfloat \
  go build -trimpath -ldflags='-s -w' -o singbox-helper-mipsle ./cmd/singbox-helper
```

## Запуск HTTP API (v1.0-α)

```bash
go run ./cmd/singbox-helper --serve --listen 0.0.0.0:8765
```

Доступные эндпоинты:

| Метод | Путь | Тело | Что делает |
|---|---|---|---|
| GET  | `/api/status` | — | состояние sing-box, TUN, текущий узел |
| POST | `/api/preview` | `{"uri":"..."}` | парсит URI, возвращает `display` + готовый `config.json` (без записи на диск) |
| POST | `/api/apply`   | `{"uri":"..."}` | пред-проверка доступности → бэкап → запись → restart sing-box |
| GET  | `/api/test` | — | 5 диагностических шагов на последнем applied узле |

Пример:

```bash
curl -sS -X POST -H 'Content-Type: application/json' \
  -d '{"uri":"hysteria2://pw@host:443"}' \
  http://192.168.1.1:8765/api/preview
```

## Запуск тестов

```bash
go test ./...
```

Текущий объём: 25 тестовых функций, все зелёные.

## Лицензия

[MIT](LICENSE)
