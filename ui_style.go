package main

import "github.com/egoist/mygo/ui"

// palette 对应原版前端 style.css 里的 CSS 变量，按深色/浅色各一套。
type palette struct {
	bg, glow1, glow2                       ui.Color
	card, cardGradA, cardGradB             ui.Color
	surface, shadow, shadowLg              ui.Color
	text, muted, faint                     ui.Color
	fill, fillHover, fillSoft, fillSofter  ui.Color
	field, fieldSoft                       ui.Color
	border, borderStrong                   ui.Color
	accent, accentSoft, accentSoftStrong   ui.Color
	accentFocus, accentLine                ui.Color
	primaryA, primaryB, primaryInk         ui.Color
	danger, dangerText, dangerBg, dangerLine ui.Color
	alertBg, alertLine, alertText          ui.Color
	overlay                                ui.Color
	toastBg, toastText, toastLine          ui.Color
	toastErrBg, toastErrText, toastErrLine ui.Color
	switchBg, switchKnob                   ui.Color
}

func darkPalette() palette {
	return palette{
		bg: ui.Hex("#131b28"), glow1: ui.Hex("#20344cd9"), glow2: ui.Hex("#1b3340cc"),
		card: ui.Hex("#1d2a3c"), cardGradA: ui.Hex("#ffffff0d"), cardGradB: ui.Hex("#ffffff04"),
		surface: ui.Hex("#263750"), shadow: ui.Hex("#0000002e"), shadowLg: ui.Hex("#00000080"),
		text: ui.Hex("#e8eef8"), muted: ui.Hex("#8b9bb4"), faint: ui.Hex("#ffffff4d"),
		fill: ui.Hex("#ffffff0d"), fillHover: ui.Hex("#ffffff17"), fillSoft: ui.Hex("#ffffff0f"), fillSofter: ui.Hex("#ffffff05"),
		field: ui.Hex("#080e1673"), fieldSoft: ui.Hex("#080e1666"),
		border: ui.Hex("#ffffff14"), borderStrong: ui.Hex("#ffffff29"),
		accent: ui.Hex("#3ccfbd"), accentSoft: ui.Hex("#3ccfbd1a"), accentSoftStrong: ui.Hex("#3ccfbd29"),
		accentFocus: ui.Hex("#3ccfbd1f"), accentLine: ui.Hex("#3ccfbd73"),
		primaryA: ui.Hex("#40d6c3"), primaryB: ui.Hex("#28b3a3"), primaryInk: ui.Hex("#04241f"),
		danger: ui.Hex("#f16a6a"), dangerText: ui.Hex("#ffb9b9"), dangerBg: ui.Hex("#f16a6a1f"), dangerLine: ui.Hex("#f16a6a59"),
		alertBg: ui.Hex("#f16a6a1a"), alertLine: ui.Hex("#f16a6a47"), alertText: ui.Hex("#ffd7d7"),
		overlay: ui.Hex("#060b12b3"),
		toastBg: ui.Hex("#263750"), toastText: ui.Hex("#e8eef8"), toastLine: ui.Hex("#ffffff29"),
		toastErrBg: ui.Hex("#45262c"), toastErrText: ui.Hex("#ffd7d7"), toastErrLine: ui.Hex("#f16a6a66"),
		switchBg: ui.Hex("#ffffff1f"), switchKnob: ui.Hex("#cbd6e6"),
	}
}

func lightPalette() palette {
	return palette{
		bg: ui.Hex("#eef2f8"), glow1: ui.Hex("#c6dbf2e6"), glow2: ui.Hex("#c8ebe3d9"),
		card: ui.Hex("#ffffff"), cardGradA: ui.Hex("#ffffff"), cardGradB: ui.Hex("#f7fafd"),
		surface: ui.Hex("#ffffff"), shadow: ui.Hex("#172b4d1a"), shadowLg: ui.Hex("#172b4d47"),
		text: ui.Hex("#16263a"), muted: ui.Hex("#5f7189"), faint: ui.Hex("#16263a73"),
		fill: ui.Hex("#1428460b"), fillHover: ui.Hex("#14284617"), fillSoft: ui.Hex("#1428460f"), fillSofter: ui.Hex("#14284606"),
		field: ui.Hex("#1428460b"), fieldSoft: ui.Hex("#1428460a"),
		border: ui.Hex("#1428461a"), borderStrong: ui.Hex("#14284633"),
		accent: ui.Hex("#12897c"), accentSoft: ui.Hex("#12897c1a"), accentSoftStrong: ui.Hex("#12897c29"),
		accentFocus: ui.Hex("#12897c26"), accentLine: ui.Hex("#12897c66"),
		primaryA: ui.Hex("#109686"), primaryB: ui.Hex("#0b766a"), primaryInk: ui.Hex("#ffffff"),
		danger: ui.Hex("#d64545"), dangerText: ui.Hex("#a92f2f"), dangerBg: ui.Hex("#d6454514"), dangerLine: ui.Hex("#d6454547"),
		alertBg: ui.Hex("#d6454514"), alertLine: ui.Hex("#d6454540"), alertText: ui.Hex("#9c2e2e"),
		overlay: ui.Hex("#121e3061"),
		toastBg: ui.Hex("#ffffff"), toastText: ui.Hex("#16263a"), toastLine: ui.Hex("#14284624"),
		toastErrBg: ui.Hex("#fdf0f0"), toastErrText: ui.Hex("#a92f2f"), toastErrLine: ui.Hex("#d6454559"),
		switchBg: ui.Hex("#1428461f"), switchKnob: ui.Hex("#ffffff"),
	}
}

// applyPalette 让 MyGo 内置控件（输入框、Toast、焦点环等）也用原版的配色。
func applyPalette(c *ui.Context, p palette, dark bool) {
	th := ui.LightTheme()
	if dark {
		th = ui.DarkTheme()
	}
	th.Background = p.bg
	th.Surface = p.surface
	th.SurfaceHover = p.fillHover
	th.SurfacePressed = p.fillSoft
	th.Border = p.border
	th.Text = p.text
	th.TextMuted = p.muted
	th.Accent = p.accent
	th.AccentHover = p.accent
	th.AccentPressed = p.accent
	th.AccentText = p.primaryInk
	th.Danger = p.danger
	th.Success = p.accent
	th.Selection = p.accentSoft
	th.Focus = p.accentLine
	th.Inverse = p.toastBg
	th.InverseText = p.toastText
	th.Scrollbar = p.borderStrong
	th.Radius = 9
	th.FontSize = 13
	c.SetTheme(th)
}

var (
	iconDownload = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 4v11m0 0l-4-4m4 4l4-4"/><path d="M5 19h14"/></svg>`))
	iconSun      = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="4"/><path d="M12 2v2m0 16v2M4.9 4.9l1.4 1.4m11.4 11.4l1.4 1.4M2 12h2m16 0h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4"/></svg>`))
	iconMoon     = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 14.5A8.5 8.5 0 1 1 9.5 4a7 7 0 0 0 10.5 10.5z"/></svg>`))
)

// 背景：原版 body 上的两处径向光晕。
func paintBackground(p *ui.Painter, r ui.Rect, pal palette) {
	p.Fill(r, pal.bg, 0)
	p.FillRadialGradient(r, ui.RadialGradient{
		Stops:  []ui.GradientStop{{Color: pal.glow1, At: 0}, {Color: ui.Transparent, At: 1}},
		Center: ui.Point{X: 0.12, Y: -0.1},
	}, 0)
	p.FillRadialGradient(r, ui.RadialGradient{
		Stops:  []ui.GradientStop{{Color: pal.glow2, At: 0}, {Color: ui.Transparent, At: 1}},
		Center: ui.Point{X: 1.1, Y: 0},
	}, 0)
}

// card 画出原版的圆角渐变卡片和阴影。
func card(c *ui.Context, pal palette, fn func()) {
	box := ui.Box(c).Grow(1).Padding(20).Gap(16).Radius(16).Border(1, pal.border).
		Shadow(0, 18, 40, 0, pal.shadow)
	box.Draw(func(p *ui.Painter, r ui.Rect) {
		p.Fill(r, pal.card, 16)
		p.FillGradient(r, ui.LinearGradient{From: pal.cardGradA, To: pal.cardGradB, Angle: 180}, 16)
	})
	box.Children(fn)
}

type buttonKind uint8

const (
	btnNormal buttonKind = iota
	btnPrimary
	btnDanger
	btnGhost
	btnTiny
	btnTinyAccent
)

// button 按原版 .btn / .btn.primary / .btn.danger / .tiny 的样式构建按钮。
func button(c *ui.Context, pal palette, label string, kind buttonKind, disabled bool, fn func()) {
	h, padX, rad, size := float32(34), float32(14), float32(9), float32(13)
	bg, border, fg := pal.fill, pal.border, pal.text
	switch kind {
	case btnPrimary:
		fg, bg, border = pal.primaryInk, pal.primaryA, ui.Transparent
	case btnDanger:
		bg, border, fg = pal.dangerBg, pal.dangerLine, pal.dangerText
	case btnGhost:
		bg = ui.Transparent
	case btnTiny:
		h, padX, rad, size = 24, 9, 6, 11.5
	case btnTinyAccent:
		h, padX, rad, size = 24, 9, 6, 11.5
		bg, border, fg = pal.accentSoft, pal.accentLine, pal.accent
	}
	b := ui.ButtonBase(c).Label(label).Disabled(disabled).
		Height(h).Padding(0, padX).Radius(rad).TextColor(fg)
	if !disabled && b.Hovered() {
		switch kind {
		case btnNormal:
			bg, border = pal.fillHover, pal.borderStrong
		case btnGhost:
			bg = pal.fillSoft
		case btnDanger:
			bg, border = pal.dangerBg, pal.dangerLine
		}
	}
	if kind == btnPrimary {
		b.Shadow(0, 4, 14, 0, pal.accentFocus)
		b.Draw(func(p *ui.Painter, r ui.Rect) {
			p.FillGradient(r, ui.LinearGradient{From: pal.primaryA, To: pal.primaryB, Angle: 180}, rad)
		})
	} else {
		b.Background(bg)
		if border.A != 0 {
			b.Border(1, border)
		}
	}
	b.Children(func() {
		txt := ui.Text(c, label).FontSize(size).SingleLine()
		if kind == btnPrimary {
			txt.Bold()
		}
	})
	if !disabled && b.Clicked() && fn != nil {
		fn()
	}
}

// statusPill 对应原版带发光圆点的状态标签。
func statusPill(c *ui.Context, pal palette, running bool) {
	fg, bg, label := pal.muted, pal.fillSoft, "已停止"
	if running {
		fg, bg, label = pal.accent, pal.accentSoft, "运行中"
	}
	pill := ui.Row(c).Gap(6).AlignItems(ui.Center).Height(26).Padding(0, 11).Radius(13).Background(bg)
	pill.Children(func() {
		ui.Box(c).Size(6, 6).Radius(3).Background(fg).Shadow(0, 0, 8, 0, fg)
		ui.Text(c, label).FontSize(12).TextColor(fg)
	})
}

// autoStartSwitch 按原版 .switch / .switch.on 绘制开关。
func autoStartSwitch(c *ui.Context, pal palette, on *bool) {
	sw := ui.SwitchBase(c, on).Label("启动时自动共享").Size(42, 24).Radius(12)
	bg, border, knob, left := pal.switchBg, pal.border, pal.switchKnob, float32(3)
	if *on {
		bg, border, knob, left = pal.accentSoftStrong, pal.accentLine, pal.accent, 21
	}
	sw.Draw(func(p *ui.Painter, r ui.Rect) {
		p.Fill(r, bg, 12)
		p.Stroke(r, border, 12, 1)
	})
	sw.Children(func() {
		ui.Box(c).Size(18, 18).Radius(9).Margin(3, 0, 0, left).Background(knob).
			Shadow(0, 1, 3, 0, ui.RGBA(0, 0, 0, 0.15))
	})
}
