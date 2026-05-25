# vpings

`vpings` is a cross-platform terminal tool for probing TCP, UDP, and QUIC latency to a target host, recording results, and presenting them in a readable CLI/TUI view.

## Current MVP

- TCP connect latency probes.
- UDP send/read probes. A UDP timeout is reported as `sent_no_reply`, because UDP does not provide a generic handshake.
- QUIC handshake latency probes.
- JSONL record storage.
- One-shot `run` command and interactive `watch` command.

## Build

```bash
go build ./cmd/vpings
```

## Examples

Run a short probe set:

```bash
go run ./cmd/vpings run --target cloudflare.com --tcp 80,443 --udp 53 --quic 443 --count 3
```

Open the terminal watch view:

```bash
go run ./cmd/vpings watch --target cloudflare.com --tcp 443 --quic 443 --interval 2s
```

Records are written to:

```text
~/.vpings/records.jsonl
```

Override the store path:

```bash
go run ./cmd/vpings run --target 1.1.1.1 --udp 53 --store ./records.jsonl
```

## Roadmap

- Release binaries for Linux, macOS, and Windows Server.
- Install scripts for `curl | sh`, Homebrew, and PowerShell.
- Historical query and aggregation commands.
- Richer TUI charts for latency, loss, and protocol comparison.
- Optional SQLite or embedded KV storage backend.
