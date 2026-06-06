```text
 __   __  ____   ___  _   _   ____  ____
 \ \ / / |  _ \ |_ _|| \ | | / ___|/ ___|
  \ V /  | |_) | | | |  \| || |  _ \___ \
   | |   |  __/  | | | |\  || |_| | ___) |
   |_|   |_|    |___||_| \_| \____||____/
```

# vpings

`vpings` 是一个用 Go 编写的跨平台终端网络探测工具，用于探测目标主机的 ICMP、TCP、UDP、QUIC 延迟，保存探测结果，并在 CLI/TUI 中查看实时与历史趋势。

当前项目已经包含三个入口：

- `app` / `menu`：完整交互式终端菜单。
- `run`：执行有限轮探测并打印结果表格。
- `watch`：打开持续刷新的终端监控视图。

## 项目状态

`vpings` 当前处于 MVP 到可维护工具的过渡阶段。核心探测、记录、TUI、图表和测试已经具备，安装器、系统自启动注册、发布自动化和更完整的历史查询能力仍在规划中。

适合当前使用场景：

- 临时排查目标主机或端口的连通性和延迟。
- 在终端里持续观察多协议探测状态。
- 记录 JSONL 数据，后续自行聚合或分析。
- 作为网络探测 TUI 工具的二次开发基础。

暂不承诺的能力：

- 不保证等价替代专业监控系统。
- 不提供 SLA、告警路由或分布式采集。
- `auto_start` 目前只是配置字段，尚未接入系统服务注册。

## 功能概览

- ICMP Echo 探测。
- TCP connect 延迟探测。
- UDP 发送/等待响应探测。UDP 没有通用握手，发送成功但超时未收到响应时记录为 `sent_no_reply`。
- QUIC 握手延迟探测。
- 支持按轮次执行探测，每个探针可配置样本数、样本间隔、超时时间。
- 探测记录持久化为 JSONL。
- 支持交互式探针管理：新增、编辑、删除、启用/禁用。
- 支持程序配置与默认探测参数配置。
- 使用 ASCII 图表展示实时窗口、过去 24 小时、过去 48 小时、过去 7 天趋势。

## 环境要求

- Go `1.25.7`，以 [go.mod](go.mod) 为准。
- 依赖主要包括：
  - `bubbletea`：TUI 应用状态与事件循环。
  - `lipgloss`：终端样式。
  - `asciigraph`：图表颜色输出。
  - `quic-go`：QUIC 握手探测。

ICMP 在部分系统或网络环境下可能需要额外权限。当前实现会先尝试 `udp4`，再尝试 `ip4:icmp`。

## 安装方式

当前仓库尚未提供正式 Release 安装脚本。推荐在开发阶段通过源码构建：

```bash
git clone https://github.com/longyiqiang/vpings.git
cd vpings
make build
```

构建后的二进制位于：

```text
bin/vpings
```

## 快速开始

安装依赖并运行测试：

```bash
go mod download
go test ./...
```

构建本机二进制：

```bash
make build
```

或直接使用 Go 构建：

```bash
go build -o bin/vpings ./cmd/vpings
```

执行一次有限轮探测：

```bash
go run ./cmd/vpings run --target dns.alidns.com --icmp --tcp 443 --udp 53 --quic 853 --count 3
```

打开持续刷新视图：

```bash
go run ./cmd/vpings watch --target dns.alidns.com --icmp --tcp 443 --quic 853 --interval 60s
```

打开完整交互式菜单：

```bash
go run ./cmd/vpings app
```

## 命令说明

### `app` / `menu`

打开完整 TUI 菜单，默认读取当前目录下的 `vpings.json`，并把结果追加到 `records.jsonl`。

```bash
go run ./cmd/vpings app --config ./vpings.json --store ./records.jsonl
```

如果配置文件不存在，程序会自动创建默认配置。默认探针目标为 `dns.alidns.com`：

- ICMP
- TCP `443`
- UDP `53`
- QUIC `853`

默认探测节奏：

- 轮次间隔：`10s`
- 单次探测超时：`1s`
- 每个探针每轮样本数：`10`
- 样本间隔：`0s`

### `run`

执行固定轮次探测，并在结束后输出结果表格。

```bash
go run ./cmd/vpings run \
  --target 1.1.1.1 \
  --icmp \
  --tcp 443,853 \
  --udp 53 \
  --quic 853 \
  --count 4 \
  --interval 1s \
  --timeout 2s \
  --store ./records.jsonl
```

常用参数：

| 参数 | 说明 |
| --- | --- |
| `--target`, `-t` | 目标主机或 IP，必填。 |
| `--icmp` | 启用 ICMP 探测。 |
| `--tcp` | 逗号分隔的 TCP 端口列表。 |
| `--udp` | 逗号分隔的 UDP 端口列表。 |
| `--quic` | 逗号分隔的 QUIC 端口列表。 |
| `--count`, `-c` | 探测轮数，默认 `4`。 |
| `--interval` | 轮次间隔，默认 `1s`。 |
| `--timeout` | 单个探测超时，默认 `2s`。 |
| `--store` | JSONL 记录路径，默认 `records.jsonl`。 |

### `watch`

打开轻量实时刷新视图，参数与 `run` 基本一致，但每次 tick 只跑一轮，`--count` 会被忽略。

```bash
go run ./cmd/vpings watch --target dns.alidns.com --icmp --tcp 443 --interval 10s
```

退出按键：`q`、`esc` 或 `ctrl+c`。

## 交互式菜单按键

| 按键 | 作用 |
| --- | --- |
| `1`-`4` | 切换 Results、Status & logs、Probes、Program 页面。 |
| `tab` / `left` / `right` | 切换页面。 |
| `r` | 立即执行一轮已启用探针。 |
| `up` / `down` / `j` / `k` | 移动选择。 |
| `enter` | 在结果页进入详情；在表单页保存。 |
| `esc` | 退出详情或取消表单。 |
| `+` / `-` / `0` | 在详情图表中放大、缩小、重置时间窗口。 |
| `n` | 新建探针。 |
| `e` | 编辑探针。 |
| `d` / `backspace` | 删除探针。 |
| `space` | 启用或禁用探针。 |
| `g` | 编辑探针默认参数。 |
| `a` | 切换 `auto_start` 配置值。 |
| `h` | 写入帮助日志。 |
| `u` | 写入更新提示日志。 |
| `q` / `ctrl+c` | 退出程序。 |

## 数据文件

### 配置文件：`vpings.json`

由 [internal/appconfig/config.go](internal/appconfig/config.go) 管理。默认路径是当前目录的 `vpings.json`。

主要字段：

| 字段 | 说明 |
| --- | --- |
| `probe_interval` | 交互式菜单中自动探测的轮次间隔。 |
| `default_timeout` | 新探针默认超时时间。 |
| `default_sample_count` | 新探针默认样本数。 |
| `default_sample_interval` | 新探针默认样本间隔。 |
| `auto_start` | 是否自动启动的配置标志，目前仅保存配置，OS 服务注册尚未接入。 |
| `probes` | 探针列表。 |

每个探针包含：

| 字段 | 说明 |
| --- | --- |
| `id` | 探针 ID，用于历史记录归组。 |
| `name` | 显示名称。 |
| `protocol` | `icmp`、`tcp`、`udp` 或 `quic`。 |
| `host` | 目标主机。 |
| `port` | 目标端口；ICMP 使用 `0`。 |
| `timeout` | 单次探测超时。 |
| `sample_count` | 每轮样本数。 |
| `sample_interval` | 同一探针内样本间隔。 |
| `enabled` | 是否启用。 |

### 记录文件：`records.jsonl`

由 [internal/store/jsonl.go](internal/store/jsonl.go) 管理。每行是一条探测结果 JSON。

当前写入字段使用：

- `duration_ms`：毫秒浮点数。
- `attempt_count`：本轮总尝试数。

读取时仍兼容旧字段：

- `duration`
- `attempts`

这意味着修改记录格式时需要同时考虑新写入格式与旧数据读取兼容。

## 权限与网络说明

- ICMP 探测可能受系统权限、内核策略、防火墙或容器网络限制影响。
- TCP 探测测量的是连接建立耗时，不代表应用层请求耗时。
- UDP 探测只能确认本地报文发送与是否收到响应，超时无响应会记录为 `sent_no_reply`。
- QUIC 探测会发起握手，并使用 `InsecureSkipVerify` 跳过证书校验；该工具测量连通性和握手耗时，不用于校验证书可信度。
- 所有探测结果默认写入本地 `records.jsonl`，不会自动上传。

## 项目结构树与注解

```text
vpings/
├── cmd/
│   └── vpings/
│       └── main.go              # CLI 入口；解析 app/run/watch 命令与命令行参数。
├── internal/
│   ├── appconfig/
│   │   ├── config.go            # 程序配置、默认探针、配置迁移、探针 ID 生成。
│   │   └── config_test.go       # 默认配置、迁移、ID 归一化测试。
│   ├── probe/
│   │   ├── probe.go             # 协议枚举、探测规格、结果模型、协议分发入口。
│   │   ├── icmp.go              # ICMP Echo 探测实现。
│   │   ├── tcp.go               # TCP connect 探测实现。
│   │   ├── udp.go               # UDP 发送与响应等待逻辑；超时可标记 sent_no_reply。
│   │   └── quic.go              # QUIC 握手探测实现。
│   ├── store/
│   │   ├── jsonl.go             # JSONL 追加写入、最近记录读取、旧格式兼容。
│   │   └── jsonl_test.go        # JSONL 字段名与旧记录兼容测试。
│   └── ui/
│       ├── app.go               # 完整 TUI 菜单、页面状态、表单、配置保存、批量采样。
│       ├── app_test.go          # App 级交互与采样行为测试。
│       ├── chart.go             # 延迟汇总、丢包率、ASCII 图表、时间窗口与缩放。
│       ├── chart_test.go        # 图表汇总与渲染相关测试。
│       ├── table.go             # run/watch 结果表格渲染与状态样式。
│       └── watch.go             # watch 模式 TUI、定时轮询、记录追加。
├── Makefile                     # build/test/clean/cross 常用任务。
├── go.mod                       # Go 模块定义与直接依赖。
├── go.sum                       # 依赖校验。
└── README.md                    # 项目说明与维护注解。
```

生成文件与运行产物：

```text
bin/                            # make build 输出目录，当前未纳入源码结构。
dist/                           # make cross 输出目录，包含多平台二进制。
records.jsonl                   # 默认探测记录文件，运行时生成。
vpings.json                     # 默认配置文件，app 首次运行时生成。
outputs/                        # 本地输出目录，当前不是核心代码路径。
```

## 核心流程注解

### `run` 流程

1. [cmd/vpings/main.go](cmd/vpings/main.go) 解析命令和参数。
2. `parseConfig` 把 `--icmp`、`--tcp`、`--udp`、`--quic` 转成 `probe.Spec`。
3. 每轮按顺序调用 `probe.Run`。
4. 每条结果追加到 JSONL。
5. 使用 `ui.RenderResults` 输出表格。

### `watch` 流程

1. CLI 参数转成一组 `probe.Spec`。
2. `ui.NewWatchModel` 创建 Bubble Tea 模型。
3. 每个 tick 并发运行一轮探测。
4. 结果追加到 JSONL，并在内存中保留最近 120 条用于显示。

### `app` 流程

1. 加载或创建 `vpings.json`。
2. 打开 `records.jsonl`。
3. 读取最近 50000 条历史记录。
4. `ui.NewAppModel` 进入完整 TUI。
5. 已启用探针按轮次并发执行；每个探针内部按 `sample_count` 串行采样。
6. 结果用于表格、实时图表和历史窗口图表。

## 修改入口建议

| 修改目标 | 优先查看 |
| --- | --- |
| 新增或调整 CLI 参数 | [cmd/vpings/main.go](cmd/vpings/main.go) |
| 新增探测协议 | [internal/probe/probe.go](internal/probe/probe.go) 与 `internal/probe/` 下新增协议文件 |
| 调整默认探针或配置格式 | [internal/appconfig/config.go](internal/appconfig/config.go) |
| 修改记录格式或历史读取策略 | [internal/store/jsonl.go](internal/store/jsonl.go) |
| 修改交互式菜单页面或按键 | [internal/ui/app.go](internal/ui/app.go) |
| 修改 watch 视图刷新逻辑 | [internal/ui/watch.go](internal/ui/watch.go) |
| 修改结果表格展示 | [internal/ui/table.go](internal/ui/table.go) |
| 修改图表、丢包率、聚合统计 | [internal/ui/chart.go](internal/ui/chart.go) |
| 增加构建目标或跨平台产物 | [Makefile](Makefile) |

## 测试与构建

运行全部测试：

```bash
go test ./...
```

运行 Makefile 测试任务：

```bash
make test
```

清理构建产物：

```bash
make clean
```

构建多平台二进制到 `dist/`：

```bash
make cross
```

当前 `cross` 目标会生成：

- Linux amd64 / arm64
- macOS amd64 / arm64
- Windows amd64

## 贡献说明

欢迎通过 Issue 或 Pull Request 参与改进。提交前建议：

1. 先运行 `go test ./...`。
2. 对行为变化补充或更新测试。
3. 保持改动聚焦，避免把格式化、重构和功能变化混在一个 PR。
4. 修改记录格式时说明兼容策略。
5. 修改 TUI 行为时同步更新 README 中的按键和流程说明。

推荐提交信息使用简短祈使句，例如：

```text
document app configuration flow
add jsonl legacy record tests
```

## 问题反馈

提交 Issue 时请尽量包含：

- 操作系统和架构。
- `vpings` 的运行命令。
- 目标协议和端口。
- 是否处于容器、VPN、代理或受限网络中。
- 相关错误输出或 `records.jsonl` 中的脱敏样例。

## 安全反馈

如果发现安全问题，请不要在公开 Issue 中贴出敏感目标、内网地址、凭据或完整抓包。当前仓库尚未配置正式安全通道；在补充 `SECURITY.md` 前，请先通过仓库所有者可用的私有联系方式沟通。

## 许可证

当前仓库尚未包含 `LICENSE` 文件，因此外部用户还不能明确判断复制、修改、分发和商用边界。若计划作为正式开源项目发布，请先选择并提交许可证，例如 MIT、Apache-2.0 或 GPL 系列。

## 后续开发注意事项

- 探针结果的归组依赖 `RoundID` 与 `ProbeID`，修改 ID 生成或记录格式时要同步检查图表汇总逻辑。
- UDP 的 `sent_no_reply` 是预期状态，不应简单当作实现失败。
- `app` 模式中每个探针的一轮包含多个样本，`run` 和 `watch` 的命令行模式目前每轮每个 `Spec` 只执行一次。
- `auto_start` 目前只是配置字段，真正的系统自启动注册还没有实现。
- 图表窗口默认值和最小缩放值定义在 `internal/ui/chart.go`，修改 UI 文案时要同时检查按键提示。
