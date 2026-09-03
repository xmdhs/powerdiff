# powerdiff

Windows 电源计划设置查看、对比与管理工具。

通过读取 Windows `powrprof.dll` 电源 API，列出电源方案的全部设置项，
对比当前值与默认值的差异，并支持在 Web 可视化界面中直接修改、导入 / 导出。

> 仅支持 Windows。

## 下载

到 [Releases](../../releases) 页下载最新版：

| 文件 | 说明 |
| ---- | ---- |
| `powerweb-windows-amd64.exe` | Intel / AMD 64 位（大多数人选这个） |
| `powerweb-windows-arm64.exe` | ARM64（如骁龙本） |

构建来源可验证：每个 Release 的二进制都附带 Sigstore build provenance attestation：

```bash
gh attestation verify powerweb-windows-amd64.exe --owner xmdhs
```

## 快速开始

双击运行，或在终端运行（**修改设置需要以管理员身份运行**，仅查看则不需要）：

```powershell
.\powerweb-windows-amd64.exe
```

启动后自动打开浏览器（端口随机分配），按 `Ctrl+C` 停止。

常用参数：

```powershell
.\powerweb-windows-amd64.exe -addr 127.0.0.1:8080  # 指定监听地址
.\powerweb-windows-amd64.exe -no-browser            # 不自动打开浏览器
.\powerweb-windows-amd64.exe -debug                 # 调试日志
```

界面操作：顶部切换电源方案，搜索框过滤设置，对比模式只列出有差异的项，
每项可分别修改 AC（接通电源）/ DC（电池）值并保存，还支持导出 XML / 脚本、导入 XML。

## 功能特性

- 电源方案与 Overlay（电源模式滑杆）查看、切换
- 设置按子组分组展示：名称、描述、GUID、隐藏状态、数值范围 / 可选值
- 搜索过滤（`Ctrl+K` 聚焦，`Esc` 清空，`F5` 刷新）
- 修改 AC / DC 值（后端做范围与步进校验）、显示 / 隐藏设置项
- 对比：与默认值对比，或方案与方案之间对比，对比结果可导出 JSON
- 导入 / 导出：全量 XML 导出与导入、导出为 `powercfg` 批处理脚本
- `/api/*` 仅允许 localhost 访问

## HTTP API

`{guid}` 为小写 GUID，`scheme` 可传具体 GUID 或 `active`。

| 方法 | 路径 | 说明 |
| ---- | ---- | ---- |
| GET | `/api/health` | 健康检查 |
| GET | `/api/schemes` | 列出全部电源方案 |
| GET | `/api/schemes/active` | 当前活动方案 |
| POST | `/api/schemes/{guid}/activate` | 切换活动方案 |
| GET | `/api/overlays` | 列出 overlay 方案 |
| POST | `/api/overlays/{guid}/activate` | 切换 overlay |
| GET | `/api/settings?scheme={scheme}` | 列出某方案全部设置 |
| PUT | `/api/settings/{guid}` | 修改 AC/DC 值 `{scheme, subgroup, ac?, dc?}` |
| PUT | `/api/settings/{guid}/hidden` | 显示/隐藏 `{subgroup, hidden}` |
| POST | `/api/settings/{guid}/possible-values` | 新增 Possible Value（仅异构阈值项） |
| GET | `/api/diff/{scheme}[?compare={other}][&all=1]` | 与默认值或另一方案对比 |
| GET | `/api/export?scheme={scheme}` | 导出 XML |
| POST | `/api/import` | 导入 XML |
| GET | `/api/script?scheme={scheme}` | 导出 `powercfg` 脚本 |

## 从源码构建

要求 Go 1.25+：

```bash
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o dist/powerweb-windows-amd64.exe ./cmd/powerweb
GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -o dist/powerweb-windows-arm64.exe ./cmd/powerweb
```

Windows 本机直接 `go build ./...`。前端源码在 `internal/server/static/`，
会被 `go:embed` 打进二进制，无需额外部署。

## 发布流程

`.github/workflows/release.yml` 为手动触发：Actions → Release → Run workflow，
填写 `tag`（如 `v0.1.0`，自动校验格式并拒绝重复 tag），可选标题 / 预发布 / 草稿。
流程：`go vet` → 交叉编译两个二进制 → `actions/attest` 生成 provenance → 创建 Release 并上传资产。

## 注意事项

- 读取仅需普通用户；修改设置、切换方案、导入需要管理员权限
- 导入 XML 会批量覆盖同名设置，操作前建议先导出备份
- 默认绑定 `127.0.0.1`，不要改为 `0.0.0.0` 对外暴露——写接口无鉴权，仅靠回环隔离保护

## 仓库结构

```text
.github/workflows/release.yml   手动发布到 Releases（含 attest）
cmd/powerweb/main.go            Web 工具入口
powerdiff_windows.go            CLI 工具（仓库保留，不参与 Release 构建）
internal/power/                 电源 API 封装
internal/server/                HTTP 服务与前端（static/ 会被 embed 打包）
```
