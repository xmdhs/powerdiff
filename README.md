# powerdiff

Windows 电源计划设置查看、对比与管理工具。

通过读取 Windows `powrprof.dll` 电源 API，列出所有电源方案的子组（Subgroup）与设置项（Setting），
对比当前值与默认值（或方案之间）的差异，并支持在 Web 可视化界面中直接修改、导入 / 导出。

> 仅支持 Windows。Web 后端在非 Windows 上可启动，但电源相关 API 会返回 `unsupported`。

## 包含的两个程序

| 程序 | 入口 | 说明 |
| ---- | ---- | ---- |
| `powerweb` | `cmd/powerweb` | Web 可视化管理工具（推荐）。启动本地 HTTP 服务 + 自动打开浏览器，前端静态资源已 `embed` 打包，单文件即用 |
| `powerdiff` | `powerdiff_windows.go`（仓库根目录） | 命令行对比工具。打印当前电源方案中与默认值不同的设置 |

Release 中一次提供 4 个二进制：两个程序 × `windows/amd64` + `windows/arm64`。

## 功能特性

- 电源方案管理：列出全部方案、查看当前活动方案、一键切换活动方案
- Overlay（电源模式滑杆，如最佳能效 / 最佳性能）查看与切换（系统不支持时自动降级为空列表）
- 设置浏览：按子组分组展示，显示名称、描述、GUID、隐藏状态、数值范围 / 可选值
- 搜索过滤：按名称、分组、GUID 模糊搜索（`Ctrl+K` 聚焦，`Esc` 清空）
- 修改设置：分别设置 AC（接通电源）/ DC（电池）值，后端做范围与步进校验
- 显示 / 隐藏设置项（改写 `Attributes` 注册表属性）
- 对比：
  - 与默认值对比（AC 当前值 vs AC 默认值，DC 当前值 vs DC 默认值）
  - 方案与方案之间对比
  - 对比结果可导出为 JSON
- 导入 / 导出：
  - 导出全量设置为 XML（包含所有方案的 AC/DC 索引）
  - 从 XML 导入并批量写回（导入前前端会二次确认）
  - 导出为 `powercfg /setacvalueindex /setdcvalueindex` 批处理脚本（`.cmd`）
- 异构 CPU 阈值（Hetero Increase/Decrease Threshold Class1/2）特殊支持：可新增 Possible Value（REG_BINARY）
- 安全限制：`/api/*` 仅允许 `localhost` 回环访问，非本机请求直接 `403`

## 下载

到 [Releases](../../releases) 页下载最新版，根据 CPU 架构选择：

| 文件 | 说明 |
| ---- | ---- |
| `powerweb-windows-amd64.exe` | Web 工具，Intel / AMD 64 位（大多数人选这个） |
| `powerweb-windows-arm64.exe` | Web 工具，ARM64（如骁龙本） |
| `powerdiff-windows-amd64.exe` | 命令行工具，x64 |
| `powerdiff-windows-arm64.exe` | 命令行工具，ARM64 |
| `SHA256SUMS.txt` | 校验和 |

校验示例（PowerShell）：

```powershell
certutil -hashfile powerweb-windows-amd64.exe SHA256
# 与 SHA256SUMS.txt 中的对应行比对
```

构建来源可验证：每个 Release 的二进制都附带 Sigstore build provenance attestation，
可在 Release 资产或仓库 Attestations 中查看，用 `gh` 验证示例：

```bash
gh attestation verify powerweb-windows-amd64.exe --owner xmdhs
```

## 快速开始

### powerweb（推荐）

双击运行，或在终端运行（**修改设置需要以管理员身份运行**，仅查看则不需要）：

```powershell
.\powerweb-windows-amd64.exe
```

启动后自动打开浏览器，地址类似 `http://127.0.0.1:52341/`（端口随机分配）。
按 `Ctrl+C` 停止服务。

常用启动参数：

```powershell
# 指定监听地址（默认 127.0.0.1:0，即随机端口）
.\powerweb-windows-amd64.exe -addr 127.0.0.1:8080

# 不自动打开浏览器
.\powerweb-windows-amd64.exe -no-browser

# 输出调试日志
.\powerweb-windows-amd64.exe -debug
```

Web 界面操作：

1. 顶部下拉框选择并切换电源方案
2. 中部搜索框过滤设置，可勾选是否显示隐藏项
3. 对比模式下拉框：`与默认值对比` 或 `与某个方案对比`，只列出有差异的项
4. 每项可分别修改 AC / DC 值，点保存即时写回系统
5. 顶部按钮：刷新（`F5`）、导出 XML、导出脚本、导出对比 JSON、导入 XML

### powerdiff（命令行）

```powershell
# 对比当前活动方案与默认值
.\powerdiff-windows-amd64.exe

# 指定方案 GUID
.\powerdiff-windows-amd64.exe -scheme 381b4222-f694-41f0-9685-ff5bb260df2e

# 同时显示读取失败的项目，便于排查
.\powerdiff-windows-amd64.exe -all
```

输出示例：

```text
Power scheme: 平衡
Scheme GUID : 381b4222-f694-41f0-9685-ff5bb260df2e

[1] 关闭显示器后
    Subgroup     : 显示
    Subgroup GUID: 7516b95f-f776-4464-8c53-06167f40cc99
    Setting GUID : 3c0bc021-c8a8-4e07-a973-6b14cbcb2b7e
    AC: current=关闭 (0), default=从不 (0)
```

## HTTP API（powerweb 后端）

仅监听回环地址，外部无法调用。`{guid}` 均为小写 GUID 字符串，`scheme` 参数可传具体 GUID 或 `active`。

| 方法 | 路径 | 说明 |
| ---- | ---- | ---- |
| GET | `/api/health` | 健康检查，返回平台信息 |
| GET | `/api/schemes` | 列出全部电源方案 |
| GET | `/api/schemes/active` | 当前活动方案 |
| POST | `/api/schemes/{guid}/activate` | 切换活动方案 |
| GET | `/api/overlays` | 列出 overlay 方案 |
| POST | `/api/overlays/{guid}/activate` | 切换 overlay |
| GET | `/api/settings?scheme={scheme}` | 列出某方案全部设置 |
| PUT | `/api/settings/{guid}` | 修改 AC/DC 值，body：`{scheme, subgroup, ac?, dc?}` |
| PUT | `/api/settings/{guid}/hidden` | 显示/隐藏，body：`{subgroup, hidden}` |
| POST | `/api/settings/{guid}/possible-values` | 新增 Possible Value（仅异构阈值项），body：`{subgroup, index, rawHex}` |
| GET | `/api/diff/{scheme}[?compare={other}\|default][&all=1]` | 与默认值或另一方案对比 |
| GET | `/api/export?scheme={scheme}` | 导出 XML（下载文件） |
| POST | `/api/import` | 导入 XML（body 为 XML，`Content-Type: application/xml`） |
| GET | `/api/script?scheme={scheme}` | 导出 `powercfg` 批处理脚本 |

## 从源码构建

要求 Go 1.25+（见 `go.mod`）。Linux / macOS 上可交叉编译出 Windows 可执行文件：

```bash
# Web 工具
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o dist/powerweb-windows-amd64.exe ./cmd/powerweb
GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o dist/powerweb-windows-arm64.exe ./cmd/powerweb

# 命令行工具
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o dist/powerdiff-windows-amd64.exe .
GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -o dist/powerdiff-windows-arm64.exe .

# 代码检查
go vet ./...
```

Windows 本机直接 `go build ./...` 即可。前端源码在 `internal/server/static/`（`index.html` + `css/styles.css` + `js/app.js`），
会被 `go:embed` 打进 `powerweb` 二进制，无需额外部署。

## 发布流程

`.github/workflows/release.yml` 为手动触发：

1. 打开仓库 Actions → Release → Run workflow
2. 填写 `tag`（如 `v0.1.0`，必须 `v` 开头且符合 semver，workflow 会校验格式并拒绝已存在的 tag）
3. 可选填标题、勾选预发布 / 草稿
4. Workflow 自动执行：`go vet` → 交叉编译 4 个二进制 → 生成 `SHA256SUMS.txt` →
   `actions/attest` 生成 SLSA build provenance → 创建 GitHub Release 并上传资产

所用 Action 均为当前最新主版本：`actions/checkout@v7`、`actions/setup-go@v7`、
`actions/attest@v4`、`softprops/action-gh-release@v3`。

## 系统要求与注意事项

- 系统：Windows 10 / 11（x64 或 ARM64），部分旧 API 在老系统上可能不存在
- 权限：读取仅需普通用户；**修改设置、切换方案、导入、改隐藏属性需要管理员权限**，否则会返回系统错误码（`writeError` 的 `code` 字段即 Win32 错误码）
- 笔记本 / 台式机差异：`HasDC`（是否有电池）由 `PowerDeterminePlatformRole` 决定，台式机无 DC 相关值属正常
- 注册表写入：切换隐藏状态、新增 Possible Value 会写系统电源配置，导入 XML 会批量覆盖同名设置，操作前建议先导出备份
- `powerweb` 默认绑定 `127.0.0.1`，不要改为 `0.0.0.0` 对外暴露——电源写接口无鉴权，仅靠回环隔离保护

## 技术栈

- 后端：Go 标准库（`net/http`、`embed`、`encoding/xml`）+ `golang.org/x/sys`（注册表）+ `syscall` 直调 `powrprof.dll`
- 前端：无框架，原生 HTML / CSS / JS 单页
- CI：GitHub Actions（手动发布 + Sigstore attestation）

## 仓库结构

```text
.github/workflows/release.yml   手动发布到 Releases（含 attest）
cmd/powerweb/main.go            Web 工具入口（监听、自动打开浏览器、优雅退出）
powerdiff_windows.go            CLI 工具入口（仅 Windows 编译）
internal/power/                 电源 API 封装：GUID、枚举、读写、对比、导入导出
internal/server/                HTTP 服务：路由、API 处理、localhost 校验
internal/server/static/         前端页面（会被 embed 打包）
```
