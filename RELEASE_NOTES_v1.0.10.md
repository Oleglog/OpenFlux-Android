# OpenFlux v1.0.10

### Fixes & Improvements
- **Yandex Volga / Classic Auto-Delegation**: Resolved `officeActionData.balancer_url missing` issue when using Yandex Docs transport on documents without an ONLYOFFICE balancer. The transport now inspects `action_url` / Volga markers and automatically delegates traffic to `YandexVolgaTransport` without failing or getting stuck in reconnect loops.
- **Strict Volga Degradation Removed**: Removed premature downgrade of Volga (`vyandex`) to classic `yandex` on URLs that don't match `/edit/d/`.
- **All Transports Supported**:
  - `vyandex` (Yandex Docs Volga with SmartCaptcha PoW solver)
  - `yandex` (Classic Yandex Docs with auto-delegation)
  - `mailru` (Mail.ru Cloud Docs with CSRF token retrieval & OFX1 chat channel)
  - `telemost` (Yandex Telemost VP8 WebRTC transceiver)
  - `cupsonline` (Cups.online live coding interview rooms with StripedTransport)
  - `bitrix24` (Bitrix24 chat message tunneling)
  - `avito` (Avito Web chat channel with StripedTransport)
