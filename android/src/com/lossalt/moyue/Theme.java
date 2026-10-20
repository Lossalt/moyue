package com.lossalt.moyue;

import android.graphics.Color;

/** Three reading themes, kept in sync with the desktop edition. */
final class Theme {
    final String name;
    final boolean dark;
    final int paper;
    final int text;
    final int muted;
    final int toolbar;
    final int control;
    final int selected;
    final int border;
    final int accent;
    final int onAccent;

    private Theme(String name, boolean dark, int paper, int text, int muted, int toolbar,
                  int control, int selected, int border, int accent, int onAccent) {
        this.name = name;
        this.dark = dark;
        this.paper = paper;
        this.text = text;
        this.muted = muted;
        this.toolbar = toolbar;
        this.control = control;
        this.selected = selected;
        this.border = border;
        this.accent = accent;
        this.onAccent = onAccent;
    }

    private static int rgb(int v) { return 0xFF000000 | v; }

    static final Theme[] ALL = new Theme[]{
            new Theme("纸黄", false, rgb(0xF7F1E6), rgb(0x2A2621), rgb(0x8C8174),
                    rgb(0xEFE7D9), rgb(0xE7DDCC), rgb(0xD9C8AE), rgb(0xD8CBB7),
                    rgb(0x7C4A2D), rgb(0xFFF8EC)),
            new Theme("青瓷", false, rgb(0xE9EFE7), rgb(0x253029), rgb(0x77847A),
                    rgb(0xDDE7DC), rgb(0xD4E0D1), rgb(0xBFD1BC), rgb(0xC9D7C7),
                    rgb(0x3E6A4E), rgb(0xF4FAF2)),
            new Theme("夜读", true, rgb(0x191A1C), rgb(0xD2CCC1), rgb(0x8A8378),
                    rgb(0x212226), rgb(0x2A2C31), rgb(0x3B3E44), rgb(0x36383D),
                    rgb(0xC69A64), rgb(0x1B1B1D))
    };

    static Theme get(int index) {
        if (index < 0 || index >= ALL.length) return ALL[0];
        return ALL[index];
    }

    static int dim(int color, float amount) {
        int a = Color.alpha(color);
        int r = Math.round(Color.red(color) * (1 - amount));
        int g = Math.round(Color.green(color) * (1 - amount));
        int b = Math.round(Color.blue(color) * (1 - amount));
        return Color.argb(a, r, g, b);
    }
}
