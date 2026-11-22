# Ada Chat

一个原生桌面聊天窗口——没有 webview、没有 HTML、没有 JavaScript。窗口、机器人侧栏和对话区都用纯 Go
配合 [MyGo](https://github.com/egoist/mygo) 绘制，视觉来自
[Lobe MyGo](https://mygo.lobehub.com) 的组件与 token。

ada_chat 是**宿主/桌面壳**：它管理「工作区 → 进程实例」、机器人、对话和进程生命周期；
真正的 agent 运行在 ada_go_v2 的进程里，两边通过 seam RPC v1 通讯。启动时没有预置对话或固定回答，
需要配置一个外部 runtime。

> 产品方向、职责边界和待补能力见 [docs/product-analysis.md](docs/product-analysis.md)
> （个人机器人助手 / 项目群组）。

## 运行

```powershell
# 1. 构建 ada_go_v2 的 RPC bot 宿主
Push-Location ..\ada_go_v2
go build -o bin\ada-rpc-agent.exe .\cmd\ada-rpc-agent
Pop-Location

# 2. 启动 ada_chat，在设置里配置 Agent 与模型供应商
go run ./cmd/ada_chat
```

设置页现在是三个页签：

- **Agent**：`Agent 路径`（`ada-rpc-agent` 可执行文件）与 `Agent preset`（`agent.preset.json`），
  都可以直接输入或打开文件选择器；
- **供应商**：选择 `跟随 preset` / `OpenAI 兼容` / `Replay（离线脚本）`。
  - OpenAI 兼容：Endpoint、模型、API Key；选 Replay 不访问网络；
  - 供应商覆盖由 runtime 在装配 preset 时生效，新进程使用，已运行进程不受影响；
- **常规**：显示名称、Enter 发送。

保存时会校验运行时/preset 文件、Endpoint 格式，并写入
`os.UserConfigDir()/ada_chat/config.json`（Windows 为 `%AppData%\ada_chat\config.json`），
下次启动自动加载。保存过的设置优先；对应项留空时回落到环境变量：

`ADA_BOT_RUNTIME` / `ADA_GO_AGENT_PRESET` / `ADA_LLM_PROVIDER` / `ADA_LLM_ENDPOINT` / `ADA_LLM_MODEL`。

API Key 保存在这个本机配置文件中（文件权限 0600）；不想落盘时把它留空，改由
`ADA_LLM_API_KEY` 提供。`ADA_BOT_RUNTIME_ARGS` 仍只从环境读取。

设置影响的是**新启动的工作区进程**；已在运行的机器人不会自动重建。要让当前机器人立即换用新
runtime/供应商，重启应用即可（关闭时会回收所有进程）。

如果不想在界面里配置，也可以先用环境变量启动一次：

```powershell
$env:ADA_BOT_RUNTIME = "E:\codes\go_projects\ada_go_v2\bin\ada-rpc-agent.exe"
$env:ADA_GO_AGENT_PRESET = "E:\codes\go_projects\ada_go_v2\examples\agent\agent.preset.json"
$env:ADA_LLM_PROVIDER = "replay"
go run ./cmd/ada_chat
```

首次启动是空工作台；点「添加机器人」，选择、输入或浏览一个**工作目录**，再发送第一条消息。

```bat
build.bat        :: 构建 bin\ada_chat.exe；双击时结束后暂停
build.bat -q     :: 构建后直接返回
```

构建使用 MyGo 发布参数（`-trimpath`、`-tags mygo_noinspector`、GUI 子系统），产物在 `bin\ada_chat.exe`。

## 当前结构

```
ada_chat/
├── cmd/ada_chat/main.go        # 装配状态、工作区进程管理器、视图、窗口
├── build.bat
├── docs/product-analysis.md    # 两条产品方向与职责/缺口分析
├── internal/
│   ├── app/                    # 状态与行为（只依赖标准库）
│   │   ├── app.go              # 机器人、会话、消息、偏好、RPC 回写
│   │   └── tools.go            # 工具调用类型（等 agent 事件）
│   ├── rpc/                    # seam RPC v1：帧/握手/调用/事件/取消/心跳 + 传输
│   ├── bot/                    # 工作区进程管理
│   │   ├── supervisor.go       # 工作目录 → 一个进程/一个 peer；bot 是成员
│   │   ├── endpoint.go         # 每次启动的私有 pipe/unix 地址和 token
│   │   └── bot.go              # runtime 环境契约
│   └── ui/                     # 视图，读 app 并用 MyGo + Lobe 绘制
│       ├── sidebar.go          # 添加机器人、机器人行、对话分组
│       ├── bots.go             # 机器人图标/状态、工作目录选择、添加弹窗
│       ├── chat.go             # 对话卡片、欢迎页、消息行、工具卡片
│       ├── composer.go         # 输入框、Enter 处理、发送按钮
│       ├── dialogs.go          # 设置、重命名、删除
│       └── tools.go            # 可折叠的工具调用卡片
├── go.mod
└── README.md
```

分层：`internal/app` 不知道 MyGo 与 RPC；`internal/ui` 只读应用状态；
`internal/bot` 实现 `app.BotEngine` 并把进程/RPC 事件回写到 App；
`cmd/ada_chat` 把三者装进窗口。

## 进程与工作区模型

1. 进程实例的 key 是**规范化后的工作目录**，不是机器人 ID。
2. 同一个目录下添加第二个机器人时，`Supervisor` 只是把它加进现有实例的成员集，
   **不会启动第二个进程**；所有成员共享同一个 pid、peer 和日志。
3. 新成员加入运行中的实例时立即拿到 `Running` 状态；进程失败时所有成员一起变成 `Error`。
   最后一个成员离开（或应用关闭）才关闭 peer 并杀掉进程。
4. 每轮 `chat.send` 都记录 `turn → botID`；共享进程返回的 `chat/delta` 仍写回发起提问的那个机器人的会话。
5. 进程状态、pid、`bot.info` 返回的真实模型/工作目录会广播给该工作区的全部机器人。
6. `Supervisor.Instances()` 提供只读诊断信息（工作目录、pid、成员数、状态），供未来的进程面板使用。

Runtime 契约（ada_chat 启动外部进程时设置）：

| 变量 | 说明 |
|---|---|
| `ADA_BOT_ADDR` | `pipe://name` / `unix:///abs/path` / `tcp://host:port` |
| `ADA_BOT_TOKEN` | RPC token，只走环境 |
| `ADA_BOT_NAME` / `ADA_BOT_MODEL` / `ADA_BOT_INSTRUCTION` | 首个机器人的配置（共享进程的初始配置） |
| `ADA_BOT_WORKSPACE` | 选中目录；进程的 `cwd` 也是它 |

RPC 方法：

| 方法 | 参数 | 结果/事件 |
|---|---|---|
| `bot.info` | — | `{name, model, workspace, pid}` |
| `chat.send` | `{turn, text, botId, botName, instruction}` | `{turn, text}`；期间 `chat/delta`，最后 `chat/done` |

> 当前 ada_go_v2 宿主在一个进程内为每个 `botId` 建独立 session，但 preset、system prompt 和 LLM provider
> 仍是进程级共享；这是项目群组方向要补的核心缺口。replay provider 的游标也是进程级，多个机器人共用
> replay preset 时会耗尽脚本；真实 LLM 不受此限。详见分析文档。

## seam RPC v1

实现对应 ada_go_v2 的 `docs/seam-rpc.md` 的 P0–P2：

- 4 字节大端长度（含 kind）+ 1 字节 kind + JSON body；帧/参数/在飞/事件/连接数都有上限。
- `challenge → auth → ready`；token 模式用 HMAC-SHA256 挑战应答；`os`/`none` 模式按 docs 校验。
- 一条连接多路复用请求/响应；订阅 + `seq` 事件；取消、ping/pong、错误码。
- 传输 `tcp`/`unix`/Windows 命名管道；pipe 用 `FILE_FLAG_OVERLAPPED` + 每操作事件，
  DACL 只授当前用户与 SYSTEM，拒绝远程客户端。
- 未实现的部分（P3/P4）：caps/guard、rpc/observe、TLS/远程 TCP、`SO_PEERCRED`、独立写队列。

与 ada_go_v2 参考实现做过双向互操作：ada_chat 客户端 ↔ ada_go_v2 服务端（TCP/pipe），
ada_go_v2 客户端 ↔ ada_chat 机器人服务端（pipe）。互操作中发现 ada_go_v2 的 pipe 同步句柄会在
RPC 客户端「读循环 + 写请求」时死锁，已改为 overlapped 流并加 `TestRPCOverPipe` 回归测试。

## 验证

按本轮要求移除了全部自动化测试、快照和演示数据，所以仓库里没有 `*_test.go`。验证目前是：

```bash
go build ./...
go vet ./...
GOOS=linux CGO_ENABLED=0 go build ./...   # 非 Windows 也能构建
```

手工/临时探针验证过的行为：

- 两个机器人指向同一个临时目录：只启动 1 个 runtime 进程；
- 真实 replay agent 读取工作目录里的 `a.txt` 并返回 `文件内容是 hello`；
- 移除一个机器人后进程仍在（还有另一个成员），移除最后一个后实例消失；
- 设置页保存的运行时/preset 路径写入 `config.json`，新进程启动时用这份配置，不依赖环境变量；
- 保存 OpenAI 兼容供应商（Endpoint/模型/API Key）后，runtime 会启用 `llm-openai`、禁用 replay，
  经 mock OpenAI SSE 端点验证：请求携带保存的 Bearer key，回复来自该供应商，`bot.info` 模型回写。

重新引入测试时，优先补三类：工作区单进程的 supervisor 测试、RPC 契约测试、
真实 runtime 的端到端测试。
