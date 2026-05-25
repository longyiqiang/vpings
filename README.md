# vpings

`vpings` is a cross-platform terminal tool for probing TCP, UDP, and QUIC latency to a target host, recording results, and presenting them in a readable CLI/TUI view.

## Current MVP

- TCP connect latency probes.
- ICMP echo probes.
- UDP send/read probes. A UDP timeout is reported as `sent_no_reply`, because UDP does not provide a generic handshake.
- QUIC handshake latency probes.
- Probe rounds with configurable sample count and sample interval.
- JSONL record storage.
- One-shot `run` command and interactive terminal menu.
- Probe creation, editing, deletion, enable/disable, status/log, and program settings views.
- Lightweight ASCII latency charts for realtime and historical windows.

## Build

```bash
go build ./cmd/vpings
```

## Examples

Run a short probe set:

```bash
go run ./cmd/vpings run --target dns.alidns.com --icmp --tcp 443 --udp 53 --quic 853 --count 3
```

Open the terminal watch view:

```bash
go run ./cmd/vpings watch --target dns.alidns.com --icmp --tcp 443 --quic 853 --interval 60s
```

Open the full interactive menu:

```bash
go run ./cmd/vpings app
```

The default app probes target AliDNS at `dns.alidns.com`. Probe rounds run every 60 seconds by default, and each probe sends 10 packets/samples with a 1 second sample gap.

Menu keys:

```text
1-4             switch views
tab/left/right  switch views
r               run enabled probes now
up/down         select a probe in the result view
enter/esc       open or close a probe detail view
n/e/d/space     create, edit, delete, enable/disable probes
g               edit probe defaults in the probe menu
a               toggle auto-start in program settings
h               write help guidance into logs
u               write update guidance into logs
q               quit
```

Result charts:

```text
Overview        realtime chart for the selected probe
Detail          realtime, past 24 hours, past 2 days, past week
X axis          sample round time
Y axis          latency in milliseconds
Main line       median latency for each sample round
Range lines     min and max latency from that round
Color           loss rate, green to red
```

Records are written to:

```text
~/.vpings/records.jsonl
```

Program configuration is written to:

```text
~/.vpings/config.json
```

Probe defaults include the round interval, default timeout, default sample count, and default sample gap. Editing defaults affects new probes; the defaults editor can also apply the timeout/sample settings to all existing probes.

Override the store path:

```bash
go run ./cmd/vpings run --target 1.1.1.1 --udp 53 --store ./records.jsonl
```

## Roadmap

- Release binaries for Linux, macOS, and Windows Server.
- Install scripts for `curl | sh`, Homebrew, and PowerShell.
- Historical query and aggregation commands.
- Loss and protocol comparison charts.
- OS-specific auto-start registration.
- Optional SQLite or embedded KV storage backend.
