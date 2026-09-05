# Codex Health Monitor

一个用于 [CLIProxyAPI（CPA）](https://github.com/router-for-me/CLIProxyAPI) 的原生 Go 插件，独立检测每个 Codex 凭证是否可用，并提供账号概览、异常原因、检测历史和自动调度面板。

> 当前发布产物面向 `linux/arm64` 和 `linux/amd64`，需要支持标准动态库插件的 CPA 版本。

## 功能

- 按 `auth_index` 独立检测每个 Codex 凭证，避免账号之间相互影响。
- 固定使用 `gpt-5.6-luna` 发起最小探测请求。
- 同时校验 HTTP 状态、完整的 Responses/SSE 流以及最终输出 `OK`。
- 区分未授权、额度异常、限流、超时、网络错误和上游错误等状态。
- 支持按分钟间隔或每天固定时间自动检测。
- 支持只检测指定邮箱，历史最多保留 100 次运行记录。
- 账号停用/启用状态实时同步：面板直接读取 CPA 的实时凭证状态，停用后立即显示已停用，重新启用后自动撤回过期的停用结果。
- 内置响应式管理页面，适配桌面和手机浏览器。

插件只读取探测所需的访问令牌，不会禁用、删除、刷新或改写凭证。令牌不会写入状态、历史、管理接口响应或日志。

## 安装

### 前置条件

- 一台运行 `linux/arm64` 或 `linux/amd64` 的 CPA 主机。
- CPA 已配置 Codex 凭证。
- CPA 的 Management API 已启用，即 `remote-management.secret-key` 非空。
- 使用预编译文件时不需要在 CPA 主机安装 Go 或 Docker。

### 使用 Release 产物

1. 从项目的 [Releases](https://github.com/hg3386628/codex-health-monitor/releases) 页面下载与主机架构匹配的产物：`codex-health-monitor-linux-arm64.so` 或 `codex-health-monitor-linux-amd64.so`。

2. 将动态库复制到 CPA 的插件目录。以下示例假设 CPA 安装在 `/opt/cli-proxy-api`，且 `plugins.dir` 使用默认值 `plugins`：

```bash
sudo install -D -m 0755 codex-health-monitor-linux-arm64.so \
  /opt/cli-proxy-api/plugins/linux/arm64/codex-health-monitor.so
```

如果主机是 `linux/amd64`，请下载 amd64 产物并安装到 `/opt/cli-proxy-api/plugins/linux/amd64/codex-health-monitor.so`。CPA 会依次查找 `<plugins.dir>/linux/<架构>/` 和 `<plugins.dir>/`。如果你的 `plugins.dir` 是绝对路径，请相应替换上面的目标目录。

3. 编辑 CPA 的 `config.yaml`，打开全局插件开关并启用本插件：

```yaml
plugins:
  enabled: true
  dir: "plugins"
  configs:
    codex-health-monitor:
      enabled: true
      priority: 1
      schedule_mode: interval
      interval_min: 30
      daily_times: ""
      timezone: Asia/Shanghai
      timeout_sec: 30
      target_emails: ""
```

`plugins.enabled` 是全局开关，`plugins.configs.codex-health-monitor.enabled` 是插件实例开关，两者都必须为 `true`。

4. 重启 CPA。使用 systemd 的安装可执行：

```bash
sudo systemctl restart cli-proxy-api
sudo systemctl status cli-proxy-api --no-pager
```

如果你的服务名不是 `cli-proxy-api`，请替换为实际 unit 名称；直接运行或使用其他进程管理器时，按原方式重启 CPA。

5. 打开 CPA 管理中心，在插件列表中找到 **Codex Health Monitor**，进入插件页面。也可以直接访问：

```text
http://<CPA_HOST>:8317/v0/resource/plugins/codex-health-monitor/panel
```

首次加载页面会复用 CPA 管理中心中的管理员密钥；密钥失效时，页面会要求重新输入。插件加载后不会立即自动检测，点击“立即检测”可执行第一次检查。

### Docker 部署的 CPA

将插件目录和配置文件挂载到 CPA 容器，并保证容器内目录与 `plugins.dir` 一致：

```yaml
services:
  cli-proxy-api:
    image: router-for-me/cli-proxy-api:latest
    volumes:
      - ./config.yaml:/CLIProxyAPI/config.yaml
      - ./plugins:/CLIProxyAPI/plugins
```

宿主机上的文件结构应为：

```text
plugins/
└── linux/
    ├── arm64/
    │   └── codex-health-monitor.so
    └── amd64/
        └── codex-health-monitor.so
```

修改配置并放好动态库后，重建或重启容器：

```bash
docker compose up -d --force-recreate cli-proxy-api
docker compose logs --tail=100 cli-proxy-api
```

## 配置

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `schedule_mode` | `interval` | `interval` 按间隔执行；`daily_times` 按固定时间执行 |
| `interval_min` | `30` | 检测间隔，范围为 5 到 10080 分钟；实际执行时间会在此间隔上附加 0~5 分钟随机抖动 |
| `daily_times` | `""` | 逗号分隔的 `HH:mm`，最多 12 个且不能重复 |
| `timezone` | `Asia/Shanghai` | IANA 时区，例如 `UTC` 或 `Asia/Shanghai` |
| `timeout_sec` | `30` | 单账号超时，范围为 5 到 120 秒 |
| `target_emails` | `""` | 逗号分隔的邮箱；留空检测全部 Codex 账号 |

固定时间示例：

```yaml
schedule_mode: daily_times
daily_times: "09:00,13:00,18:00"
timezone: Asia/Shanghai
```

间隔模式的随机抖动说明：每次计算下一次执行时间时，会在 `interval_min` 的基础上独立随机附加 0~5 分钟。例如 `interval_min: 30` 时，实际间隔在 30~35 分钟之间浮动，且每次不同。面板和 `state.json` 中显示的 `next_run_at` 已包含抖动。`daily_times` 模式不附加抖动，始终按配置的精确时刻执行。

停用状态实时同步说明：`/accounts` 接口和面板会在每次刷新时读取 CPA 当前的凭证启用状态。账号在 CPA 中被停用后，面板立即显示"已停用"，无需等待下一次检测；重新启用后，上一轮检测留下的停用结果会被自动撤回并显示"未检测"，直到下一轮检测重新确认。检测历史记录不会因此被改写。

面板中保存的调度会写入插件数据目录的 `state.json`，并在后续启动时继续使用。

## 从源码构建

仓库提供的脚本会在 `golang:1.24-bookworm` 容器中运行测试并交叉构建动态库，默认目标为 `linux/arm64`：

```bash
git clone https://github.com/hg3386628/codex-health-monitor.git
cd codex-health-monitor
chmod +x build.sh
./build.sh
```

产物位于：

```text
dist/codex-health-monitor-linux-arm64.so
```

可通过环境变量覆盖版本、目标架构和 Go 镜像。目标架构支持 `arm64` 和 `amd64`：

```bash
VERSION=0.1.5 ARCH=amd64 GO_IMAGE=golang:1.24-bookworm ./build.sh
```

构建 arm64：

`ARCH=arm64 ./build.sh`

构建 amd64：

`ARCH=amd64 ./build.sh`

本机具备 Go 1.24、C 编译器和目标平台 CGO 工具链时，也可以设置 `BUILD_WITH_DOCKER=0`。普通开发测试不需要构建动态库：

```bash
go test ./...
go test -race ./...
go vet ./...
```

## 健康判定

账号只有同时满足以下条件才会标记为健康：

1. 上游返回 HTTP 2xx。
2. Responses/SSE 流完整结束，并包含 `response.completed`。
3. 去除首尾空白后的输出严格等于 `OK`。

原始响应只在内存中解析，完成分类后立即丢弃。

## 管理接口

```text
GET  /v0/management/plugins/codex-health-monitor/status
GET  /v0/management/plugins/codex-health-monitor/accounts
GET  /v0/management/plugins/codex-health-monitor/history
POST /v0/management/plugins/codex-health-monitor/run
GET  /v0/management/plugins/codex-health-monitor/schedule
POST /v0/management/plugins/codex-health-monitor/schedule
```

向 `run` 接口发送 `{"wait":true}` 可等待本轮检测结束。所有 Management API 请求都需要 CPA 管理员密钥。

## 升级与卸载

升级前停止 CPA，替换 `codex-health-monitor.so` 后重新启动。动态库已被进程加载时不应直接覆盖。

卸载时停止 CPA，删除动态库，并移除 `plugins.configs.codex-health-monitor` 配置块。若不再使用任何插件，也可以将 `plugins.enabled` 设为 `false`。

## 排查

- 插件没有出现在列表：确认系统架构是 `linux/arm64` 或 `linux/amd64`，并将对应产物重命名为 `codex-health-monitor.so` 放入相应架构目录，同时检查 `plugins.dir` 与实际挂载路径。
- 插件存在但未启用：确认全局和实例两个 `enabled` 都为 `true`，然后查看 CPA 启动日志。
- 页面返回 401：重新输入 `remote-management.secret-key` 对应的管理员密钥。
- 没有账号：确认 CPA 中存在类型为 `codex` 的凭证，并检查 `target_emails` 是否过滤了全部账号。
- 检测超时：确认 CPA 可以访问 `https://chatgpt.com`，必要时适当增加 `timeout_sec`。

## 社区

交流、反馈和分享可以前往 [LINUX DO](https://linux.do/)。提交 Bug 时请附上 CPA 版本、CPU 架构和已脱敏的插件日志，切勿公开访问令牌或管理员密钥。

## License

[MIT](LICENSE)
