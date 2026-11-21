# LocalSever（MyGo 原生界面版）

局域网文件传输工具。桌面控制面板使用 **MyGo 原生 UI（`ui` 包）** 绘制，不使用 webview；
控制面板只负责配置和启停服务，局域网设备用浏览器访问服务内置的网页即可传输文件。

## 功能

- 一键开启 / 停止文件共享，可设置端口（默认 `8080`）
- 自定义共享目录，默认 `~/LocalSeverShare`
- 客户端网页支持：
  - 浏览目录、面包屑导航
  - 上传文件（多选、拖拽）、上传整个文件夹（保留目录结构）
  - 下载文件（支持断点续传 / Range）
  - 图片、视频、音频、PDF 在线预览
  - 新建文件夹、删除文件或文件夹
  - 移动端自适应布局
- 显示所有可用的局域网访问地址，一键复制、在浏览器打开
- 每个局域网地址可生成二维码，手机扫码直接打开
- 深色 / 浅色主题切换并记住选择（跟随系统也可在 `config.json` 中设置 `system`）
- 设置持久化到 `%AppData%\LocalSever\config.json`（沿用原 Wails 版路径）
- 退出应用时自动停止服务

## 目录结构

```
main.go                          # 入口：MyGo 窗口与生命周期
app.go                           # 原生 UI 面板（ui.View）与服务控制逻辑
config.go                        # 配置文件读写
internal/fileshare/server.go     # 文件共享 HTTP 服务与接口
internal/fileshare/web/index.html# 客户端网页（内嵌进二进制）
mygo.json                        # MyGo 原生应用配置（无前端/webview）
```

## 开发

CLI 作为 Go tool 固定在 `go.mod` 中，无需 Bun / npm：

```bash
go tool mygo dev              # 开发运行
go tool mygo build            # 正式打包到 build/windows-amd64/（含安装包）
go tool mygo vet .            # 检查 MyGo 构建期取值与 goroutine 捕获
go test ./...
CGO_ENABLED=0 go test ./...
```

`go tool mygo build` 会生成 `LocalSever.exe` 和安装程序 `localsever-setup-0.1.0-amd64.exe`，
并把 `resources/` 下的图标等资源一起复制到输出目录。

跨平台打包（MyGo 不需要 cgo，可在 Windows 上直接交叉编译）：

```bash
go tool mygo build -platform linux/amd64          # deb / rpm / tar.gz / PKGBUILD
go tool mygo build -platform darwin/universal     # LocalSever.app（Intel + Apple Silicon）
go tool mygo build -platform windows/amd64,linux/amd64,darwin/universal
```

macOS 的 `.dmg` 和签名需要在 macOS 上执行；在其它系统上打包时 MyGo 会跳过磁盘映像，
但仍会产出可用的 `LocalSever.app`。Linux 包由于 `mygo.json` 里 `linux.webview: false`，
只依赖 GTK，不依赖 WebKitGTK。

原生桌面能力均使用 MyGo 自带 API：`mygo.Dialog`（目录选择）、`mygo.Clipboard`（复制）、
`mygo.Shell`（打开链接/文件夹）、`mygo.Theme`（主题），无需 webview，也无需额外的原生模块。

> 服务没有访问密码，局域网内的任何人都可以访问。请只在可信网络中使用。
