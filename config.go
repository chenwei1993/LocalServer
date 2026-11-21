package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config 是持久化到用户配置目录的设置。
type Config struct {
	ShareDir  string `json:"shareDir"`
	Port      int    `json:"port"`
	AutoStart bool   `json:"autoStart"`
	// Theme 是桌面控制面板的外观："dark"、"light" 或 "system"。
	// 客户端网页仍由访问者自行切换主题。
	Theme string `json:"theme,omitempty"`
}

func defaultConfig() Config {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "."
	}
	return Config{
		ShareDir:  filepath.Join(home, "LocalSeverShare"),
		Port:      8080,
		AutoStart: true,
		Theme:     "dark",
	}
}

func configFile() string {
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		dir = "."
	}
	return filepath.Join(dir, "LocalSever", "config.json")
}

func loadConfig() Config {
	cfg := defaultConfig()

	data, err := os.ReadFile(configFile())
	if err != nil {
		return cfg
	}

	// 用指针区分「未写入该字段」与「显式设为 false」。
	var saved struct {
		ShareDir  string `json:"shareDir"`
		Port      int    `json:"port"`
		AutoStart *bool  `json:"autoStart"`
		Theme     string `json:"theme"`
	}
	if err := json.Unmarshal(data, &saved); err != nil {
		return cfg
	}

	if saved.ShareDir != "" {
		cfg.ShareDir = saved.ShareDir
	}
	if saved.Port > 0 && saved.Port <= 65535 {
		cfg.Port = saved.Port
	}
	if saved.AutoStart != nil {
		cfg.AutoStart = *saved.AutoStart
	}
	switch saved.Theme {
	case "dark", "light", "system":
		cfg.Theme = saved.Theme
	}
	return cfg
}

func saveConfig(cfg Config) error {
	path := configFile()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
