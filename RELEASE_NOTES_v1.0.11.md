# OpenFlux v1.0.11

### Improvements & Fixes
- **Resilient DNS-over-HTTPS (DoH) with Yandex DoH**: Prioritize `https://common.dot.yandex.net/dns-query` (Yandex DoH) and multi-endpoint fallback (Google DoH, Cloudflare) to ensure DNS resolution works reliably under Russian mobile carrier blocks (where Cloudflare `1.1.1.1` is blocked by TSPU).
- **Mail.ru & Volga URL Normalization**: Cleaned query strings and parameters in web links for robust document session establishment.
