# Changelog

All notable changes to this fork are documented here.

## [1.0.21] - 2026-11-05

### Fixed

- **Telemost asked the SFU for a thumbnail slot and paid for it.** The first
  two sharing slots now request 1920x1080 instead of 608x342. The SFU sizes
  the bitrate it forwards to the slot it thinks we render, so the old default
  capped each lane at a few hundred Kbps regardless of what we sent; a
  measured sweep in one room read 0.45 MB/s at 608x342 and 1.15 MB/s at
  1920x1080 (2.44x), with no downside for a tunnel that renders nothing.
- **A dead Telemost session left the node running and silent.** When the SFU
  session dies every lane goes down and nothing rebuilds it, so the process
  kept running while carrying no traffic. The stripe now has a health
  deadline (`SetHealthDeadline`, `ErrAllLanesDead`); Telemost sets 60 s, after
  which the stripe stops and the supervisor restarts the node.
- **Control frames no longer wait behind a full bulk queue.** `TrySend` puts a
  control frame into a small priority queue that goes out ahead of data and
  never blocks. A carrier that takes nothing used to strand a new client's
  handshake behind hours of queued data on a long-lived exit.
- **Avito 403s.** The transport now sends a current Firefox User-Agent; Avito
  rejects the old Chrome/137 Mac one on both the API and ShareDB.

## Unreleased

### Added

- synced with upstream p1neappleXpress/OpenFlux through commit `3249724`:
  fixed swapped `maxToken`/`maxUid` flag descriptions, `yandex` transport now
  returns errors instead of panicking on unexpected document config (with new
  tests), and an experimental Yandex.Docs Volga transport (`vyandex.go`) is
  now in the tree.

### Notes

- the new `vyandex` transport is intentionally not wired into the
  `--transport` CLI switch: it has no mandatory encryption wrapper yet, so
  exposing it would contradict this fork's encrypted-by-default security
  model for Yandex Docs transports.

## 0.3.0 - 2026-09-10

### Added

- Android APKs for `arm64-v8a`, `armeabi-v7a`, `x86_64`, `x86` and universal
  devices;
- automated GitHub Release publishing for tags matching `v*`;
- SHA-256 checksums for every release APK.

## 0.2.0 - 2026-09-10

### Added

- native Android `VpnService` client with connection, logs and settings tabs;
- Android Keystore-backed protection for the saved document URL and secret;
- mandatory AES-256-GCM encryption for the Yandex document transport;
- encrypted ping frames and an animated latency graph;
- DNS-over-HTTPS support for the Android VPN;
- a hardened sample systemd service for the Linux exit node;
- CI checks for Go and Android debug builds.

### Changed

- secret values can be loaded from root-only files instead of command-line
  arguments;
- Android application version is now 0.2.0 (version code 2).

### Removed

- the incomplete iOS prototype and generated IDE/build artifacts.
