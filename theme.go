//go:build windows

package main

type Theme struct {
	Name      string
	Paper     uint32
	Text      uint32
	Muted     uint32
	Toolbar   uint32
	Control   uint32
	Hover     uint32
	Selected  uint32
	Border    uint32
	Accent    uint32
	OnAccent  uint32
	Watermark uint32
	Scroll    uint32
	Danger    uint32
}

func colorRef(hex uint32) uint32 {
	r := (hex >> 16) & 0xFF
	g := (hex >> 8) & 0xFF
	b := hex & 0xFF
	return rgb(r, g, b)
}

func themes() [3]Theme {
	return [3]Theme{
		{
			Name:      "纸黄",
			Paper:     colorRef(0xF7F1E6),
			Text:      colorRef(0x2A2621),
			Muted:     colorRef(0x8C8174),
			Toolbar:   colorRef(0xEFE7D9),
			Control:   colorRef(0xE7DDCC),
			Hover:     colorRef(0xE1D4BF),
			Selected:  colorRef(0xD9C8AE),
			Border:    colorRef(0xD8CBB7),
			Accent:    colorRef(0x7C4A2D),
			OnAccent:  colorRef(0xFFF8EC),
			Watermark: colorRef(0xEFE4D1),
			Scroll:    colorRef(0xC3B098),
			Danger:    colorRef(0xA84535),
		},
		{
			Name:      "青瓷",
			Paper:     colorRef(0xE9EFE7),
			Text:      colorRef(0x253029),
			Muted:     colorRef(0x77847A),
			Toolbar:   colorRef(0xDDE7DC),
			Control:   colorRef(0xD4E0D1),
			Hover:     colorRef(0xC9D8C6),
			Selected:  colorRef(0xBFD1BC),
			Border:    colorRef(0xC9D7C7),
			Accent:    colorRef(0x3E6A4E),
			OnAccent:  colorRef(0xF4FAF2),
			Watermark: colorRef(0xDEE8DB),
			Scroll:    colorRef(0xAFC0AC),
			Danger:    colorRef(0xA84535),
		},
		{
			Name:      "夜读",
			Paper:     colorRef(0x191A1C),
			Text:      colorRef(0xD2CCC1),
			Muted:     colorRef(0x8A8378),
			Toolbar:   colorRef(0x212226),
			Control:   colorRef(0x2A2C31),
			Hover:     colorRef(0x34373C),
			Selected:  colorRef(0x3B3E44),
			Border:    colorRef(0x36383D),
			Accent:    colorRef(0xC69A64),
			OnAccent:  colorRef(0x1B1B1D),
			Watermark: colorRef(0x232428),
			Scroll:    colorRef(0x4C4F55),
			Danger:    colorRef(0xD17A6A),
		},
	}
}
