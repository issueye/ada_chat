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
# 1. 构建 ada_go 的 RPC bot 宿主（模块名是 ada_go_v2，目录名是 ada_go）
Push-Location ..\ada_go
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

机器人与对话另行保存在同目录的 `state.json`：添加/编辑机器人、发送消息、一轮结束、
改名/删除对话以及关窗时都会原子写入；下次启动自动恢复侧栏机器人、对话与消息（含工具卡片）。
进程状态不落盘——恢复后的机器人是「未启动」，点机器人或发第一条消息时才起进程；
重启前还在生成的消息会标记为「回答未完成」，跑着的工具记为中断。

API Key 保存在这个本机配置文件中（文件权限 0600）；不想落盘时把它留空，改由
`ADA_LLM_API_KEY` 提供。`ADA_BOT_RUNTIME_ARGS` 仍只从环境读取。

设置影响的是**新启动的工作区进程**；已在运行的机器人不会自动重建。要让当前机器人立即换用新
runtime/供应商，重启应用即可（关闭时会回收所有进程）。

如果不想在界面里配置，也可以先用环境变量启动一次：

```powershell
$env:ADA_BOT_RUNTIME = "E:\codes\go_projects\ada_work\ada_go\bin\ada-rpc-agent.exe"
$env:ADA_GO_AGENT_PRESET = "E:\codes\go_projects\ada_work\ada_go\examples\agent\agent.preset.json"
$env:ADA_LLM_PROVIDER = "replay"
go run ./cmd/ada_chat
```

首次启动是空工作台；点「添加机器人」，选择、输入或浏览一个**工作目录**，再发送第一条消息。

右键侧栏里的机器人可以「新建会话」「编辑信息」或「启动/重启进程」：新建会话会立刻切过去，
并在 runtime 侧拿到独立的 agent session（新对话不会带上旧对话的模型上下文）；改名与图标立即生效；
工作目录改动会重启该机器人的进程；名称与系统提示词由每轮 `chat.send` 带给 runtime，**下一轮**即生效，
不必重启进程（preset/供应商仍是进程级，改了要重启进程）。

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
│   │   ├── tools.go            # 工具调用类型与 chat/tool 状态
│   │   └── state.go            # 机器人/对话的 state.json 读写与重启后归一化
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
5. 一个机器人可以有多条对话：每条对话带自己的 `sessionId`，runtime 为每个 `sessionId` 建独立 agent session，
   所以「新建会话」等于开一个干净的模型上下文；同一个进程、preset、模型仍然共享。
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
| `chat.send` | `{turn, text, botId, botName, instruction, sessionId}` | `{turn, text}`；期间 `chat/reasoning`、`chat/delta`、`chat/tool`、`chat/approval`、`chat/error`，最后 `chat/done` |
| `chat.approve` | `{id, allow, reason}` | `{id, allow}`；回答一次 `chat/approval`，未知 id 返回错误 |

## 流式回答与工具卡片

`ada-rpc-agent` 现在把 agent-loop 的 `llm.EventStream` 与 `tools/*` 事件实时转发给 ada_chat：

- 文本增量走 `chat/delta`，`App.BotDelta` 追加到消息，`Animated` Markdown 边到边渲染；
- 模型的思考内容走 `chat/reasoning`，`App.BotReasoning` 按顺序保留为独立的思考块，
  界面显示成默认展开的「思考过程」卡片，可点标题收起；它和正文、工具按真实先后排列；
- 机器人生成期间，输入框的发送按钮变成「停止生成」的方形停止图标；点击会取消这一轮的 RPC
  调用，模型/工具随之停止，消息保留并标注「已停止生成」，进程本身继续运行，可以直接发下一条。
  对话头部的「…」菜单里也有同一个「停止生成」；
- 本轮结束后输入框恢复成发送箭头，草稿保留；
- 工具开始/结束走 `chat/tool`（`running` → `ok`/`denied`/`error`/`timeout`），
  `App.BotTool` 按 `turn + id` 建卡片并更新参数、结果、耗时；卡片**默认折叠**，只显示工具名、
  状态与耗时，点标题才展开参数和结果；
- 一轮回答按真实顺序保存：`Message.Parts` 记录「文字 → 工具 → 文字」的先后，
  界面按这个顺序渲染，不再固定把工具卡片放在文字上方；工具并行执行时事件可能乱序，
  卡片顺序按首次出现固定，结果回填到对应 id。
- 工具或沙箱要越权时，runtime 发 `chat/approval`；界面弹出「需要审批」面板，显示机器人、发起方、
  目标与原因，选「允许一次」/「拒绝」后经 `chat.approve` 回写。超时未答复由 runtime 按拒绝处理
  （fail-closed），面板会随该轮结束自动关闭。
- 本轮失败时 runtime 先发 `chat/error`，对话直接显示可读原因；只有事件没到（进程断开等）才回落到
  RPC 错误文本与机器人日志末行。

回答里出现 `run_shell` 卡片，说明这次 preset 启用了 ada_go_v2 的 `shell-helper` 插件。命令实际上由
`ada_go_v2` 进程在本机执行：`agent-release` preset 使用 `sandbox-none`（**policy-only，不做 OS 级隔离**）
与 `workspace-write` 模式，权限等同当前用户。只读模式下 `shell-guard` 白名单会拦住未列出的程序；
发布预设没有接审批插件，拒绝一律 fail-closed（想试审批可把 preset 换成
`ada_go/examples/agent/approval.preset.json`，它把 `read_file` 设为需要审批）。
安全说明见 ada_go_v2 的 `docs/tool-shell-helper.md`。

> 当前 ada_go_v2 宿主在一个进程内为每个 `botId` 建独立 session，并按 session 替换
> system prompt 里的身份段（名称/指令下一轮生效）；但 preset、模型与 LLM provider 仍是进程级共享，
> 这是项目群组方向要补的核心缺口。replay provider 的游标也是进程级，多个机器人共用
> replay preset 时会耗尽脚本；真实 LLM 不受此限。详见分析文档。

## seam RPC v1

实现对应 ada_go_v2 的 `docs/seam-rpc.md` 的 P0–P2：

- 4 字节大端长度（含 kind）+ 1 字节 kind + JSON body；帧/参数/在飞/事件/连接数都有上限。
- `challenge → auth → ready`；token 模式用 HMAC-SHA256 挑战应答；`os`/`none` 模式按 docs 校验。
- 一条连接多路复用请求/响应；订阅 + `seq` 事件；取消、ping/pong、错误码。
- 传输 `tcp`/`unix`/Windows 命名管道；pipe 用 `FILE_FLAG_OVERLAPPED` + 每操作事件，
  DACL 只授当前用户与 SYSTEM，拒绝远程客户端。
- 客户端仍未实现 P3/P4：caps/guard、rpc/observe、TLS/远程 TCP、`SO_PEERCRED`、独立写队列。
  ada_go_v2 服务端已经实现并挂载 `rpc-guard`/`rpc-observe`（见其 `cmd/ada-rpc-agent/README.md`）。
- 本包只保留客户端：早期的 `server.go`、`Service.Listen` 与传输层监听实现已删除，
  「chat 也当服务端」的互操作只留在历史记录里；协议以 ada_go 的 `docs/seam-rpc.md` 为准。

## 验证

自动化测试按此前的约定基本移除，但补回了一个跨仓库的端到端互操作测试（默认跳过，只有配置了
外部 runtime 才真正拉起进程）：

```bash
go build ./...
go vet ./...
GOOS=linux CGO_ENABLED=0 go build ./...   # 非 Windows 也能构建

# 跨仓库互操作（先构建 ada_go 的 bin/ada-rpc-agent）
$env:ADA_INTEROP_AGENT  = "E:\codes\go_projects\ada_work\ada_go\bin\ada-rpc-agent.exe"
$env:ADA_INTEROP_PRESET = "E:\codes\go_projects\ada_work\ada_go\examples\agent\agent.preset.json"
$env:ADA_INTEROP_APPROVAL_PRESET = "E:\codes\go_projects\ada_work\ada_go\examples\agent\approval.preset.json"
$env:ADA_INTEROP_BASE   = "E:\codes\go_projects\ada_work\ada_go"
go test ./internal/rpc -run TestInterop -v -count=1
```

- `TestInteropReplayChat`：握手 → `bot.info` → `chat.send`（replay）→ `chat/delta`/`chat/tool`/`chat/done`，
  断言回答里有 `hello`、事件完整；
- `TestInteropApproval`：用 `approval.preset.json` 让 `read_file` 需要审批，测试充当宿主回 `chat.approve`
  允许一次，断言工具继续执行、回答正确；
- `TestInteropSessionIsolation`：同一个机器人用不同 `sessionId` 连发三轮，用 mock OpenAI 端点确认
  新会话第一轮只带 `system + user`，不带旧会话的模型上下文；
- `TestInteropPerBotPersona`：mock OpenAI 端点捕获系统提示，断言同一进程里的第二个机器人看到
  自己的名字与指令，而不是进程级默认身份。

手工/临时探针验证过的行为：

- 两个机器人指向同一个临时目录：只启动 1 个 runtime 进程；
- 真实 replay agent 读取工作目录里的 `a.txt` 并返回 `文件内容是 hello`；
- 移除一个机器人后进程仍在（还有另一个成员），移除最后一个后实例消失；
- 一个机器人「添加 → 发送 → 关闭应用 → 重新打开」后仍在侧栏，对话与工具卡片一起恢复，
  机器人按「未启动」状态显示，发消息时再起进程（`internal/app/state_test.go` 覆盖归一化与 id 回填）；
- 设置页保存的运行时/preset 路径写入 `config.json`，新进程启动时用这份配置，不依赖环境变量；
- 保存 OpenAI 兼容供应商（Endpoint/模型/API Key）后，runtime 会启用 `llm-openai`、禁用 replay，
  经 mock OpenAI SSE 端点验证：请求携带保存的 Bearer key，回复来自该供应商，`bot.info` 模型回写；
- 真实 agent 调用 `run_shell` 执行 `echo probe-ok`：先收到 running 卡片，再收到结果；
  首个文本增量早于轮次结束（真流式），卡片在 UI 里显示「已完成 / 84ms / probe-ok」；
- 发布包风格 preset（`data/…` 相对包根 + `.ada-base` 锚点）可以经 `ada-rpc-agent` 装配，
  不再要求从包根启动。

仍建议补：工作区单进程的 supervisor 测试、RPC 帧/握手的契约测试。
