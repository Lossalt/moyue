package com.lossalt.moyue;

import android.content.Context;
import android.graphics.Typeface;
import android.util.TypedValue;
import android.view.Gravity;
import android.view.View;
import android.view.ViewGroup;
import android.widget.BaseAdapter;
import android.widget.TextView;

import java.util.ArrayList;
import java.util.List;

final class ReaderAdapter extends BaseAdapter {
    static final int BOOK_TITLE = 0;
    static final int AUTHOR = 1;
    static final int CHAPTER_TITLE = 2;
    static final int BODY = 3;
    static final int QUOTE = 4;
    static final int DIVIDER = 5;

    static final class Row {
        final int type;
        final String text;
        final int chapter;
        Row(int type, String text, int chapter) {
            this.type = type;
            this.text = text;
            this.chapter = chapter;
        }
    }

    private final Context context;
    private final List<Row> rows = new ArrayList<>();
    private Typeface typeface = Typeface.SERIF;
    private float fontSize = 19f;
    private float lineSpacing = 1.7f;
    private Theme theme = Theme.ALL[0];

    ReaderAdapter(Context context) { this.context = context; }

    void setRows(List<Row> data) {
        rows.clear();
        if (data != null) rows.addAll(data);
        notifyDataSetChanged();
    }

    void setStyle(Typeface typeface, float fontSize, float lineSpacing, Theme theme) {
        if (typeface != null) this.typeface = typeface;
        this.fontSize = fontSize;
        this.lineSpacing = lineSpacing;
        this.theme = theme;
        notifyDataSetChanged();
    }

    int chapterAt(int position) {
        if (position < 0 || position >= rows.size()) return -1;
        return rows.get(position).chapter;
    }

    int rowCount() { return rows.size(); }

    @Override public int getCount() { return rows.size(); }
    @Override public Object getItem(int position) { return rows.get(position); }
    @Override public long getItemId(int position) { return position; }
    @Override public int getViewTypeCount() { return 6; }
    @Override public int getItemViewType(int position) { return rows.get(position).type; }

    @Override
    public View getView(int position, View convertView, ViewGroup parent) {
        TextView tv;
        if (convertView instanceof TextView) {
            tv = (TextView) convertView;
        } else {
            tv = new TextView(context);
            tv.setBackgroundColor(theme.paper);
            tv.setIncludeFontPadding(false);
            tv.setBreakStrategy(android.text.Layout.BREAK_STRATEGY_SIMPLE);
            tv.setHyphenationFrequency(android.text.Layout.HYPHENATION_FREQUENCY_NONE);
            tv.setSingleLine(false);
        }
        Row row = rows.get(position);
        int side = dp(22);
        int top = dp(5);
        int bottom = dp(5);
        float sizeFactor = 1f;
        int style = Typeface.NORMAL;
        int gravity = Gravity.START;
        int color = theme.text;
        float spacing = lineSpacing;
        String text = row.text == null ? "" : row.text;

        switch (row.type) {
            case BOOK_TITLE:
                side = dp(26); top = dp(38); bottom = dp(8);
                sizeFactor = 2f; style = Typeface.BOLD; gravity = Gravity.CENTER;
                spacing = 1.15f;
                break;
            case AUTHOR:
                side = dp(26); top = dp(2); bottom = dp(26);
                color = theme.muted; gravity = Gravity.CENTER;
                spacing = 1.2f;
                break;
            case CHAPTER_TITLE:
                side = dp(20); top = dp(26); bottom = dp(12);
                sizeFactor = 1.42f; style = Typeface.BOLD; gravity = Gravity.CENTER;
                spacing = 1.2f;
                break;
            case QUOTE:
                side = dp(38); top = dp(7); bottom = dp(7);
                color = theme.muted; spacing = lineSpacing;
                text = "\u3000" + text;
                break;
            case DIVIDER:
                side = dp(20); top = dp(16); bottom = dp(16);
                sizeFactor = 1f; color = theme.muted; gravity = Gravity.CENTER;
                if (text.trim().isEmpty()) text = "· · ·";
                spacing = 1.2f;
                break;
            default:
                text = "\u3000\u3000" + text;
                break;
        }

        tv.setBackgroundColor(theme.paper);
        tv.setTextColor(color);
        tv.setTypeface(typeface, style);
        tv.setTextSize(TypedValue.COMPLEX_UNIT_SP, fontSize * sizeFactor);
        tv.setLineSpacing(0f, spacing);
        tv.setGravity(gravity | Gravity.CENTER_VERTICAL);
        tv.setPadding(side, top, side, bottom);
        tv.setText(text);
        return tv;
    }

    private int dp(float v) {
        return Math.round(v * context.getResources().getDisplayMetrics().density);
    }
}
