package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"localserver-go/internal/fileshare"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	qrcode "github.com/skip2/go-qrcode"
)

// Status 是控制面板展示的服务状态。
type Status struct {
	Running   bool     `json:"running"`
	Port      int      `json:"port"`
	ShareDir  string   `json:"shareDir"`
	URLs      []string `json:"urls"`
	AutoStart bool     `json:"autoStart"`
	Error     string   `json:"error"`
}

// App 持有共享服务、持久化设置与（仅在主线程上访问的）界面状态。
type App struct {
	mu       sync.Mutex
	srv      *fileshare.Server
	cfg      Config
	startErr string

	win *mygo.Window

	// 设置表单，绑定到控件；只在主线程上读写。
	draftPort      string
	draftShareDir  string
	draftAutoStart bool

	// 界面状态，只在主线程上读写。
	qrOpen  bool
	qrURL   string
	qrImage ui.ImageSource
	notice  string
}

// NewApp 创建应用实例。
func NewApp() *App {
	cfg := loadConfig()
	return &App{
		srv:            fileshare.New(),
		cfg:            cfg,
		draftPort:      strconv.Itoa(cfg.Port),
		draftShareDir:  cfg.ShareDir,
		draftAutoStart: cfg.AutoStart,
	}
}

// applyTheme 在 App.Run 之前应用保存的外观。
func (a *App) applyTheme() {
	mygo.Theme.SetSource(themeSource(a.themeName()))
}

func (a *App) themeName() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cfg.Theme == "" {
		return "dark"
	}
	return a.cfg.Theme
}

func themeSource(name string) mygo.ThemeSource {
	switch name {
	case "light":
		return mygo.ThemeLight
	case "system":
		return mygo.ThemeSystem
	default:
		return mygo.ThemeDark
	}
}

// isDark 决定使用哪套配色：手动设置优先，"system" 跟随系统。
func (a *App) isDark() bool {
	switch a.themeName() {
	case "light":
		return false
	case "dark":
		return true
	default:
		return mygo.Theme.IsDark()
	}
}

// toggleTheme 在深色/浅色之间切换并记住选择。
func (a *App) toggleTheme() {
	a.mu.Lock()
	next := "light"
	if a.cfg.Theme == "light" {
		next = "dark"
	}
	a.cfg.Theme = next
	if err := saveConfig(a.cfg); err != nil {
		log.Printf("localserver: 保存主题失败: %v", err)
	}
	a.mu.Unlock()
	mygo.Theme.SetSource(themeSource(next))
}

// GetStatus 返回当前服务状态。
func (a *App) GetStatus() Status {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.statusLocked()
}

func (a *App) statusLocked() Status {
	st := a.srv.Status()
	errMsg := a.startErr
	if errMsg == "" {
		errMsg = st.Error
	}
	urls := st.URLs
	if urls == nil {
		urls = []string{}
	}
	return Status{
		Running:   st.Running,
		Port:      a.cfg.Port,
		ShareDir:  a.cfg.ShareDir,
		URLs:      urls,
		AutoStart: a.cfg.AutoStart,
		Error:     errMsg,
	}
}

// Start 使用当前设置启动共享服务。
func (a *App) Start() (Status, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.startLocked()
}

func (a *App) startLocked() (Status, error) {
	if err := a.srv.Start(a.cfg.ShareDir, a.cfg.Port); err != nil {
		a.startErr = err.Error()
		return a.statusLocked(), err
	}
	a.startErr = ""
	return a.statusLocked(), nil
}

// Stop 停止共享服务。
func (a *App) Stop() (Status, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if err := a.srv.Stop(); err != nil && !errors.Is(err, fileshare.ErrNotRunning) {
		a.startErr = err.Error()
		return a.statusLocked(), err
	}
	a.startErr = ""
	return a.statusLocked(), nil
}

// UpdateSettings 保存共享目录、端口和自动启动设置。
// 如果服务正在运行，会使用新设置自动重启。
func (a *App) UpdateSettings(port int, shareDir string, autoStart bool) (Status, error) {
	if port < 1 || port > 65535 {
		return a.GetStatus(), fmt.Errorf("端口需在 1-65535 之间")
	}

	shareDir = strings.TrimSpace(shareDir)
	if shareDir == "" {
		return a.GetStatus(), errors.New("请选择共享目录")
	}
	absDir, err := filepath.Abs(shareDir)
	if err != nil {
		return a.GetStatus(), fmt.Errorf("共享目录无效: %w", err)
	}
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		return a.GetStatus(), fmt.Errorf("无法创建共享目录: %w", err)
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	wasRunning := a.srv.Status().Running
	if wasRunning {
		_ = a.srv.Stop()
	}

	a.cfg = Config{
		ShareDir:  absDir,
		Port:      port,
		AutoStart: autoStart,
		Theme:     a.cfg.Theme,
	}
	a.startErr = ""
	if err := saveConfig(a.cfg); err != nil {
		log.Printf("localserver: 保存设置失败: %v", err)
	}

	if wasRunning {
		if err := a.srv.Start(a.cfg.ShareDir, a.cfg.Port); err != nil {
			a.startErr = err.Error()
			return a.statusLocked(), err
		}
	}
	return a.statusLocked(), nil
}

// shutdown 在应用退出时停止服务。
func (a *App) shutdown() {
	a.mu.Lock()
	defer a.mu.Unlock()
	_ = a.srv.Stop()
}

func (a *App) shareDir() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cfg.ShareDir
}

func (a *App) draftPortNumber() (int, error) {
	return strconv.Atoi(strings.TrimSpace(a.draftPort))
}

func (a *App) dirty() bool {
	st := a.GetStatus()
	port, err := a.draftPortNumber()
	if err != nil || port != st.Port {
		return true
	}
	return strings.TrimSpace(a.draftShareDir) != st.ShareDir || a.draftAutoStart != st.AutoStart
}

func (a *App) syncDraft(st Status) {
	a.draftPort = strconv.Itoa(st.Port)
	a.draftShareDir = st.ShareDir
	a.draftAutoStart = st.AutoStart
}

// openDirPicker 打开系统目录选择框；对话框会阻塞，所以放在 goroutine 中。
func (a *App) openDirPicker() {
	win, current := a.win, a.shareDir()
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:            win,
			Title:             "选择共享目录",
			DefaultPath:       current,
			Directory:         true,
			CreateDirectories: true,
		})
		if err != nil || len(paths) == 0 {
			return
		}
		win.Update(func() {
			a.draftShareDir = paths[0]
			port, err := a.draftPortNumber()
			if err != nil {
				a.notice = "端口需在 1-65535 之间"
				return
			}
			if _, err := a.UpdateSettings(port, a.draftShareDir, a.draftAutoStart); err != nil {
				a.notice = err.Error()
				return
			}
			a.notice = "共享目录已更新"
		})
	}()
}

// openShareDir 在系统文件管理器中打开共享目录。
func (a *App) openShareDir() {
	dir := a.shareDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		a.notice = err.Error()
		return
	}
	if err := mygo.Shell.OpenPath(dir); err != nil {
		a.notice = err.Error()
	}
}

// openURL 用系统默认浏览器打开链接。
func (a *App) openURL(rawURL string) {
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		a.notice = "无效的链接"
		return
	}
	if err := mygo.Shell.OpenExternal(rawURL); err != nil {
		a.notice = err.Error()
	}
}

// copyText 把文本写入系统剪贴板。
func (a *App) copyText(text string) {
	mygo.Clipboard.WriteText(text)
	a.notice = "链接已复制到剪贴板"
}

// showQR 生成访问地址二维码并在模态框中显示。
func (a *App) showQR(rawURL string) {
	qr, err := qrcode.New(rawURL, qrcode.Medium)
	if err != nil {
		a.notice = "二维码生成失败: " + err.Error()
		return
	}
	a.qrImage = ui.NewBitmap(qr.Image(256)).Pixelated()
	a.qrURL = rawURL
	a.qrOpen = true
}

type urlItem struct {
	URL   string
	Label string
}

func (a *App) allURLs(st Status) []urlItem {
	items := []urlItem{{URL: fmt.Sprintf("http://localhost:%d", st.Port), Label: "本机"}}
	for _, u := range st.URLs {
		items = append(items, urlItem{URL: u, Label: "局域网"})
	}
	return items
}

// statusSignature 用于后台轮询：只有状态变化时才请求重绘。
func (a *App) statusSignature() string {
	st := a.GetStatus()
	return fmt.Sprintf("%t|%d|%s|%s|%t|%s",
		st.Running, st.Port, st.ShareDir, strings.Join(st.URLs, ","), st.AutoStart, st.Error)
}

// watchStatus 每 4 秒检查一次服务状态（例如服务线程异常退出）。
func (a *App) watchStatus(win *mygo.Window) {
	ticker := time.NewTicker(4 * time.Second)
	defer ticker.Stop()
	var last string
	for range ticker.C {
		sig := a.statusSignature()
		mygo.RunOnMain(func() {
			if sig != last {
				last = sig
				win.Invalidate()
			}
		})
	}
}

// view 用 MyGo 原生 UI 还原原版 Wails/Vue 控制面板的视觉设计。
func (a *App) view(c *ui.Context) {
	pal := darkPalette()
	dark := a.isDark()
	if !dark {
		pal = lightPalette()
	}
	applyPalette(c, pal, dark)
	st := a.GetStatus()

	// 后台任务留下的提示，在下一次构建时显示。
	a.flushNotice(c)

	root := ui.Scroll(c).Fill()
	root.Draw(func(p *ui.Painter, r ui.Rect) { paintBackground(p, r, pal) })
	root.Children(func() {
		ui.Column(c).FillHeight().Padding(24, 28, 14).Gap(16).Children(func() {
			a.header(c, pal, st)
			if st.Error != "" {
				alert := ui.Box(c).Padding(10, 14).Radius(10).Background(pal.alertBg).Border(1, pal.alertLine)
				alert.Children(func() { ui.Text(c, st.Error).FontSize(13).TextColor(pal.alertText).Wrap() })
			}
			ui.Row(c).Gap(16).AlignItems(ui.Stretch).Children(func() {
				a.settingsCard(c, pal)
				a.urlsCard(c, pal, st)
			})
			ui.Spacer(c)
			foot := "文件只在本机与局域网之间传输，不经过任何服务器"
			if st.Running {
				foot = "正在共享 " + st.ShareDir
			}
			ui.Text(c, foot).FontSize(11).TextColor(pal.faint).SingleLine()
		})
	})

	a.qrModal(c, pal)
}

func (a *App) header(c *ui.Context, pal palette, st Status) {
	ui.Row(c).Gap(14).AlignItems(ui.Center).Children(func() {
		mark := ui.Box(c).Size(42, 42).Radius(12)
		mark.Draw(func(p *ui.Painter, r ui.Rect) {
			p.FillGradient(r, ui.LinearGradient{From: pal.primaryA, To: pal.primaryB, Angle: 180}, 12)
		})
		mark.Children(func() { ui.Icon(c, iconDownload).FontSize(22).TextColor(pal.primaryInk) })

		ui.Column(c).Gap(2).Children(func() {
			ui.Text(c, "LocalSever").FontSize(18).Bold()
			ui.Text(c, "局域网文件传输").FontSize(12).TextColor(pal.muted)
		})
		ui.Spacer(c)

		// 和原版一致：深色时显示太阳（点击切到浅色），浅色时显示月亮。
		tip, icon := "切换到浅色模式", iconSun
		if !a.isDark() {
			tip, icon = "切换到深色模式", iconMoon
		}
		themeBtn := ui.ButtonBase(c).Label(tip).Tooltip(tip).Size(32, 32).Radius(16)
		themeBg, themeBorder := pal.fill, pal.border
		if themeBtn.Hovered() {
			themeBg, themeBorder = pal.fillHover, pal.borderStrong
		}
		themeBtn.Background(themeBg).Border(1, themeBorder)
		themeBtn.Children(func() { ui.Icon(c, icon).FontSize(16).TextColor(pal.muted) })
		if themeBtn.Clicked() {
			a.toggleTheme()
		}

		statusPill(c, pal, st.Running)

		if st.Running {
			button(c, pal, "停止服务", btnDanger, false, func() {
				next, err := a.Stop()
				if err != nil {
					a.toastError(c, err)
					return
				}
				a.syncDraft(next)
				c.Toast("共享服务已停止")
			})
		} else {
			button(c, pal, "启动服务", btnPrimary, false, func() {
				next, err := a.Start()
				if err != nil {
					a.toastError(c, err)
					return
				}
				a.syncDraft(next)
				c.Toast("共享服务已开启")
			})
		}
	})
}

func (a *App) settingsCard(c *ui.Context, pal palette) {
	card(c, pal, func() {
		ui.Text(c, "共享设置").FontSize(14).Bold()

		ui.Column(c).Gap(6).Children(func() {
			ui.Text(c, "共享目录").FontSize(12).TextColor(pal.muted)
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				display := a.draftShareDir
				if len(display) > 46 {
					display = "…" + display[len(display)-45:]
				}
				if display == "" {
					display = "尚未选择目录"
				}
				path := ui.Box(c).Grow(1).Height(34).AlignItems(ui.Center).Padding(0, 11).Radius(9).Background(pal.field).Border(1, pal.border)
				path.Children(func() { ui.Text(c, display).FontSize(12.5).TextColor(pal.text).SingleLine().Selectable() })
				button(c, pal, "更换", btnNormal, false, a.openDirPicker)
			})
			ui.Text(c, "客户端可以浏览、上传、下载和删除此目录中的文件。").FontSize(11.5).TextColor(pal.muted).Wrap()
		})

		ui.Column(c).Gap(6).Children(func() {
			ui.Text(c, "服务端口").FontSize(12).TextColor(pal.muted)
			in := ui.TextInputBase(c, &a.draftPort).Label("服务端口").Width(140).Height(34).Padding(0, 11).Radius(9)
			in.Draw(func(p *ui.Painter, r ui.Rect) {
				p.Fill(r, pal.field, 9)
				p.Stroke(r, pal.border, 9, 1)
			})
			ui.Text(c, "修改端口或目录后点击「保存设置」，运行中的服务会自动重启。").FontSize(11.5).TextColor(pal.muted).Wrap()
		})

		ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, "启动时自动共享").FontSize(12).TextColor(pal.muted)
				ui.Text(c, "应用启动后立即开启，无需手动点击。").FontSize(11.5).TextColor(pal.muted)
			})
			ui.Spacer(c)
			autoStartSwitch(c, pal, &a.draftAutoStart)
		})

		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			button(c, pal, "打开目录", btnGhost, false, func() {
				a.openShareDir()
				a.flushNotice(c)
			})
			button(c, pal, "保存设置", btnPrimary, !a.dirty(), func() {
				port, err := a.draftPortNumber()
				if err != nil || port < 1 || port > 65535 {
					a.toastError(c, fmt.Errorf("端口需在 1-65535 之间"))
					return
				}
				next, err := a.UpdateSettings(port, a.draftShareDir, a.draftAutoStart)
				if err != nil {
					a.toastError(c, err)
					return
				}
				a.syncDraft(next)
				c.Toast("设置已保存")
			})
		})
	})
}

func (a *App) urlsCard(c *ui.Context, pal palette, st Status) {
	card(c, pal, func() {
		ui.Text(c, "访问地址").FontSize(14).Bold()
		if !st.Running {
			ui.Column(c).Gap(6).AlignItems(ui.Center).Children(func() {
				idle := ui.Box(c).Size(52, 52).Radius(16).Border(1, pal.borderStrong).BorderStyle(ui.BorderDashed).Background(pal.fillSofter)
				idle.Children(func() {
					ui.Text(c, "⇅").FontSize(24).TextColor(pal.muted)
				})
				ui.Text(c, "服务未运行").FontSize(13).Bold()
				ui.Text(c, "点击右上角「启动服务」开始共享文件").FontSize(11.5).TextColor(pal.muted)
			})
		} else {
			ui.Text(c, "同一 Wi-Fi 下的手机、平板或电脑，打开以下任一地址即可传输文件；手机也可以直接扫码打开。").FontSize(11.5).TextColor(pal.muted).Wrap()
			ui.Column(c).Gap(8).Children(func() {
				for _, item := range a.allURLs(st) {
					url, label := item.URL, item.Label
					row := ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(9, 10).Radius(10).Background(pal.fieldSoft).Border(1, pal.border)
					row.Children(func() {
						tag := ui.Box(c).AlignItems(ui.Center).Padding(1, 7).Radius(99).Background(pal.accentSoft)
						tag.Children(func() { ui.Text(c, label).FontSize(10.5).TextColor(pal.accent) })
						ui.Text(c, url).Grow(1).SingleLine().Font("monospace").FontSize(12.5).TextColor(pal.text).Selectable()
						if label == "局域网" {
							button(c, pal, "二维码", btnTinyAccent, false, func() { a.showQR(url) })
						}
						button(c, pal, "复制", btnTiny, false, func() {
							a.copyText(url)
							a.flushNotice(c)
						})
					})
				}
			})
			ui.Row(c).Gap(8).Children(func() {
				button(c, pal, "在浏览器打开", btnGhost, false, func() {
					a.openURL(fmt.Sprintf("http://localhost:%d", st.Port))
					a.flushNotice(c)
				})
				if len(st.URLs) > 0 {
					button(c, pal, "显示二维码", btnGhost, false, func() { a.showQR(st.URLs[0]) })
				}
			})
		}

		ui.Box(c).Height(1).Margin(18, 0, 0, 0).Background(pal.border)
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, "提示").FontSize(11).Bold().TextColor(pal.faint)
			ui.Text(c, "• 首次开启时 Windows 可能弹出防火墙提示，请选择「允许访问」。").FontSize(11.5).TextColor(pal.muted).Wrap()
			ui.Text(c, "• 服务对局域网内所有人可见，请仅在可信网络中使用。").FontSize(11.5).TextColor(pal.muted).Wrap()
		})
	})
}

func (a *App) qrModal(c *ui.Context, pal palette) {
	ui.DialogBase(c, &a.qrOpen, func(backdrop, panel ui.Element) {
		backdrop.Background(pal.overlay)
		panel.Width(360).Padding(20, 20, 16).Radius(20).Background(pal.card).
			Border(1, pal.borderStrong).Shadow(0, 30, 70, 0, pal.shadowLg)
		panel.Children(func() {
			ui.Text(c, "扫码访问").FontSize(15).Bold()
			ui.Text(c, "手机与电脑连接同一个 Wi-Fi，扫码即可打开。").FontSize(11.5).TextColor(pal.muted)
			if a.qrImage != nil {
				box := ui.Box(c).Size(248, 248).Padding(12).Radius(14).Background(ui.RGB(255, 255, 255))
				box.Children(func() { ui.Image(c, a.qrImage).Size(224, 224) })
			}
			ui.Text(c, a.qrURL).FontSize(12).TextColor(pal.muted).Font("monospace").Selectable().Wrap()
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				button(c, pal, "复制链接", btnGhost, false, func() {
					a.copyText(a.qrURL)
					a.flushNotice(c)
				})
				button(c, pal, "完成", btnPrimary, false, func() { a.qrOpen = false })
			})
		})
	})
}

// toastError 显示错误信息。
func (a *App) toastError(c *ui.Context, err error) {
	c.AddToast(ui.Toast{Title: err.Error(), Type: "error"})
}

// flushNotice 把后台任务留下的提示显示出来。
func (a *App) flushNotice(c *ui.Context) {
	if a.notice != "" {
		msg := a.notice
		a.notice = ""
		c.Toast(msg)
	}
}
