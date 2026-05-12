# sing-box-helper

Утилита для управления sing-box на роутерах Keenetic / Entware через простой веб-интерфейс.

**Статус: ранний этап разработки (MVP-4 завершён: парсер VLESS + REALITY + xhttp).**

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
| v0.5 | Рендер полного `config.json` + CLI-команда `--from-uri` | ⏳ |
| v1.0 | HTTP-сервер + веб-UI + `.ipk`-пакет для Keenetic | ⏳ |

## Запуск тестов

```bash
go test ./...
```

## Лицензия

[MIT](LICENSE)
