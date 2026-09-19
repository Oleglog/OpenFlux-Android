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

> Это форк проекта [p1neappleXpress/OpenFlux](https://github.com/p1neappleXpress/OpenFlux) с нативным клиентом Android VPN, поддержкой документов Mail.ru Docs, нового движка Яндекс.Документов («Волга») и опциональным сквозным шифрованием AES-256-GCM.

OpenFlux — исследовательский TCP-туннель с подключаемыми транспортами через совместное редактирование документов.

**[Скачать релизы (APK и бинарники сервера)](https://github.com/Oleglog/OpenFlux-Android/releases/latest)**

```text
Android VPN или SOCKS5-клиент -> Яндекс.Документы (Volga / Classic) или Mail.ru Docs -> Linux-нода -> интернет
```

## Возможности

- Android-клиент для Android 8+ на системном `VpnService` (ARM64, ARMv7, x86, x86_64);
- **Поддерживаемые транспорты**:
  - **Яндекс.Документы**: автоопределение редактора «Волга» (транспорт `vyandex`) и классического редактора (`yandex`);
  - **Mail.ru Docs**: редактор документов Mail.ru (транспорт `mailru`);
- **Опциональное сквозное шифрование AES-256-GCM**:
  - Защита данных от чтения провайдером облачного сервиса документов;
  - Деривация ключей через scrypt и защита от атак повторного воспроизведения (replay attack);
  - По умолчанию ключи не требуются (режим без шифрования для максимальной простоты), при желании ключ задается на сервере и клиенте;
- **Настраиваемые кодеки**:
  - `legacy`: сжатие LZ4 (по умолчанию на сервере OlConnect Manager для 100% обратной совместимости);
  - `batched`: батчинг пакетов с компрессией zstd для высокоскоростных каналов;
- **Изоляция сети и оптимизация**:
  - Флаг `--local-ip` для явной привязки сетевого интерфейса на VPS;
  - Буферы 16 MiB для плавной передачи трафика;
- Готовые собранные бинарники для Linux VPS прямо в релизах;
- DNS-over-HTTPS на Android;
- SOCKS5-клиент для компьютера и режим выходной Linux-ноды.

---

## Быстрый запуск

### 1. Подготовка документа

#### Вариант A: Яндекс.Документы
1. Создайте текстовый документ на [Яндекс Диске](https://disk.yandex.ru/).
2. Откройте к нему доступ: **«Поделиться» → «Редактирование по ссылке»**.
3. Скопируйте ссылку на документ.

#### Вариант B: Mail.ru Docs
1. Создайте текстовый документ в [Облаке Mail.ru](https://cloud.mail.ru/).
2. Включите доступ по ссылке с правом редактирования и скопируйте ссылку.

### 2. Запуск на сервере (VPS)
Скачайте готовый бинарник сервера из релизов:
```bash
wget https://github.com/Oleglog/OpenFlux-Android/releases/latest/download/openflux-linux-amd64 -O openflux
chmod +x openflux

# Заглушить RST-пакеты (обязательно на Linux exit-node):
sudo iptables -A OUTPUT -p tcp --tcp-flags RST RST -j DROP

# Запуск без шифрования (по умолчанию):
sudo nohup ./openflux --exit-node --url "ВАША_ССЫЛКА_НА_ДОКУМЕНТ" > openflux.log 2>&1 &

# Или со сквозным шифрованием AES-256-GCM:
sudo nohup ./openflux --exit-node --url "ВАША_ССЫЛКА_НА_ДОКУМЕНТ" --encryption-key "СЕКРЕТНЫЙ_КЛЮЧ_ОТ_16_СИМВОЛОВ" > openflux.log 2>&1 &
```
*(Проверить логи сервера: `tail -f openflux.log`, остановить: `sudo pkill -f openflux`)*.

### 3. Запуск на Android
1. Скачайте `OpenFlux-android-arm64-v8a-debug.apk` (или `universal`) из [релизов](https://github.com/Oleglog/OpenFlux-Android/releases/latest).
2. Во вкладке **«Настройки»** вставьте ссылку на ваш документ.
3. На главном экране нажмите **«Запустить VPN»**. Клиент автоматически определит тип редактора (Mail.ru, Volga или классический) и подключится.

---

## Флаги командной строки

| Флаг | По умолчанию | Описание |
| --- | --- | --- |
| `--client` | выкл. | Запустить SOCKS5-клиент |
| `--exit-node` | выкл. | Запустить выходную ноду (нужен root) |
| `--socks5` | `:1080` | Адрес SOCKS5-прокси |
| `--transport` | `yandex` | Транспорт (`yandex`, `mailru`, `oneme`) |
| `--url` | пусто | Ссылка в аргументе; безопаснее `--url-file` |
| `--url-file` | пусто | Прочитать ссылку на документ из файла |
| `--encryption-key` | пусто | Секретный ключ сквозного шифрования AES-256-GCM |
| `--encryption-key-file` | пусто | Прочитать секретный ключ шифрования из файла |
| `--codec` | `batched` | Кодек пакетов: `legacy` (LZ4) или `batched` (zstd) |
| `--local-ip` | пусто | Локальный IP-адрес для исходящих соединений ноды |
| `--debug` | выкл. | Включить подробные логи |

---

## Сборка из исходников (для разработчиков)

```bash
# Сборка сервера для Linux:
go build -o openflux .

# Сборка Android APK:
./build_android_app.sh
```

Дополнительные сведения находятся в [android/README.md](android/README.md).

---

## Лицензия

OpenFlux распространяется по лицензии GNU General Public License v3.0 или более поздней версии. См. [LICENSE](LICENSE), [COPYRIGHT](COPYRIGHT) и [NOTICE](NOTICE). Этот форк не одобрен компаниями Яндекс или VK (Mail.ru) и не связан с ними.
