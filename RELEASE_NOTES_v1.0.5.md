# OpenFlux Android v1.0.5 / Server Update

## Ключевые изменения и исправления

### 1. Полная маршрутизация трафика сигнализации через SOCKS-прокси (устранение 401 и блокировок)
- **Проблема**: В предыдущих версиях параметры `--socks-proxy` (например, `socks5://127.0.0.1:40000`) не применялись к REST-клиенту `fetchDocInfo` в Mail.ru и обходились в ряде вызовов `dialIPv4First` в Яндекс.В результате запросы к API отправлялись напрямую с IP-адреса зарубежного VPS датацентра, что приводило к ошибке `API returned status 401 (FORBIDDEN/ANONYMOUSE/ACCESS)` в Mail.ru и немедленному вызову SmartCaptcha в Яндекс.
- **Исправление**: 
  - `fetchDocInfo` и WebSocket-соединения Mail.ru теперь строго используют `utils.GetSignalingHTTPTransport()` и `utils.ConfigureWebsocketDialerProxy`.
  - В `transport/yandex/dial.go` функция `dialIPv4First` теперь маршрутизирует соединения через настроенный сигнальный прокси.
  - Все остальные транспорты (Yandex Volga, Yandex Docs, Boards, Avito, Cups.online, Telemost) теперь унифицированно направляют HTTP и WebSocket трафик сигнализации через SOCKS5-прокси.

### 2. Поддержка SmartCaptcha в ответах HTTP 200 OK (устранение ошибки "client-config not found")
- **Проблема**: Защита Яндекс обновила выдачу SmartCaptcha — вместо 302 редиректа на `showcaptchafast` капча теперь отдается с HTTP 200 OK прямо на URL документа со встроенным скриптом `window.__SSR_DATA__ = {...}` и формой `checkbox-captcha-form`. Транспорт Volga ожидал в 200 OK страницу редактора и падал с фатальной ошибкой `client-config not found`.
- **Исправление**:
  - `vyandex.go` (`authorizeWithJar`) теперь проверяет HTTP 200 OK на наличие капчи (`x-yandex-captcha: captcha`, `window.__SSR_DATA__`, `checkbox-captcha-form`) и передает страницу в солвер.
  - В `captcha.go` парсер адаптирован под современный JSON без base64-кодирования, динамическое определение хоста отправки (`disk.yandex.ru`) и гибкие типы сложности PoW.
  - После решения PoW-задачи и подтверждения токена сессия корректно авторизуется и загружает реальный `client-config`.

### 3. Поддержка импорта cookies (Netscape cookies.txt)
- Добавлен флаг `--cookies` / `--cookies-file` (и переменные окружения `OLCRTC_COOKIES`, `OPENFLUX_COOKIES`).
- Реализован парсер cookies для Mail.ru (`loadMailruCookies`) и метод `LoadCookieFile` для `MailruDocsTransport`.
- Позволяет открывать документы, требующие авторизованной сессии пользователя, без сбоев.

### 4. Обновление мобильного клиента Android
- Версия обновлена до `1.0.5` (versionCode 13).
- Включены все исправления транспортов и обхода блокировок.
