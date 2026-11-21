package main

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestPanelView 用 ui.NewTester 渲染原生控制面板，确认基本结构可构建。
func TestPanelView(t *testing.T) {
	app := NewApp()
	tt := ui.NewTester(app.view, 1024, 768)

	for _, text := range []string{"LocalSever", "局域网文件传输", "共享设置", "访问地址", "启动服务"} {
		if !tt.HasText(text) {
			t.Errorf("面板中找不到 %q，实际文本: %q", text, tt.Texts())
		}
	}
}

// TestRenderPreview 把面板渲染成 PNG，便于人工核对视觉效果。
// 设置 RENDER_UI=1 时运行，输出到 build/ui-preview.png。
func TestRenderPreview(t *testing.T) {
	if os.Getenv("RENDER_UI") == "" {
		t.Skip("set RENDER_UI=1 to render a preview")
	}
	app := NewApp()
	if theme := os.Getenv("RENDER_THEME"); theme != "" {
		app.cfg.Theme = theme
	}
	if os.Getenv("RENDER_RUNNING") != "" {
		app.cfg.Port = 18080
		if _, err := app.Start(); err != nil {
			t.Fatal(err)
		}
		defer app.Stop()
	}
	if os.Getenv("RENDER_QR") != "" {
		app.showQR("http://192.168.1.20:8080")
	}
	tt := ui.NewTester(app.view, 1024, 768)
	if err := os.MkdirAll("build", 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create("build/ui-preview.png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
	t.Log("wrote build/ui-preview.png")
}
