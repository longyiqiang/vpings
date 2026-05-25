# vpings

`vpings` is a cross-platform terminal tool for probing TCP, UDP, and QUIC latency to a target host, recording results, and presenting them in a readable CLI/TUI view.

## Current MVP

- TCP connect latency probes.
- UDP send/read probes. A UDP timeout is reported as `sent_no_reply`, because UDP does not provide a generic handshake.
- QUIC handshake latency probes.
- JSONL record storage.
- One-shot `run` command and interactive terminal menu.
- Probe creation, editing, deletion, enable/disable, status/log, and program settings views.

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

Open the full interactive menu:

```bash
go run ./cmd/vpings app
```

Menu keys:

```text
1-4             switch views
tab/left/right  switch views
r               run enabled probes now
n/e/d/space     create, edit, delete, enable/disable probes
a               toggle auto-start in program settings
h               write help guidance into logs
u               write update guidance into logs
q               quit
```

Records are written to:

```text
~/.vpings/records.jsonl
```

Program configuration is written to:

```text
~/.vpings/config.json
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
- OS-specific auto-start registration.
- Optional SQLite or embedded KV storage backend.
