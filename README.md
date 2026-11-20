# Ada Chat

一个原生桌面聊天窗口——没有 webview、没有 HTML、没有 JavaScript。窗口、对话列表和对话区都用纯 Go
配合 [MyGo](https://github.com/egoist/mygo) 绘制，视觉来自
[Lobe MyGo](https://mygo.lobehub.com) 的组件与 token。

```bash
go run ./cmd/ada_chat   # 真正的帧，GPU 全开
build.bat               # 编译 Windows 可执行文件到 bin\ada_chat.exe
go test ./...           # 状态与交互，全部由测试驱动
```

## 编译

双击 `build.bat`，或从终端运行：

```bat
build.bat        :: 构建；双击时结束后暂停
build.bat -q     :: 构建后直接返回，给终端或 CI 用
```

它按 MyGo 官方发布参数构建：`-trimpath`、`-tags mygo_noinspector`、
`-ldflags "-s -w -H=windowsgui -X github.com/egoist/mygo.production=1"`，
去掉调试符号与开发者 inspector，并以 Windows GUI 子系统启动（不带控制台窗口）。
产物在 `bin\ada_chat.exe`，脚本会打印大小；缺少 `go` 或构建失败时返回非零退出码。

## 目录结构

```
ada_chat/
├── cmd/ada_chat/main.go        # 装配：应用状态、视图、窗口
├── build.bat                   # Windows 编译脚本，产物在 bin/
├── internal/
│   ├── app/                    # 状态与行为，只依赖标准库
│   │   ├── app.go              # 会话、消息、偏好、发送与回复
│   │   ├── seeds.go            # 内置对话、固定回答、相对时间
│   │   ├── tools.go            # 工具类型、调用计划、运行状态
│   │   └── app_test.go         # 发送、重生成、删除与工具计划的测试
│   └── ui/                     # 视图，读 app 并用 MyGo + Lobe 绘制
│       ├── view.go             # UI 状态与根视图
│       ├── window.go           # 窗口参数
│       ├── titlebar.go         # 自绘标题栏、模型菜单
│       ├── sidebar.go          # 侧栏、工作区切换、分组列表
│       ├── chat.go             # 对话卡片、消息行、消息操作
│       ├── composer.go         # 输入框、Enter 处理、发送按钮
│       ├── dialogs.go          # 设置、重命名、删除
│       ├── tools.go            # 可折叠的工具调用卡片
│       ├── ui_test.go          # ui.NewTester 交互测试
│       ├── snapshot_test.go    # 视觉快照
│       └── testdata/preview/   # 浅色、深色、空状态、设置、失败工具
├── go.mod
└── README.md
```

分层只有一条规则：`internal/app` 不知道 MyGo 的存在；`internal/ui` 只读应用状态；
`cmd/ada_chat` 把两者装进窗口。运行中的回复由 `app.SetDispatch` 交给 `Window.Update`，
测试里不设 dispatch，同一条路径同步跑完，所以帧是确定的。

## 窗口里有什么

- **标题栏**——界面自己在系统窗口按钮下方画出标题栏（收起侧栏、品牌、模型菜单、设置）。
  `c.TitleBar()` 避开窗口按钮，`DragWindow` 负责拖动窗口。
- **侧栏**——`layout.DraggablePanel`（240–360 DIP，可收成窄条），里面有工作区切换、
  绑定 `mod+k` 的 `field.SearchBar`，对话按“置顶 / 今天 / 更早”分组。
- **对话卡片**——头部用 `display.Tag` 标示已置顶与模型，菜单负责置顶、重命名、复制对话和删除；
  对话区是 `ui.List` + `ListState{FollowEnd: true, FollowAnimation: &ui.Smooth}`，
  流式输出时始终停在最新一条。回复经 `markdown.Markdown` 渲染，流式期间 `Animated: true`，
  每条下方有 `action.ActionIconGroup` 操作（复制回复、重新生成、删除消息）。
- **工具调用**——助手回答前先调用本地工具，对话里出现一张可折叠的卡片：工具名、状态
  （运行中 / 已完成 / 失败）、耗时；展开后是参数（`code.Snippet` 高亮 JSON）和结果
  （Markdown，失败时是 `feedback.Alert`）。卡片由 `ui.CollapsibleBase` 开合，键盘和读屏可用。
- **空状态**——`display.Empty` 加三张 `layout.Block` 建议卡。
- **输入框**——浮起的卡片里放 `field.TextArea`；Enter 发送、Shift+Enter 换行，
  设置里可以改成 Cmd/Ctrl+Enter 发送。发送按钮是主色圆形，草稿为空时禁用。
- **设置**——`feedback.Modal` 套 `form.View` / `form.Field`：显示名称、模型、
  流式输出、Enter 发送。保存后立刻作用到头部模型、侧栏底部和流式输出本身。
- **反馈**——根部放 `feedback.ToastHost`；复制、删除（带撤销）、重命名、保存都有 toast。

状态放在 `internal/app`，视图只读它。深浅色来自 `theme.Apply(c)` 与 `theme.TokensOf(c)`，
没有写死颜色。

应用不带网络客户端：`internal/app/seeds.go` 的 `ReplyText` 按问题挑一段固定回答再逐字送出，
`internal/app/tools.go` 的 `toolPlan` 按问题挑要调用的工具（搜索文档、读取文件、运行测试、
检索上下文、拉取评审），所以流式与工具调用这两条链路都是真的、可测的。把它们换成模型和真实工具，
`internal/ui` 不用改。

## 测试与预览

```bash
go build ./... && go vet ./... && go test ./...
CGO_ENABLED=0 go test ./...                              # 发布构建不需要 cgo
ADA_SNAPSHOT=1 go test ./internal/ui -run TestSnapshots  # 重新生成 testdata/preview
```

无头测试能证明文字、布局、焦点和事件分派，但证明不了目标机器上的 GPU 渲染：
发布前仍然需要跑一次 `go run ./cmd/ada_chat`。
