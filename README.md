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

> This repository is a fork of [p1neappleXpress/OpenFlux](https://github.com/p1neappleXpress/OpenFlux) featuring a native Android VPN client, no encryption keys required, and support for the new Yandex Volga editor engine.

OpenFlux is a research TCP tunnel with pluggable transports.

**[Download latest releases (APKs & server binaries)](https://github.com/Oleglog/OpenFlux-Android/releases/latest)**

```text
Android VPN or SOCKS5 client -> Yandex Docs (Volga / Classic) -> Linux exit node -> Internet
```

## Features

- Android 8+ client using the system `VpnService` API (ARM64, ARMv7, x86, x86_64);
- **Auto-detection of Yandex editor**: supports both the new Volga engine (`vyandex`) and classic editor (`yandex`);
- **No encryption keys**: compatible with upstream OpenFlux protocol;
- Pre-built Linux server binaries attached to GitHub releases;
- DNS-over-HTTPS on Android;
- desktop SOCKS5 client and Linux exit-node modes.

> **MAX transport warning:** the MAX backend sends packets via WebRTC
> DataChannel on your MAX account. Do not use a primary or important account;
> running it from an external VPS may lead to account restrictions that
> persist after OpenFlux stops. Treat MAX transport as experimental until its
> detection and blocking behavior is better understood.

## Important limitations

OpenFlux is experimental research software, not an audited replacement for
WireGuard or another mature VPN. The Android tunnel currently supports IPv4 and
TCP. DNS is handled separately over HTTPS; arbitrary UDP and IPv6 are not
tunneled. The document provider can still observe metadata such as connection
times, traffic sizes and encrypted payloads. Anyone with document edit access
can disrupt the connection.

Use the software only on systems and networks you own or are authorized to
test.

## Requirements

- Go 1.26.4 or newer for the desktop client and exit node;
- a Linux VPS/VDS with root access for the exit node;
- for Android builds: Java 17, Android SDK/API 35, Build Tools 35.0.0,
  NDK 27.0.12077973, Gradle 8.14.3 and `gomobile`;
- an editable document opened with the legacy Yandex Docs editor when using
  the Yandex transport.

## Prepare the private configuration

## Quick start

### 1. Prepare Yandex document
1. Create a document on [Yandex Disk](https://disk.yandex.ru/).
2. Share access: **"Share" → "Anyone with link can edit"**.
3. Copy the document URL.

### 2. Run on Linux VPS
Download prebuilt binary from releases:
```bash
wget https://github.com/Oleglog/OpenFlux-Android/releases/latest/download/openflux-linux-amd64 -O openflux
chmod +x openflux

# Drop RST packets:
sudo iptables -A OUTPUT -p tcp --tcp-flags RST RST -j DROP

# Run in background via nohup:
sudo nohup ./openflux --exit-node --url "YOUR_DOCUMENT_URL" > openflux.log 2>&1 &
```
*(View logs: `tail -f openflux.log`, stop: `sudo pkill -f openflux`)*.

### 3. Run on Android
1. Download `OpenFlux-android-arm64-v8a-debug.apk` (or `universal`) from [Releases](https://github.com/Oleglog/OpenFlux-Android/releases/latest).
2. Enter your document URL in **Settings**.
3. Tap **Start VPN** on Home tab. The client auto-detects Volga vs classic engine.

## Build and install the Android app

Set `ANDROID_SDK_ROOT` (or `ANDROID_HOME`) and ensure `gomobile` and Gradle are
available, then run:

```bash
go install golang.org/x/mobile/cmd/gomobile@v0.0.0-20260908204917-8b95e45f8d3e
go install golang.org/x/mobile/cmd/gobind@v0.0.0-20260908204917-8b95e45f8d3e
gomobile init
./build_android_app.sh
```

The build creates separate APKs for `arm64-v8a`, `armeabi-v7a`, `x86_64` and
`x86`, plus `OpenFlux-android-universal-debug.apk` for devices whose architecture
is unknown. Transfer the appropriate APK to an Android 8+ device, install it,
enter your own document URL and shared secret in **Settings**, then approve
Android's VPN prompt.

Configuration survives a normal in-place app update when the application ID
and signing certificate stay the same. Clearing app data or uninstalling the
app removes it. APKs signed with a different certificate cannot update the
existing installation. CI artifacts are debug builds; APKs attached to GitHub
Releases use the project's persistent release certificate. Moving from a debug
build to the release channel requires one uninstall and therefore clears saved
settings.

See [android/README.md](android/README.md) for Android-specific details.

## Command-line flags

| Flag | Default | Description |
| --- | --- | --- |
| `--client` | off | Run the SOCKS5 client |
| `--exit-node` | off | Run the exit node (requires root) |
| `--socks5` | `:1080` | SOCKS5 listen address |
| `--transport` | `yandex` | Transport backend (`yandex` or `oneme`) |
| `--url` | empty | Inline document URL; prefer `--url-file` |
| `--url-file` | empty | Read the document URL from a file |
| `--encryption-key-file` | empty | Read the Yandex transport secret from a file |
| `--maxToken` | empty | MAX transport token |
| `--maxUid` | empty | MAX transport user ID |
| `--debug` | off | Enable verbose logging |

## Development and security

Run checks before committing:

```bash
gofmt -w $(git ls-files '*.go')
go test ./...
go vet ./...
git diff --check
```

Contributions are described in [CONTRIBUTING.md](CONTRIBUTING.md). Please read
[SECURITY.md](SECURITY.md) before reporting a vulnerability. Changes are listed
in [CHANGELOG.md](CHANGELOG.md).

## License

OpenFlux is licensed under the GNU General Public License v3.0 or later. See
[LICENSE](LICENSE), [COPYRIGHT](COPYRIGHT) and [NOTICE](NOTICE). This fork is not
endorsed by or affiliated with Yandex.
