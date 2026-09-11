<div align="center">
  <img src="design/logo/avatar.svg" width="112" alt="Логотип OpenFlux">
  <h1>OpenFlux Android</h1>
  <p>VPN через документ-транспорт для Android, компьютера и выходной Linux-ноды.</p>
  <p>
    <a href="https://github.com/Oleglog/OpenFlux-Android/releases/latest"><img src="https://img.shields.io/github/v/release/Oleglog/OpenFlux-Android?display_name=tag&amp;sort=semver&amp;style=flat-square&amp;color=7aa2f7" alt="Последний релиз"></a>
    <a href="https://github.com/Oleglog/OpenFlux-Android/actions/workflows/ci.yml"><img src="https://github.com/Oleglog/OpenFlux-Android/actions/workflows/ci.yml/badge.svg" alt="Статус CI"></a>
    <a href="LICENSE"><img src="https://img.shields.io/github/license/Oleglog/OpenFlux-Android?style=flat-square" alt="Лицензия GPL-3.0"></a>
    <img src="https://img.shields.io/badge/Android-8.0%2B-3DDC84?style=flat-square&amp;logo=android&amp;logoColor=white" alt="Android 8 или новее">
  </p>
  <p><a href="README.md">English</a> · <strong>Русский</strong></p>
</div>

![OpenFlux Android: подключение, логи и настройки](docs/images/openflux-android-tabs.png)

> Это форк проекта [p1neappleXpress/OpenFlux](https://github.com/p1neappleXpress/OpenFlux) с нативным клиентом Android VPN и поддержкой нового движка Яндекс.Документов («Волга»).

OpenFlux — исследовательский TCP-туннель с подключаемыми транспортами через совместное редактирование документов.

**[Скачать релизы (APK и бинарники сервера)](https://github.com/Oleglog/OpenFlux-Android/releases/latest)**

```text
Android VPN или SOCKS5-клиент -> Яндекс.Документы (Volga / Classic) -> Linux-нода -> интернет
```

## Возможности

- Android-клиент для Android 8+ на системном `VpnService` (ARM64, ARMv7, x86, x86_64);
- **Автоопределение движка Яндекса**: поддержка как нового редактора («Волга», транспорт `vyandex`), так и классического (`yandex`);
- **Без ключей шифрования**: полная совместимость с оригинальным протоколом OpenFlux;
- Готовые собранные бинарники для Linux VPS прямо в релизах;
- DNS-over-HTTPS на Android;
- SOCKS5-клиент для компьютера и режим выходной Linux-ноды.

---

## Быстрый запуск

### 1. Подготовка документа
1. Создайте текстовый документ на [Яндекс Диске](https://disk.yandex.ru/).
2. Откройте к нему доступ: **«Поделиться» → «Редактирование по ссылке»**.
3. Скопируйте ссылку на документ.

### 2. Запуск на сервере (VPS)
Скачайте готовый бинарник сервера из релизов:
```bash
wget https://github.com/Oleglog/OpenFlux-Android/releases/latest/download/openflux-linux-amd64 -O openflux
chmod +x openflux

# Заглушить RST-пакеты (обязательно):
sudo iptables -A OUTPUT -p tcp --tcp-flags RST RST -j DROP

# Запуск в фоне через nohup:
sudo nohup ./openflux --exit-node --url "ВАША_ССЫЛКА_НА_ДОКУМЕНТ" > openflux.log 2>&1 &
```
*(Проверить логи сервера: `tail -f openflux.log`, остановить: `sudo pkill -f openflux`)*.

### 3. Запуск на Android
1. Скачайте `OpenFlux-android-arm64-v8a-debug.apk` (или `universal`) из [релизов](https://github.com/Oleglog/OpenFlux-Android/releases/latest).
2. Во вкладке **«Настройки»** вставьте ссылку на ваш документ.
3. На главном экране нажмите **«Запустить VPN»**. Клиент автоматически определит тип редактора (Volga или классический) и подключится.
### 4. Автозапуск через systemd на сервере (по желанию)
```bash
sudo install -d -m 700 /root/openflux
sudo install -m 755 ./openflux /root/openflux/openflux
printf '%s\n' 'https://ссылка-на-ваш-документ' > /root/openflux/document-url
sudo install -m 644 deploy/openflux.service /etc/systemd/system/openflux.service
sudo systemctl daemon-reload
sudo systemctl enable --now openflux
sudo systemctl status openflux
```

## Сборка из исходников (для разработчиков)

```bash
# Сборка сервера для Linux:
go build -o openflux .

# Сборка Android APK:
./build_android_app.sh
```

## Сборка и установка Android-приложения

Укажите `ANDROID_SDK_ROOT` (или `ANDROID_HOME`), установите `gomobile` и Gradle,
затем выполните:

```bash
go install golang.org/x/mobile/cmd/gomobile@v0.0.0-20260908204917-8b95e45f8d3e
go install golang.org/x/mobile/cmd/gobind@v0.0.0-20260908204917-8b95e45f8d3e
gomobile init
./build_android_app.sh
```

Сборка создаёт отдельные APK для `arm64-v8a`, `armeabi-v7a`, `x86_64` и `x86`, а
также `OpenFlux-android-universal-debug.apk` для устройств с неизвестной
архитектурой. Передайте подходящий APK на устройство с Android 8+, установите,
укажите собственные ссылку и общий секрет во вкладке **«Настройки»**, затем
подтвердите системный запрос Android на создание VPN.

Настройки сохраняются после обычного обновления приложения, если Application ID
и сертификат подписи не менялись. Очистка данных или удаление приложения стирает
их. APK с другим сертификатом не сможет обновить установленную версию. Артефакты
CI подписаны debug-ключом, а APK в GitHub Releases — постоянным release-ключом.
При переходе с debug на release приложение потребуется один раз удалить, поэтому
сохранённые настройки будут очищены.

Дополнительные сведения находятся в [android/README.md](android/README.md).

## Флаги командной строки

| Флаг | По умолчанию | Описание |
| --- | --- | --- |
| `--client` | выкл. | Запустить SOCKS5-клиент |
| `--exit-node` | выкл. | Запустить выходную ноду (нужен root) |
| `--socks5` | `:1080` | Адрес SOCKS5-прокси |
| `--transport` | `yandex` | Транспорт (`yandex` или `oneme`) |
| `--url` | пусто | Ссылка в аргументе; безопаснее `--url-file` |
| `--url-file` | пусто | Прочитать ссылку на документ из файла |
| `--encryption-key-file` | пусто | Прочитать секрет транспорта Yandex из файла |
| `--maxToken` | пусто | Токен транспорта MAX |
| `--maxUid` | пусто | ID пользователя транспорта MAX |
| `--debug` | выкл. | Включить подробные логи |

## Разработка и безопасность

Перед коммитом выполните:

```bash
gofmt -w $(git ls-files '*.go')
go test ./...
go vet ./...
git diff --check
```

Правила участия находятся в [CONTRIBUTING.md](CONTRIBUTING.md), порядок сообщения
об уязвимостях — в [SECURITY.md](SECURITY.md), список изменений — в
[CHANGELOG.md](CHANGELOG.md).

## Лицензия

OpenFlux распространяется по GNU General Public License v3.0 или более поздней
версии. См. [LICENSE](LICENSE), [COPYRIGHT](COPYRIGHT) и [NOTICE](NOTICE). Этот
форк не одобрен Yandex и не связан с компанией.
