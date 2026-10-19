package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type Settings struct {
	Font        string             `json:"font"`
	FontSize    float64            `json:"font_size"`
	LineSpacing float64            `json:"line_spacing"`
	Theme       int                `json:"theme"`
	TOC         bool               `json:"toc"`
	WindowX     int                `json:"window_x"`
	WindowY     int                `json:"window_y"`
	WindowW     int                `json:"window_w"`
	WindowH     int                `json:"window_h"`
	LastBook    string             `json:"last_book"`
	LastTitle   string             `json:"last_title"`
	Progress    map[string]float64 `json:"progress,omitempty"`
}

func defaultSettings() Settings {
	return Settings{
		Font:        "serif",
		FontSize:    19,
		LineSpacing: 1.7,
		Theme:       0,
		TOC:         false,
		WindowX:     -1,
		WindowY:     -1,
		WindowW:     1040,
		WindowH:     780,
		Progress:    make(map[string]float64),
	}
}

func settingsPath() string {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		base = os.Getenv("APPDATA")
	}
	if base == "" {
		base = "."
	}
	return filepath.Join(base, "MoYue", "settings.json")
}

func loadSettings() Settings {
	s := defaultSettings()
	data, err := os.ReadFile(settingsPath())
	if err == nil {
		_ = json.Unmarshal(data, &s)
	}
	s.normalize()
	return s
}

func (s *Settings) normalize() {
	if s.Font != "serif" && s.Font != "sans" && s.Font != "kai" {
		s.Font = "serif"
	}
	if s.FontSize < 13 {
		s.FontSize = 13
	}
	if s.FontSize > 40 {
		s.FontSize = 40
	}
	if s.LineSpacing < 1.1 {
		s.LineSpacing = 1.1
	}
	if s.LineSpacing > 3.0 {
		s.LineSpacing = 3.0
	}
	if s.Theme < 0 || s.Theme > 2 {
		s.Theme = 0
	}
	if s.WindowW < 720 {
		s.WindowW = 1040
	}
	if s.WindowH < 520 {
		s.WindowH = 780
	}
	if s.Progress == nil {
		s.Progress = make(map[string]float64)
	}
}

func saveSettings(s Settings) error {
	p := settingsPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func progressKey(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		abs = p
	}
	return strings.ToLower(filepath.Clean(abs))
}

func (s *Settings) bookProgress(p string) float64 {
	if s.Progress == nil {
		return 0
	}
	v, ok := s.Progress[progressKey(p)]
	if !ok {
		return 0
	}
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func (s *Settings) setBookProgress(p string, v float64) {
	if s.Progress == nil {
		s.Progress = make(map[string]float64)
	}
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	s.Progress[progressKey(p)] = v
}
