# OpenFlux v1.0.3 — Ускорение Yandex Volga: HTTP/2 мультиплексирование и устранение задержек

В данном релизе исправлено критическое падение скорости и задержек при работе через транспорт **Yandex Volga** (`vyandex`).

---

### 🚀 Что было исправлено

1. **HTTP/2 мультиплексирование для Volga Relay**:
   - В клиенте отправки пакетов `newRelayClient` включен обязательный флаг `ForceAttemptHTTP2: true`.
   - Ранее из-за кастомного `DialContext` в `http.Transport` Go автоматически отключал HTTP/2 и переходил на HTTP/1.1. В результате 2000 воркеров отправки данных пытались слать параллельные HTTP POST запросы по отдельным TCP-соединениям, что вызывало лавинообразные рукопожатия TLS, перегрузку очередей и сбросы пакетов со стороны шлюзов Яндекса.
   - С HTTP/2 все запросы стримятся через мультиплексированные потоки по единым постоянным соединениям, восстанавливая стабильный пинг и пропускную способность.

2. **Нормализация сигнального транспорта**:
   - В `utils.GetSignalingHTTPTransport()` восстановлен `ForceAttemptHTTP2: true` для прямых соединений.

---

### 📦 Инструкция по обновлению на VPS:
```bash
systemctl stop openflux@* || true
curl -fsSL https://github.com/Oleglog/OpenFlux-Android/releases/download/v1.0.3/openflux-linux-amd64 -o /usr/local/bin/openflux
chmod +x /usr/local/bin/openflux
systemctl restart openflux@* || true
```
