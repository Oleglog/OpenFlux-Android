<div align="center">
  <img src="design/logo/avatar.svg" width="112" alt="OpenFlux logo">
  <h1>OpenFlux Android</h1>
  <p>Document-transport VPN for Android, desktop clients and Linux exit nodes.</p>
  <p>
    <a href="https://github.com/Oleglog/OpenFlux-Android/releases/latest"><img src="https://img.shields.io/github/v/release/Oleglog/OpenFlux-Android?display_name=tag&amp;sort=semver&amp;style=flat-square&amp;color=7aa2f7" alt="Latest release"></a>
    <a href="https://github.com/Oleglog/OpenFlux-Android/actions/workflows/ci.yml"><img src="https://img.shields.io/github/v/release/Oleglog/OpenFlux-Android?display_name=tag&amp;sort=semver&amp;style=flat-square&amp;color=7aa2f7" alt="CI status"></a>
    <a href="LICENSE"><img src="https://img.shields.io/github/license/Oleglog/OpenFlux-Android?style=flat-square" alt="GPL-3.0 license"></a>
    <img src="https://img.shields.io/badge/Android-8.0%2B-3DDC84?style=flat-square&amp;logo=android&amp;logoColor=white" alt="Android 8 or newer">
  </p>
  <p>
    <img src="https://img.shields.io/badge/Go-1.26.4%2B-00ADD8?style=flat-square&amp;logo=go&amp;logoColor=white" alt="Go 1.26.4 or newer">
    <img src="https://img.shields.io/badge/Java-17-ED8B00?style=flat-square&amp;logo=openjdk&amp;logoColor=white" alt="Java 17">
    <img src="https://img.shields.io/badge/ABI-ARM64%20%7C%20ARMv7%20%7C%20x86__64%20%7C%20x86-455a64?style=flat-square" alt="Supported Android architectures">
    <img src="https://img.shields.io/badge/IPv4%20%2F%20TCP-experimental-f59e0b?style=flat-square" alt="Experimental IPv4 and TCP support">
  </p>
  <p><strong>English</strong> · <a href="README.ru.md">Русский</a></p>
</div>

![OpenFlux Android: connection, logs and settings](docs/images/openflux-android-tabs.png)

> This repository is a fork of [p1neappleXpress/OpenFlux](https://github.com/p1neappleXpress/OpenFlux) featuring a native Android VPN client, optional end-to-end AES-256-GCM encryption, Mail.ru Docs transport, and support for the Yandex Volga editor engine.

OpenFlux is a research TCP tunnel with pluggable document-based transports.

**[Download latest releases (APKs & server binaries)](https://github.com/Oleglog/OpenFlux-Android/releases/latest)**

```text
Android VPN or SOCKS5 client -> Yandex Docs (Volga / Classic) or Mail.ru Docs -> Linux exit node -> Internet
```

## Features

- Android 8+ client using the system `VpnService` API (ARM64, ARMv7, x86, x86_64);
- **Transports**:
  - **Yandex Docs**: auto-detects Volga editor engine (`vyandex`) and classic editor (`yandex`);
  - **Mail.ru Docs**: auto-detected or explicitly configured (`mailru`);
- **Optional End-to-End AES-256-GCM Encryption**:
  - Directional AEAD with scrypt key derivation and replay protection;
  - Zero encryption keys required by default for seamless setup; keys can be optionally enabled on both server and client;
- **Tunable Codecs**:
  - `legacy`: LZ4-compressed framing (default on OlConnect servers, fully backwards-compatible with OlConnect Android client);
  - `batched`: batched transport with zstd compression for high-throughput links;
- **Network Isolation & Performance**:
  - Optional `--local-ip` binding for multi-homed exit nodes;
  - 16 MiB internal ring buffers for smooth packet streaming;
- Pre-built Linux server binaries attached to GitHub releases;
- DNS-over-HTTPS on Android;
- Desktop SOCKS5 client and Linux exit-node modes.

---

## Quick start

### 1. Prepare Document

#### Option A: Yandex Docs
1. Create a document on [Yandex Disk](https://disk.yandex.ru/).
2. Share access: **"Share" → "Anyone with link can edit"**.
3. Copy the document URL.

#### Option B: Mail.ru Docs
1. Create a document on [Mail.ru Cloud](https://cloud.mail.ru/).
2. Enable public editing link and copy the document URL.

### 2. Run on Linux VPS
Download prebuilt binary from releases:
```bash
wget https://github.com/Oleglog/OpenFlux-Android/releases/latest/download/openflux-linux-amd64 -O openflux
chmod +x openflux

# Drop RST packets (required on Linux exit nodes):
sudo iptables -A OUTPUT -p tcp --tcp-flags RST RST -j DROP

# Unencrypted mode (default):
sudo nohup ./openflux --exit-node --url "YOUR_DOCUMENT_URL" > openflux.log 2>&1 &

# Or with AES-256-GCM end-to-end encryption:
sudo nohup ./openflux --exit-node --url "YOUR_DOCUMENT_URL" --encryption-key "YOUR_SECRET_KEY_MIN_16_CHARS" > openflux.log 2>&1 &
```
*(View logs: `tail -f openflux.log`, stop: `sudo pkill -f openflux`)*.

### 3. Run on Android
1. Download `OpenFlux-android-arm64-v8a-debug.apk` (or `universal`) from [Releases](https://github.com/Oleglog/OpenFlux-Android/releases/latest).
2. Enter your document URL in **Settings**.
3. Tap **Start VPN** on Home tab. The client auto-detects the transport (Mail.ru vs Volga vs classic Yandex).

---

## Command-line flags

| Flag | Default | Description |
| --- | --- | --- |
| `--client` | off | Run the SOCKS5 client |
| `--exit-node` | off | Run the exit node (requires root) |
| `--socks5` | `:1080` | SOCKS5 listen address |
| `--transport` | `yandex` | Transport backend (`yandex`, `mailru`, `oneme`) |
| `--url` | empty | Inline document URL; prefer `--url-file` |
| `--url-file` | empty | Read the document URL from a file |
| `--encryption-key` | empty | Shared secret for AES-256-GCM authenticated encryption |
| `--encryption-key-file` | empty | Read the encryption secret from a file |
| `--codec` | `batched` | Packet codec: `legacy` (LZ4) or `batched` (zstd) |
| `--local-ip` | empty | Specific local IP address for exit node outbound connections |
| `--debug` | off | Enable verbose logging |

---

## Build and install the Android app

Set `ANDROID_SDK_ROOT` (or `ANDROID_HOME`) and ensure `gomobile` and Gradle are
available, then run:

```bash
go install golang.org/x/mobile/cmd/gomobile@v0.0.0-20260908204917-8b95e45f8d3e
go install golang.org/x/mobile/cmd/gobind@v0.0.0-20260908204917-8b95e45f8d3e
gomobile init
./build_android_app.sh
```

See [android/README.md](android/README.md) for Android-specific details.

---

## License

OpenFlux is licensed under the GNU General Public License v3.0 or later.
See [LICENSE](LICENSE), [COPYRIGHT](COPYRIGHT) and [NOTICE](NOTICE). This project is not affiliated with or endorsed by Yandex or Mail.ru.
