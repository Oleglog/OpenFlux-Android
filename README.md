<div align="center">
  <img src="design/logo/avatar.svg" width="112" alt="OpenFlux logo">
  <h1>OpenFlux Android</h1>
  <p>Transport core and exit node for a tunnel that rides on "white" services.</p>
  <p>
    <a href="https://github.com/Oleglog/OpenFlux-Android/releases/latest"><img src="https://img.shields.io/github/v/release/Oleglog/OpenFlux-Android?display_name=tag&amp;sort=semver&amp;style=flat-square&amp;color=7aa2f7" alt="Latest release"></a>
    <a href="https://github.com/Oleglog/OpenFlux-Android/actions/workflows/ci.yml"><img src="https://github.com/Oleglog/OpenFlux-Android/actions/workflows/ci.yml/badge.svg" alt="CI status"></a>
    <a href="LICENSE"><img src="https://img.shields.io/github/license/Oleglog/OpenFlux-Android?style=flat-square" alt="GPL-3.0"></a>
  </p>
  <p><strong>English</strong> · <a href="README.ru.md">Русский</a></p>
</div>

> A fork of [p1neappleXpress/OpenFlux](https://github.com/p1neappleXpress/OpenFlux). Upstream tunnelled through collaborative document editors (Yandex Docs, Mail.ru Docs); this fork grew into a configurable core: several carriers, a multi-channel stripe with ARQ, end-to-end encryption and isolated exit-node modes.

This repository is the **Go core and the `openflux` binary**. The Android app people actually use lives in [OlConnect_client](https://github.com/Oleglog/OlConnect_client); the control panel and subscription server live in [OlConnect_manager](https://github.com/Oleglog/OlConnect_manager).

```text
Phone or SOCKS5 client -> carrier (Boards, Cups, ...) -> exit node -> internet
```

## What it does

- **Carriers (transports)** — the channel our traffic rides inside. Status and numbers below.
- **Multi-channel stripe.** `striped.go` cuts the stream into pieces, spreads them across channels, restores order and re-asks for what was lost (ARQ with a reorder window and retransmits). One channel cannot beat its own latency, so throughput comes from several.
- **Packet codec.** `batched` (default) coalesces packets and compresses with zstd; batch-size and linger hints travel through every wrapper. `legacy` is per-packet LZ4.
- **AES-256-GCM end-to-end encryption** with scrypt key derivation and replay protection. For Yandex Boards the key is **mandatory**: the board drops frames that carry visible IP packets.
- **Two exit-node modes**: `l4` (default, gVisor, user-space TCP, egress through SOCKS5) and `l3` (kernel raw sockets, L3 IP packets, the same framing the phone uses). `l3` is the phone path; `l4` is safe on a shared test box.
- **A client for testing without a phone**: `--client` exposes SOCKS5, `--tun` pushes real IP traffic through an existing TUN device, which exercises the exact phone path.

## Carriers and their state

| Carrier | `--transport` | State | Measured on the stand |
| --- | --- | --- | --- |
| Yandex Boards | `boards` | **Primary, fastest** | 1 MB in 1.56 s (641 KB/s), 10 MB — 694 KB/s, 60 s — 952 KB/s, `refused=0` |
| Cups | `cups` / `cupsonline` | **Working** | 10 MB in 136 s — 71.8 KB/s over 8 rooms (11 KB/s and a hard stall at 40–45 s before the framing fix) |
| Yandex Docs (Volga) | `volga` / `vyandex` | Works, slow — reserve | 5–7 KB/s |
| Mail.ru Docs | `mailru` | Works, slow — reserve | 6–8 KB/s |
| Telemost | `telemost` | **Closed**: SFU-side ceiling, not ours | — |
| Avito | `avito` | Broken server-side | — |
| Bitrix24 | `bitrix24` / `b24` | Experimental | — |
| MAX / OneMe | `oneme` | Not implemented: needs `--maxToken` and `--maxUid` | — |

Numbers come from a live VPS over the same path the phone uses. Boards wins because one object carries up to 48 KiB — 45000 bytes here, with headroom under the board's 65536-character cap.

## Exit node in a hurry

```bash
gh release download v2.0.0 --repo Oleglog/OpenFlux-Android --pattern 'openflux-linux-amd64'
install -m 755 openflux-linux-amd64 /usr/local/bin/openflux

# Linux needs the RST drop, or the kernel answers packets that are not ours:
iptables -A OUTPUT -p tcp --tcp-flags RST RST -j DROP

openflux --exit-node --mode l4 --transport boards \
  --url "https://docs.yandex.ru/..." \
  --encryption-key "$(cat /etc/openflux/key.hex)" \
  --socks5 127.0.0.1:1080
```

`l3` needs root and a free TUN device; `l4` avoids raw sockets entirely, which is why it is the safe choice for tests.

## Client for tests and desktops

```bash
# desktop SOCKS5 client
openflux --client --transport boards --url "..." --encryption-key "..." --socks5 127.0.0.1:1080

# the phone path: real IP through a prepared TUN device
ip tuntap add dev ofxb mode tun && ip link set ofxb up
ip addr add 10.10.10.2/32 dev ofxb
ip route replace default dev ofxb table 100
ip rule add priority 100 from 10.10.10.2 lookup 100
openflux --client --transport boards --url "..." --encryption-key "..." --tun ofxb
curl --interface 10.10.10.2 https://example.com
```

## Flags

| Flag | Default | Purpose |
| --- | --- | --- |
| `--exit-node` | off | Run as exit node (root needed for `l3`) |
| `--client` | off | Run as client: SOCKS5 or `--tun` |
| `--mode` | `l4` | `l4` — gVisor and SOCKS5; `l3` — kernel raw sockets |
| `--transport` | `yandex` | `boards`, `cups`, `volga`, `mailru`, `telemost`, `avito`, `bitrix24`, `oneme`, `yandex` |
| `--url` / `--url-file` | empty | Document, board or room link |
| `--encryption-key` / `--encryption-key-file` | empty | AES-256-GCM key |
| `--codec` | `batched` | `batched` (zstd plus coalescing) or `legacy` (per-packet LZ4) |
| `--socks5` | `:1080` | SOCKS5 listen address |
| `--tun` | empty | Client over an existing TUN (phone framing, needs an `l3` exit) |
| `--local-ip` | empty | Exit-node egress IP |
| `--warp-proxy` | empty | WARP/SOCKS5 for exit traffic in `l4` |
| `--socks-proxy` | empty | Proxy for transport signalling |
| `--cookies` / `--cookies-file` | empty | Netscape cookies.txt for Volga and Mail.ru |
| `--maxToken` / `--maxUid` | empty | MAX; the transport is not implemented yet |
| `--debug` | off | Verbose logs |

`STRIPE_*` and `OPENFLUX_CUPS_*` environment variables tune the stripe and the carrier pacer; defaults were chosen by measurement, so do not change them without a bench.

## Build and releases

Local builds and tests are **not** run in this project — GitHub Actions only: `ci.yml` (tests, vet, lint) and `release.yml` (Linux amd64/arm64, Windows, debug APKs). A `v1.0.x` tag is a release.

Releases are consolidated: the whole history from `v1.0.0` to the current version is described in one global release; tags and git history are preserved.

## What no longer works, and why

- **Telemost** — closed. The SFU drops 10–30 % of what it relays in bursts; no setting on our side changes that.
- **Avito** — broken server-side and reproducible without us.
- **Volga and Mail.ru** — not broken, but 5–8 KB/s is not enough. Kept as a reserve, no active work.
- **MAX / OneMe** — transport not implemented; needs call tokens and identifiers.
- **The Android app in this repository** is fork heritage: the working client is [OlConnect_client](https://github.com/Oleglog/OlConnect_client); only debug builds remain here for compatibility.

## Experimental

The "Rail" design experiment was rolled back out of the main code and lives in the `experimental/design-rail-2026-11` branch of the client and panel repositories, with mockups kept outside the repositories.

## Related repositories

- [OlConnect_client](https://github.com/Oleglog/OlConnect_client) — Android app (VPNService, profiles, subscriptions, appearance).
- [OlConnect_manager](https://github.com/Oleglog/OlConnect_manager) — panel, instance launcher, subscription server and mirror.

## License

GNU General Public License v3.0 or later. See [LICENSE](LICENSE), [COPYRIGHT](COPYRIGHT), [NOTICE](NOTICE). This fork is not endorsed by or affiliated with Yandex or VK (Mail.ru).
