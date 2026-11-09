<div align="center">
  <img src="design/logo/avatar.svg" width="112" alt="Логотип OpenFlux">
  <h1>OpenFlux Android</h1>
  <p>Ядро транспортов и выходная нода для туннеля поверх «белых» сервисов.</p>
  <p>
    <a href="https://github.com/Oleglog/OpenFlux-Android/releases/latest"><img src="https://img.shields.io/github/v/release/Oleglog/OpenFlux-Android?display_name=tag&amp;sort=semver&amp;style=flat-square&amp;color=7aa2f7" alt="Последний релиз"></a>
    <a href="https://github.com/Oleglog/OpenFlux-Android/actions/workflows/ci.yml"><img src="https://github.com/Oleglog/OpenFlux-Android/actions/workflows/ci.yml/badge.svg" alt="Статус CI"></a>
    <a href="LICENSE"><img src="https://img.shields.io/github/license/Oleglog/OpenFlux-Android?style=flat-square" alt="Лицензия GPL-3.0"></a>
  </p>
  <p><a href="README.md">English</a> · <strong>Русский</strong></p>
</div>

> Форк [p1neappleXpress/OpenFlux](https://github.com/p1neappleXpress/OpenFlux). Исходный проект — туннель через совместное редактирование документов (Яндекс.Документы, Mail.ru Docs). Здесь он вырос в настраиваемое ядро: несколько носителей, полоса из нескольких каналов с ARQ, сквозное шифрование и изолированные режимы выходной ноды.

Этот репозиторий — **Go-ядро и бинарник `openflux`**. Android-приложение, которым пользуется человек, живёт отдельно: [OlConnect_client](https://github.com/Oleglog/OlConnect_client). Панель управления и сервер подписок — тоже отдельно: [OlConnect_manager](https://github.com/Oleglog/OlConnect_manager).

```text
Телефон или SOCKS5-клиент -> носитель (Доски, Cups, ...) -> выходная нода -> интернет
```

## Что умеет

- **Носители (транспорты)** — канал, внутри которого едет наш трафик. Подробности и статус в таблице ниже.
- **Полоса из нескольких каналов.** `striped.go` режет поток на куски, раскладывает их по каналам, восстанавливает порядок и переспрашивает потерянное (ARQ с окном переупорядочивания и повторами). Один канал не может быть быстрее своей задержки — полоса получается из нескольких.
- **Кодек пакетов.** `batched` (по умолчанию) — склейка пакетов и zstd, подсказки о размере батча и окне склейки пробрасываются через все обёртки; `legacy` — по пакету, LZ4.
- **Сквозное шифрование AES-256-GCM** с деривацией ключа через scrypt и защитой от повторного воспроизведения. Для Досков ключ **обязателен**: доска отбрасывает кадры, в которых видно IP-пакеты.
- **Два режима выходной ноды**: `l4` (по умолчанию, gVisor, TCP в user-space, наружу через SOCKS5) и `l3` (сырые сокеты ядра, L3 IP-пакеты, тот же кадр, что у телефона). `l3` нужен для телефонного пути, `l4` безопасен на изолированном стенде.
- **Клиент для проверки без телефона**: `--client` поднимает SOCKS5, `--tun` гоняет настоящий IP-трафик через существующий TUN-интерфейс — так проверяется именно телефонный путь.

## Носители и их состояние

| Носитель | `--transport` | Состояние | Замеры на стенде |
| --- | --- | --- | --- |
| Яндекс Доски | `boards` | **Основной, самый быстрый** | 1 МБ за 1.56 с (641 КБ/с), 10 МБ — 694 КБ/с, на 60 с — 952 КБ/с, `refused=0` |
| Cups | `cups` / `cupsonline` | **Рабочий** | 10 МБ за 136 с — 71.8 КБ/с при 8 комнатах (до исправления кадрирования — 11 КБ/с и остановка на 40–45 с) |
| Яндекс.Документы (Волга) | `volga` / `vyandex` | Работает, но медленно — резерв | 5–7 КБ/с |
| Mail.ru Docs | `mailru` | Работает, но медленно — резерв | 6–8 КБ/с |
| Telemost | `telemost` | **Закрыт**: внешний потолок SFU, не наша сторона | — |
| Avito | `avito` | Сломан на стороне сервиса | — |
| Bitrix24 | `bitrix24` / `b24` | Экспериментальный | — |
| MAX / OneMe | `oneme` | Не реализовано: нужны `--maxToken` и `--maxUid` | — |

Цифры получены на живом VPS, один и тот же путь, что у телефона. Доски быстрее всех за счёт того, что объект уходит одним большим значением (до 48 КиБ, у нас 45000 байт с запасом под ограничение доски в 65536 символов).

## Быстрый запуск выходной ноды

```bash
# бинарник из последнего релиза
gh release download v1.0.63 --repo Oleglog/OpenFlux-Android --pattern 'openflux-linux-amd64'
install -m 755 openflux-linux-amd64 /usr/local/bin/openflux

# на Linux нужен RST-drop, иначе ядро отвечает на чужие пакеты:
iptables -A OUTPUT -p tcp --tcp-flags RST RST -j DROP

# Доски: ключ обязателен, ссылка на доску — как у клиента
openflux --exit-node --mode l4 --transport boards \
  --url "https://docs.yandex.ru/..." \
  --encryption-key "$(cat /etc/openflux/key.hex)" \
  --socks5 127.0.0.1:1080
```

`l3`-режим (телефонный путь) требует root и свободного TUN-устройства; `l4` обходится без сырых сокетов, поэтому годится для тестов на общем сервере.

## Клиент для проверки и для компьютера

```bash
# SOCKS5-клиент на компьютере
openflux --client --transport boards --url "..." --encryption-key "..." --socks5 127.0.0.1:1080

# телефонный путь: настоящий IP через готовый TUN
ip tuntap add dev ofxb mode tun && ip link set ofxb up
ip addr add 10.10.10.2/32 dev ofxb
ip route replace default dev ofxb table 100
ip rule add priority 100 from 10.10.10.2 lookup 100
openflux --client --transport boards --url "..." --encryption-key "..." --tun ofxb
curl --interface 10.10.10.2 https://example.com
```

## Флаги

| Флаг | По умолчанию | Что делает |
| --- | --- | --- |
| `--exit-node` | выкл. | Выходная нода (нужен root для `l3`) |
| `--client` | выкл. | Клиент: SOCKS5 или `--tun` |
| `--mode` | `l4` | `l4` — gVisor и SOCKS5, `l3` — сырые сокеты ядра |
| `--transport` | `yandex` | `boards`, `cups`, `volga`, `mailru`, `telemost`, `avito`, `bitrix24`, `oneme`, `yandex` |
| `--url` / `--url-file` | пусто | Ссылка на документ/доску/комнату |
| `--encryption-key` / `--encryption-key-file` | пусто | Ключ AES-256-GCM |
| `--codec` | `batched` | `batched` (zstd и склейка) или `legacy` (LZ4 по пакету) |
| `--socks5` | `:1080` | Адрес SOCKS5 |
| `--tun` | пусто | Клиент поверх существующего TUN (телефонный кадр, нужен `l3`-выход) |
| `--local-ip` | пусто | Исходящий IP выходной ноды |
| `--warp-proxy` | пусто | WARP/SOCKS5 для выходного трафика в `l4` |
| `--socks-proxy` | пусто | Прокси для сигналинга транспорта |
| `--cookies` / `--cookies-file` | пусто | Netscape cookies.txt для Волги и Mail.ru |
| `--maxToken` / `--maxUid` | пусто | MAX, транспорт пока не реализован |
| `--debug` | выкл. | Подробные логи |

Отдельные `STRIPE_*` и `OPENFLUX_CUPS_*` переменные окружения управляют полосой и пейсером носителя; их значения по умолчанию подобраны замерами и менять их без стенда не стоит.

## Сборка и релизы

Локальные сборки и тесты в этом проекте **не запускаются** — только GitHub Actions: `ci.yml` (тесты и линтеры) и `release.yml` (бинарники Linux amd64/arm64, Windows, debug-APK). Тег `v1.0.x` — релиз; собрать локально можно только для проверки синтаксиса.

Релизы консолидированы: вся история версий от `v1.0.0` до текущей описана в одном глобальном релизе, теги и история git сохранены.

## Что больше не работает и почему

- **Telemost** — закрыт. Потолок на стороне SFU: он теряет 10–30 % ретранслируемого в пачках, и никакая наша настройка это не меняет.
- **Avito** — сломан на стороне сервиса, воспроизводится без нашего участия.
- **Волга и Mail.ru** — не сломаны, но их 5–8 КБ/с недостаточно: держим как резерв, работы по ним не ведутся.
- **MAX / OneMe** — транспорт не реализован, нужны токены и идентификаторы звонка.
- **Отдельные головы Android-клиента в этом репозитории** — наследие форка: рабочий клиент теперь в [OlConnect_client](https://github.com/Oleglog/OlConnect_client), здесь остаются только debug-сборки для совместимости.

## Экспериментальное

Дизайн-эксперимент «Рейка» (новая визуальная система панели и клиента) откатан из основного кода и лежит в ветках `experimental/design-rail-2026-11` в репозиториях клиента и панели, макеты — вне репозиториев. В основном коде его нет.

## Связанные репозитории

- [OlConnect_client](https://github.com/Oleglog/OlConnect_client) — Android-приложение (VPNService, профили, подписки, оформление).
- [OlConnect_manager](https://github.com/Oleglog/OlConnect_manager) — панель управления, лаунчер инстансов, сервер подписок и зеркало.

## Лицензия

GNU General Public License v3.0 или более поздняя. См. [LICENSE](LICENSE), [COPYRIGHT](COPYRIGHT), [NOTICE](NOTICE). Форк не одобрен компаниями Яндекс или VK (Mail.ru) и не связан с ними.
