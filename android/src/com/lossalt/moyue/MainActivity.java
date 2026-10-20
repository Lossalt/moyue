package com.lossalt.moyue;

import android.app.Activity;
import android.app.Dialog;
import android.content.Intent;
import android.content.SharedPreferences;
import android.database.Cursor;
import android.graphics.Color;
import android.graphics.Typeface;
import android.graphics.drawable.ColorDrawable;
import android.graphics.drawable.GradientDrawable;
import android.net.Uri;
import android.os.Bundle;
import android.provider.OpenableColumns;
import android.text.TextUtils;
import android.util.TypedValue;
import android.view.Gravity;
import android.view.View;
import android.view.ViewGroup;
import android.view.Window;
import android.view.WindowManager;
import android.widget.AbsListView;
import android.widget.AdapterView;
import android.widget.BaseAdapter;
import android.widget.FrameLayout;
import android.widget.HorizontalScrollView;
import android.widget.LinearLayout;
import android.widget.ListView;
import android.widget.ScrollView;
import android.widget.TextView;
import android.widget.Toast;

import java.io.File;
import java.io.FileOutputStream;
import java.io.InputStream;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Locale;
import java.util.Map;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

public final class MainActivity extends Activity {
    private static final int REQ_OPEN = 1001;
    private static final String PREFS = "moyue_reader";
    private static final String KEY_FONT = "font";
    private static final String KEY_SIZE = "font_size";
    private static final String KEY_LINE = "line_spacing";
    private static final String KEY_THEME = "theme";
    private static final String KEY_URI = "last_uri";
    private static final String KEY_TITLE = "last_title";
    private static final String KEY_POS = "last_pos";

    private SharedPreferences prefs;
    private final ExecutorService executor = Executors.newSingleThreadExecutor();
    private final Map<String, Typeface> fonts = new HashMap<>();
    private Theme theme = Theme.ALL[0];
    private int themeIndex = 0;
    private String fontKey = "serif";
    private float fontSize = 19f;
    private float lineSpacing = 1.7f;

    private LinearLayout root;
    private HorizontalScrollView toolbarScroll;
    private LinearLayout toolbar;
    private ListView list;
    private LinearLayout empty;
    private TextView emptyBrand, emptySub, emptyPrimary, emptySecondary, emptyHint;
    private TextView status;
    private TextView openBtn, tocBtn, fontBtn, minusBtn, sizeBtn, plusBtn, lineBtn, themeBtn, moreBtn;

    private ReaderAdapter adapter;
    private Epub.Book book;
    private Uri currentUri;
    private String currentUriString = "";
    private String lastUri = "";
    private String lastTitle = "";
    private boolean loading = false;
    private int loadToken = 0;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        prefs = getSharedPreferences(PREFS, MODE_PRIVATE);
        fontKey = prefs.getString(KEY_FONT, "serif");
        fontSize = prefs.getFloat(KEY_SIZE, 19f);
        lineSpacing = prefs.getFloat(KEY_LINE, 1.7f);
        themeIndex = prefs.getInt(KEY_THEME, 0);
        lastUri = prefs.getString(KEY_URI, "");
        lastTitle = prefs.getString(KEY_TITLE, "");
        theme = Theme.get(themeIndex);
        loadFonts();
        buildInterface();
        applyTheme();
        Uri incoming = getIntent() == null ? null : getIntent().getData();
        if (incoming != null) {
            loadUri(incoming, true);
            return;
        }
        if (!lastUri.isEmpty()) {
            try {
                loadUri(Uri.parse(lastUri), false);
                return;
            } catch (Exception ignored) {}
        }
        showWelcome(null);
    }

    @Override
    protected void onNewIntent(Intent intent) {
        super.onNewIntent(intent);
        if (intent == null) return;
        setIntent(intent);
        Uri uri = intent.getData();
        if (uri != null) loadUri(uri, true);
    }

    private void loadFonts() {
        fonts.put("serif", loadFont(R.font.ink_serif, Typeface.SERIF));
        fonts.put("sans", loadFont(R.font.ink_sans, Typeface.SANS_SERIF));
        fonts.put("kai", loadFont(R.font.ink_kai, Typeface.create("serif", Typeface.NORMAL)));
    }

    private Typeface loadFont(int res, Typeface fallback) {
        try {
            return getResources().getFont(res);
        } catch (Exception e) {
            return fallback;
        }
    }

    private Typeface typeface() {
        Typeface t = fonts.get(fontKey);
        return t == null ? Typeface.SERIF : t;
    }

    private void buildInterface() {
        root = new LinearLayout(this);
        root.setOrientation(LinearLayout.VERTICAL);
        root.setBackgroundColor(theme.paper);

        toolbarScroll = new HorizontalScrollView(this);
        toolbarScroll.setHorizontalScrollBarEnabled(false);
        toolbarScroll.setFillViewport(false);
        toolbarScroll.setBackgroundColor(theme.toolbar);
        toolbar = new LinearLayout(this);
        toolbar.setOrientation(LinearLayout.HORIZONTAL);
        toolbar.setGravity(Gravity.CENTER_VERTICAL);
        toolbar.setPadding(dp(7), dp(5), dp(7), dp(5));
        toolbarScroll.addView(toolbar, new FrameLayout.LayoutParams(
                ViewGroup.LayoutParams.WRAP_CONTENT, ViewGroup.LayoutParams.MATCH_PARENT));
        root.addView(toolbarScroll, new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, dp(54)));

        openBtn = addTool("打开", new View.OnClickListener() {
            @Override public void onClick(View v) { pickEpub(); }
        });
        tocBtn = addTool("目录", new View.OnClickListener() {
            @Override public void onClick(View v) { showToc(); }
        });
        fontBtn = addTool("宋体", new View.OnClickListener() {
            @Override public void onClick(View v) { showFontDialog(); }
        });
        minusBtn = addTool("A−", new View.OnClickListener() {
            @Override public void onClick(View v) { changeSize(-1); }
        });
        sizeBtn = addTool("19", new View.OnClickListener() {
            @Override public void onClick(View v) { showSizeDialog(); }
        });
        plusBtn = addTool("A+", new View.OnClickListener() {
            @Override public void onClick(View v) { changeSize(1); }
        });
        lineBtn = addTool("行距", new View.OnClickListener() {
            @Override public void onClick(View v) { showLineDialog(); }
        });
        themeBtn = addTool("☀", new View.OnClickListener() {
            @Override public void onClick(View v) { showThemeDialog(); }
        });
        moreBtn = addTool("⋯", new View.OnClickListener() {
            @Override public void onClick(View v) { showMore(); }
        });

        FrameLayout content = new FrameLayout(this);
        root.addView(content, new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, 0, 1f));

        list = new ListView(this);
        list.setDivider(null);
        list.setDividerHeight(0);
        list.setClipToPadding(false);
        list.setBackgroundColor(theme.paper);
        list.setSelector(new ColorDrawable(Color.TRANSPARENT));
        list.setVerticalScrollBarEnabled(true);
        list.setOverScrollMode(View.OVER_SCROLL_IF_CONTENT_SCROLLS);
        int widthDp = getResources().getConfiguration().screenWidthDp;
        int side = Math.max(dp(10), dp((widthDp - 720) / 2));
        list.setPadding(side, dp(6), side, dp(18));
        content.addView(list, new FrameLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT));

        buildEmptyView();
        content.addView(empty, new FrameLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT));
        list.setEmptyView(empty);

        status = new TextView(this);
        status.setTextSize(TypedValue.COMPLEX_UNIT_SP, 11f);
        status.setGravity(Gravity.CENTER_VERTICAL);
        status.setSingleLine(true);
        status.setEllipsize(TextUtils.TruncateAt.END);
        status.setPadding(dp(14), 0, dp(14), 0);
        status.setBackgroundColor(theme.toolbar);
        status.setTextColor(theme.muted);
        root.addView(status, new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, dp(30)));

        setContentView(root);
        list.setOnScrollListener(new AbsListView.OnScrollListener() {
            @Override public void onScrollStateChanged(AbsListView view, int scrollState) {}
            @Override public void onScroll(AbsListView view, int firstVisibleItem, int visibleItemCount, int totalItemCount) {
                updateStatus();
            }
        });
  }

    private TextView addTool(String text, View.OnClickListener listener) {
        TextView v = new TextView(this);
        v.setText(text);
        v.setTextSize(TypedValue.COMPLEX_UNIT_SP, 12.5f);
        v.setGravity(Gravity.CENTER);
        v.setSingleLine(true);
        v.setPadding(dp(9), dp(7), dp(9), dp(7));
        LinearLayout.LayoutParams lp = new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.WRAP_CONTENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        lp.setMargins(dp(2), 0, dp(2), 0);
        v.setLayoutParams(lp);
        v.setClickable(true);
        v.setFocusable(true);
        v.setOnClickListener(listener);
        toolbar.addView(v);
        return v;
    }

    private void buildEmptyView() {
        empty = new LinearLayout(this);
        empty.setOrientation(LinearLayout.VERTICAL);
        empty.setGravity(Gravity.CENTER);
        empty.setPadding(dp(28), 0, dp(28), 0);
        empty.setBackgroundColor(theme.paper);

        emptyBrand = new TextView(this);
        emptyBrand.setText("墨阅");
        emptyBrand.setGravity(Gravity.CENTER);
        emptyBrand.setTypeface(typeface(), Typeface.BOLD);
        emptyBrand.setTextSize(TypedValue.COMPLEX_UNIT_SP, 38f);
        emptyBrand.setTextColor(theme.text);
        empty.addView(emptyBrand, wrap());

        emptySub = new TextView(this);
        emptySub.setText("轻量 EPUB 小说阅读器");
        emptySub.setGravity(Gravity.CENTER);
        emptySub.setTextSize(TypedValue.COMPLEX_UNIT_SP, 13f);
        emptySub.setTextColor(theme.muted);
        LinearLayout.LayoutParams subLp = wrap();
        subLp.topMargin = dp(6);
        subLp.bottomMargin = dp(34);
        empty.addView(emptySub, subLp);

        emptyPrimary = makePill("打开一本 EPUB", true);
        emptyPrimary.setOnClickListener(new View.OnClickListener() {
            @Override public void onClick(View v) { pickEpub(); }
        });
        LinearLayout.LayoutParams plp = new LinearLayout.LayoutParams(dp(230), dp(50));
        plp.gravity = Gravity.CENTER_HORIZONTAL;
        empty.addView(emptyPrimary, plp);

        emptySecondary = makePill("继续阅读", false);
        emptySecondary.setOnClickListener(new View.OnClickListener() {
            @Override public void onClick(View v) {
                if (!lastUri.isEmpty()) {
                    try { loadUri(Uri.parse(lastUri), false); }
                    catch (Exception e) { showTextDialog("打开失败", "无法访问上次的书，请重新选择。"); }
                }
            }
        });
        LinearLayout.LayoutParams slp = new LinearLayout.LayoutParams(dp(230), dp(44));
        slp.gravity = Gravity.CENTER_HORIZONTAL;
        slp.topMargin = dp(12);
        empty.addView(emptySecondary, slp);

        emptyHint = new TextView(this);
        emptyHint.setGravity(Gravity.CENTER);
        emptyHint.setTextSize(TypedValue.COMPLEX_UNIT_SP, 11f);
        emptyHint.setTextColor(theme.muted);
        emptyHint.setMaxLines(4);
        emptyHint.setEllipsize(TextUtils.TruncateAt.END);
        LinearLayout.LayoutParams hlp = wrap();
        hlp.topMargin = dp(20);
        empty.addView(emptyHint, hlp);
    }

    private TextView makePill(String text, boolean primary) {
        TextView v = new TextView(this);
        v.setText(text);
        v.setGravity(Gravity.CENTER);
        v.setTextSize(TypedValue.COMPLEX_UNIT_SP, primary ? 15f : 14f);
        v.setTypeface(Typeface.DEFAULT, primary ? Typeface.BOLD : Typeface.NORMAL);
        v.setClickable(true);
        v.setFocusable(true);
        stylePill(v, primary);
        return v;
    }

    private void stylePill(TextView v, boolean primary) {
        GradientDrawable d = new GradientDrawable();
        d.setCornerRadius(dp(10));
        d.setColor(primary ? theme.accent : theme.control);
        v.setBackground(d);
        v.setTextColor(primary ? theme.onAccent : theme.text);
    }

    private LinearLayout.LayoutParams wrap() {
        return new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.WRAP_CONTENT, ViewGroup.LayoutParams.WRAP_CONTENT);
    }

    private void applyTheme() {
        theme = Theme.get(themeIndex);
        root.setBackgroundColor(theme.paper);
        toolbarScroll.setBackgroundColor(theme.toolbar);
        toolbar.setBackgroundColor(theme.toolbar);
        status.setBackgroundColor(theme.toolbar);
        status.setTextColor(theme.muted);
        list.setBackgroundColor(theme.paper);
        empty.setBackgroundColor(theme.paper);
        emptyBrand.setTextColor(theme.text);
        emptySub.setTextColor(theme.muted);
        emptyHint.setTextColor(theme.muted);
        stylePill(emptyPrimary, true);
        stylePill(emptySecondary, false);
        for (TextView v : new TextView[]{openBtn, tocBtn, fontBtn, minusBtn, sizeBtn, plusBtn, lineBtn, themeBtn, moreBtn}) {
            if (v == null) continue;
            boolean primary = v == openBtn;
            GradientDrawable d = new GradientDrawable();
            d.setCornerRadius(dp(8));
            d.setColor(primary ? theme.accent : theme.control);
            v.setBackground(d);
            v.setTextColor(primary ? theme.onAccent : theme.text);
        }
        if (adapter != null) {
            int pos = list.getFirstVisiblePosition();
            adapter.setStyle(typeface(), fontSize, lineSpacing, theme);
            list.setSelection(pos);
        }
        applySystemBars();
        refreshToolbar();
    }

    private void applySystemBars() {
        Window w = getWindow();
        w.setStatusBarColor(theme.toolbar);
        w.setNavigationBarColor(theme.paper);
        View decor = w.getDecorView();
        int flags = decor.getSystemUiVisibility();
        int light = View.SYSTEM_UI_FLAG_LIGHT_STATUS_BAR | View.SYSTEM_UI_FLAG_LIGHT_NAVIGATION_BAR;
        if (theme.dark) flags &= ~light;
        else flags |= light;
        decor.setSystemUiVisibility(flags);
    }

    private void refreshToolbar() {
        fontBtn.setText(fontLabel());
        sizeBtn.setText(String.valueOf(Math.round(fontSize)));
        lineBtn.setText(String.format(Locale.US, "行距 %.1f", lineSpacing));
        themeBtn.setText(theme.dark ? "☾" : "☀");
        tocBtn.setAlpha(book == null ? 0.45f : 1f);
    }

    private String fontLabel() {
        if ("sans".equals(fontKey)) return "黑体";
        if ("kai".equals(fontKey)) return "楷体";
        return "宋体";
    }

    private void showWelcome(String error) {
        loading = false;
        emptyBrand.setTypeface(typeface(), Typeface.BOLD);
        emptyBrand.setTextSize(TypedValue.COMPLEX_UNIT_SP, 38f);
        emptyBrand.setText("墨阅");
        emptySub.setText(error == null ? "轻量 EPUB 小说阅读器" : "没有打开书籍");
        emptyPrimary.setVisibility(View.VISIBLE);
        emptyPrimary.setText("打开一本 EPUB");
        if (lastUri.isEmpty()) {
            emptySecondary.setVisibility(View.GONE);
        } else {
            emptySecondary.setVisibility(View.VISIBLE);
            emptySecondary.setText("继续阅读《" + shortTitle(lastTitle) + "》");
        }
        emptyHint.setText(error == null ? "支持 EPUB 2 / EPUB 3 · 完全离线" : error);
        emptyHint.setTextColor(error == null ? theme.muted : theme.accent);
        status.setText("未打开书籍");
        book = null;
        adapter = null;
        list.setAdapter(null);
        empty.setVisibility(View.VISIBLE);
        refreshToolbar();
    }

    private void showLoading(String name) {
        loading = true;
        emptyBrand.setTypeface(typeface(), Typeface.NORMAL);
        emptyBrand.setTextSize(TypedValue.COMPLEX_UNIT_SP, 22f);
        emptyBrand.setText("正在展开书页…");
        emptySub.setText(name == null ? "" : name);
        emptyPrimary.setVisibility(View.GONE);
        emptySecondary.setVisibility(View.GONE);
        emptyHint.setText("正在解析目录与正文，请稍候");
        emptyHint.setTextColor(theme.muted);
        status.setText("正在打开…");
        list.setAdapter(null);
        empty.setVisibility(View.VISIBLE);
    }

    private void displayBook(Epub.Book b, Uri uri, int position) {
        book = b;
        currentUri = uri;
        currentUriString = uri.toString();
        lastUri = currentUriString;
        lastTitle = b.title;
        adapter = new ReaderAdapter(this);
        adapter.setStyle(typeface(), fontSize, lineSpacing, theme);
        List<ReaderAdapter.Row> rows = buildRows(b);
        adapter.setRows(rows);
        list.setAdapter(adapter);
        empty.setVisibility(View.GONE);
        int pos = Math.max(0, Math.min(position, rows.size() - 1));
        final int target = pos;
        list.post(new Runnable() {
            @Override public void run() {
                list.setSelection(target);
                updateStatus();
            }
        });
        prefs.edit().putString(KEY_URI, currentUriString).putString(KEY_TITLE, b.title).apply();
        cleanupCache();
        refreshToolbar();
    }

    private List<ReaderAdapter.Row> buildRows(Epub.Book b) {
        List<ReaderAdapter.Row> rows = new ArrayList<>();
        rows.add(new ReaderAdapter.Row(ReaderAdapter.BOOK_TITLE, b.title, -1));
        rows.add(new ReaderAdapter.Row(ReaderAdapter.AUTHOR, b.author, -1));
        for (int i = 0; i < b.chapters.size(); i++) {
            Epub.Chapter ch = b.chapters.get(i);
            ch.startRow = rows.size();
            rows.add(new ReaderAdapter.Row(ReaderAdapter.CHAPTER_TITLE, ch.title, i));
            for (Epub.Block block : ch.blocks) {
                int type;
                if (block.type == Epub.BLOCK_HEADING) type = ReaderAdapter.CHAPTER_TITLE;
                else if (block.type == Epub.BLOCK_QUOTE) type = ReaderAdapter.QUOTE;
                else if (block.type == Epub.BLOCK_DIVIDER) type = ReaderAdapter.DIVIDER;
                else type = ReaderAdapter.BODY;
                rows.add(new ReaderAdapter.Row(type, block.text, i));
            }
        }
        rows.add(new ReaderAdapter.Row(ReaderAdapter.DIVIDER, "· 全书完 ·", -1));
        return rows;
    }

    private void pickEpub() {
        if (loading) return;
        Intent intent = new Intent(Intent.ACTION_OPEN_DOCUMENT);
        intent.addCategory(Intent.CATEGORY_OPENABLE);
        intent.setType("*/*");
        intent.putExtra(Intent.EXTRA_MIME_TYPES, new String[]{
                "application/epub+zip", "application/zip", "application/octet-stream", "text/xml"
        });
        try {
            startActivityForResult(intent, REQ_OPEN);
        } catch (Exception e) {
            showTextDialog("无法打开文件选择器", "请确认系统文件管理器可用。");
        }
    }

    @Override
    protected void onActivityResult(int requestCode, int resultCode, Intent data) {
        super.onActivityResult(requestCode, resultCode, data);
        if (requestCode != REQ_OPEN || resultCode != RESULT_OK || data == null) return;
        final Uri uri = data.getData();
        if (uri == null) return;
        try {
            int flags = data.getFlags() & (Intent.FLAG_GRANT_READ_URI_PERMISSION | Intent.FLAG_GRANT_WRITE_URI_PERMISSION);
            getContentResolver().takePersistableUriPermission(uri, flags);
        } catch (Exception ignored) {}
        loadUri(uri, true);
    }

    private void loadUri(final Uri uri, final boolean userPicked) {
        if (loading) return;
        final int token = ++loadToken;
        showLoading(displayName(uri));
        executor.execute(new Runnable() {
            @Override public void run() {
                try {
                    File file = copyToCache(uri);
                    final Epub.Book parsed = Epub.parse(file);
                    final int pos = userPicked ? 0 : prefs.getInt(KEY_POS, 0);
                    runOnUiThread(new Runnable() {
                        @Override public void run() {
                            if (token != loadToken || isFinishing()) return;
                            loading = false;
                            displayBook(parsed, uri, pos);
                        }
                    });
                } catch (final Exception e) {
                    runOnUiThread(new Runnable() {
                        @Override public void run() {
                            if (token != loadToken || isFinishing()) return;
                            loading = false;
                            String msg = e.getMessage();
                            if (msg == null || msg.trim().isEmpty()) msg = "无法读取这本 EPUB";
                            if (!userPicked) {
                                lastUri = "";
                                lastTitle = "";
                                prefs.edit().remove(KEY_URI).remove(KEY_TITLE).apply();
                            }
                            showWelcome(msg);
                            showTextDialog("打开失败", msg);
                        }
                    });
                }
            }
        });
    }

    private File copyToCache(Uri uri) throws Exception {
        File out = File.createTempFile("moyue-", ".epub", getCacheDir());
        InputStream in = getContentResolver().openInputStream(uri);
        if (in == null) throw new Exception("无法读取文件");
        try {
            FileOutputStream fos = new FileOutputStream(out);
            try {
                byte[] buf = new byte[64 * 1024];
                long total = 0;
                int n;
                while ((n = in.read(buf)) > 0) {
                    total += n;
                    if (total > 200L * 1024 * 1024) throw new Exception("文件过大");
                    fos.write(buf, 0, n);
                }
            } finally {
                try { fos.close(); } catch (Exception ignored) {}
            }
        } finally {
            try { in.close(); } catch (Exception ignored) {}
        }
        return out;
    }

    private void cleanupCache() {
        File[] files = getCacheDir().listFiles();
        if (files == null) return;
        long cutoff = System.currentTimeMillis() - 60L * 60 * 1000;
        for (File f : files) {
            if (f.getName().startsWith("moyue-") && f.getName().endsWith(".epub") && f.lastModified() < cutoff) {
                try { f.delete(); } catch (Exception ignored) {}
            }
        }
    }

    private String displayName(Uri uri) {
        String name = null;
        try {
            Cursor c = getContentResolver().query(uri, new String[]{OpenableColumns.DISPLAY_NAME}, null, null, null);
            if (c != null) {
                try {
                    if (c.moveToFirst()) {
                        int idx = c.getColumnIndex(OpenableColumns.DISPLAY_NAME);
                        if (idx >= 0) name = c.getString(idx);
                    }
                } finally { c.close(); }
            }
        } catch (Exception ignored) {}
        if (name == null) name = uri.getLastPathSegment();
        return name == null ? "EPUB" : name;
    }

    private int currentChapter() {
        if (adapter == null) return -1;
        return adapter.chapterAt(list.getFirstVisiblePosition());
    }

    private void updateStatus() {
        if (adapter == null || book == null) {
            status.setText("未打开书籍");
            return;
        }
        int total = adapter.rowCount();
        int pos = Math.max(0, list.getFirstVisiblePosition());
        int ch = adapter.chapterAt(pos);
        int percent = total <= 1 ? 0 : Math.round(pos * 100f / (total - 1));
        if (ch >= 0 && ch < book.chapters.size()) {
            status.setText(String.format(Locale.US, "第 %d/%d 章 · %d%%", ch + 1, book.chapters.size(), percent));
        } else {
            status.setText(String.format(Locale.US, "扉页 · %d%%", percent));
        }
    }

    private void showToc() {
        if (book == null) {
            Toast.makeText(this, "请先打开一本 EPUB", Toast.LENGTH_SHORT).show();
            return;
        }
        int n = book.chapters.size();
        String[] labels = new String[n];
        for (int i = 0; i < n; i++) {
            Epub.Chapter ch = book.chapters.get(i);
            StringBuilder sb = new StringBuilder();
            int level = Math.min(2, Math.max(0, ch.level));
            for (int k = 0; k < level; k++) sb.append('\u3000');
            sb.append(ch.title);
            labels[i] = sb.toString();
        }
        final int selected = Math.max(0, currentChapter());
        showChoiceDialog("目录 · " + n + " 章", labels, selected, new ChoiceListener() {
            @Override public void onChoice(int index) { jumpChapter(index); }
        });
    }

    private void jumpChapter(int index) {
        if (book == null || index < 0 || index >= book.chapters.size()) return;
        Epub.Chapter ch = book.chapters.get(index);
        list.setSelection(ch.startRow);
        list.post(new Runnable() {
            @Override public void run() { updateStatus(); }
        });
    }

    private void showFontDialog() {
        final String[] keys = {"serif", "sans", "kai"};
        String[] labels = {"宋体 · 思源宋体", "黑体 · 思源黑体", "楷体 · 霞鹜文楷"};
        int selected = 0;
        if ("sans".equals(fontKey)) selected = 1;
        if ("kai".equals(fontKey)) selected = 2;
        showChoiceDialog("字体", labels, selected, new ChoiceListener() {
            @Override public void onChoice(int index) {
                setFont(keys[index]);
            }
        });
    }

    private void showSizeDialog() {
        final int min = 13, max = 40;
        String[] labels = new String[max - min + 1];
        for (int i = min; i <= max; i++) labels[i - min] = i + " pt";
        int selected = Math.max(0, Math.min(labels.length - 1, Math.round(fontSize) - min));
        showChoiceDialog("字号", labels, selected, new ChoiceListener() {
            @Override public void onChoice(int index) {
                setSize(min + index);
            }
        });
    }

    private void showLineDialog() {
        final float[] values = {1.2f, 1.4f, 1.6f, 1.7f, 1.8f, 2.0f, 2.2f, 2.5f, 3.0f};
        String[] labels = {"紧凑 1.2", "舒适 1.4", "宽松 1.6", "标准 1.7", "疏朗 1.8", "大字 2.0", "宽松 2.2", "疏朗 2.5", "极宽 3.0"};
        int selected = 0;
        for (int i = 0; i < values.length; i++) {
            if (Math.abs(values[i] - lineSpacing) < 0.05f) selected = i;
        }
        showChoiceDialog("行距", labels, selected, new ChoiceListener() {
            @Override public void onChoice(int index) {
                setLine(values[index]);
            }
        });
    }

    private void showThemeDialog() {
        String[] labels = {"纸黄 · 温暖纸页", "青瓷 · 护眼淡绿", "夜读 · 深色模式"};
        showChoiceDialog("主题", labels, themeIndex, new ChoiceListener() {
            @Override public void onChoice(int index) {
                setReaderTheme(index);
            }
        });
    }

    private void showMore() {
        String[] labels = {"快捷键", "关于墨阅"};
        showChoiceDialog("更多", labels, -1, new ChoiceListener() {
            @Override public void onChoice(int index) {
                if (index == 0) showTextDialog("快捷键", "Android 版通过工具栏操作：\n\n· 打开：选择 EPUB 文件\n· 目录：跳转章节\n· 宋体/黑体/楷体：切换字体\n· A− / A+：调整字号\n· 行距：调整行间距\n· ☀/☾：切换主题\n\n退出后会自动记住上次的书和阅读位置。");
                else showTextDialog("关于墨阅", "墨阅 1.0.0 · Android 版\n\n轻量 EPUB 小说阅读器。单 APK、完全离线，内嵌思源宋体、思源黑体和霞鹜文楷。\n\n字体以 SIL Open Font License 1.1 授权。");
            }
        });
    }

    private void changeSize(int delta) {
        setSize(Math.round(fontSize) + delta);
    }

    private void setSize(int size) {
        float v = Math.max(13f, Math.min(40f, size));
        if (v == fontSize) return;
        int pos = list.getFirstVisiblePosition();
        fontSize = v;
        if (adapter != null) {
            adapter.setStyle(typeface(), fontSize, lineSpacing, theme);
            list.setSelection(pos);
        }
        refreshToolbar();
        saveSettings();
    }

    private void setFont(String key) {
        if (key == null || key.equals(fontKey)) return;
        int pos = list.getFirstVisiblePosition();
        fontKey = key;
        if (adapter != null) {
            adapter.setStyle(typeface(), fontSize, lineSpacing, theme);
            list.setSelection(pos);
        }
        emptyBrand.setTypeface(typeface(), Typeface.BOLD);
        refreshToolbar();
        saveSettings();
    }

    private void setLine(float value) {
        if (Math.abs(value - lineSpacing) < 0.001f) return;
        int pos = list.getFirstVisiblePosition();
        lineSpacing = value;
        if (adapter != null) {
            adapter.setStyle(typeface(), fontSize, lineSpacing, theme);
            list.setSelection(pos);
        }
        refreshToolbar();
        saveSettings();
    }

    private void setReaderTheme(int index) {
        if (index < 0 || index >= Theme.ALL.length || index == themeIndex) return;
        int pos = list.getFirstVisiblePosition();
        themeIndex = index;
        theme = Theme.get(index);
        applyTheme();
        list.setSelection(pos);
        saveSettings();
    }

    private void saveSettings() {
        prefs.edit()
                .putString(KEY_FONT, fontKey)
                .putFloat(KEY_SIZE, fontSize)
                .putFloat(KEY_LINE, lineSpacing)
                .putInt(KEY_THEME, themeIndex)
                .apply();
    }

    @Override
    protected void onPause() {
        super.onPause();
        saveSettings();
        if (book != null && adapter != null) {
            prefs.edit()
                    .putString(KEY_URI, currentUriString)
                    .putString(KEY_TITLE, book.title)
                    .putInt(KEY_POS, list.getFirstVisiblePosition())
                    .apply();
        }
    }

    @Override
    protected void onDestroy() {
        loadToken++;
        executor.shutdownNow();
        super.onDestroy();
    }

    private void showChoiceDialog(String title, String[] items, int selected, final ChoiceListener listener) {
        final Dialog dialog = new Dialog(this);
        dialog.requestWindowFeature(Window.FEATURE_NO_TITLE);
        LinearLayout box = new LinearLayout(this);
        box.setOrientation(LinearLayout.VERTICAL);
        box.setPadding(dp(16), dp(16), dp(16), dp(16));
        GradientDrawable bg = new GradientDrawable();
        bg.setColor(theme.paper);
        bg.setCornerRadius(dp(16));
        box.setBackground(bg);

        TextView header = new TextView(this);
        header.setText(title);
        header.setTextSize(TypedValue.COMPLEX_UNIT_SP, 18f);
        header.setTypeface(Typeface.DEFAULT, Typeface.BOLD);
        header.setTextColor(theme.text);
        header.setPadding(dp(4), 0, dp(4), dp(10));
        box.addView(header);

        final ListView lv = new ListView(this);
        lv.setDivider(null);
        lv.setDividerHeight(0);
        lv.setBackgroundColor(Color.TRANSPARENT);
        lv.setSelector(new ColorDrawable(Color.TRANSPARENT));
        final int sel = selected;
        lv.setAdapter(new BaseAdapter() {
            @Override public int getCount() { return items.length; }
            @Override public Object getItem(int i) { return items[i]; }
            @Override public long getItemId(int i) { return i; }
            @Override public View getView(int i, View convertView, ViewGroup parent) {
                TextView tv = new TextView(MainActivity.this);
                String label = items[i];
                if (i == sel) label = "✓  " + label;
                tv.setText(label);
                tv.setTextSize(TypedValue.COMPLEX_UNIT_SP, 15f);
                tv.setGravity(Gravity.CENTER_VERTICAL);
                tv.setPadding(dp(14), dp(12), dp(14), dp(12));
                tv.setTextColor(i == sel ? theme.accent : theme.text);
                tv.setTypeface(Typeface.DEFAULT, i == sel ? Typeface.BOLD : Typeface.NORMAL);
                GradientDrawable rowBg = new GradientDrawable();
                rowBg.setCornerRadius(dp(8));
                rowBg.setColor(i == sel ? theme.selected : Color.TRANSPARENT);
                tv.setBackground(rowBg);
                return tv;
            }
        });
        int maxH = (int) (getResources().getDisplayMetrics().heightPixels * 0.62f);
        int wanted = dp(50) * items.length + dp(8);
        lv.setLayoutParams(new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, Math.min(maxH, wanted)));
        box.addView(lv);

        dialog.setContentView(box);
        dialog.setCanceledOnTouchOutside(true);
        lv.setOnItemClickListener(new AdapterView.OnItemClickListener() {
            @Override public void onItemClick(AdapterView<?> parent, View view, int position, long id) {
                dialog.dismiss();
                listener.onChoice(position);
            }
        });
        dialog.show();
        Window win = dialog.getWindow();
        if (win != null) {
            win.setBackgroundDrawable(new ColorDrawable(Color.TRANSPARENT));
            WindowManager.LayoutParams lp = win.getAttributes();
            int width = (int) (getResources().getDisplayMetrics().widthPixels * 0.86f);
            lp.width = Math.min(width, dp(480));
            lp.height = WindowManager.LayoutParams.WRAP_CONTENT;
            win.setAttributes(lp);
        }
    }

    private void showTextDialog(String title, String message) {
        final Dialog dialog = new Dialog(this);
        dialog.requestWindowFeature(Window.FEATURE_NO_TITLE);
        LinearLayout box = new LinearLayout(this);
        box.setOrientation(LinearLayout.VERTICAL);
        box.setPadding(dp(20), dp(18), dp(20), dp(16));
        GradientDrawable bg = new GradientDrawable();
        bg.setColor(theme.paper);
        bg.setCornerRadius(dp(16));
        box.setBackground(bg);

        TextView header = new TextView(this);
        header.setText(title);
        header.setTextSize(TypedValue.COMPLEX_UNIT_SP, 18f);
        header.setTypeface(Typeface.DEFAULT, Typeface.BOLD);
        header.setTextColor(theme.text);
        box.addView(header);

        ScrollView scroll = new ScrollView(this);
        TextView body = new TextView(this);
        body.setText(message);
        body.setTextSize(TypedValue.COMPLEX_UNIT_SP, 14f);
        body.setTextColor(theme.text);
        body.setLineSpacing(0f, 1.3f);
        body.setTextIsSelectable(true);
        body.setPadding(0, dp(12), 0, dp(10));
        scroll.addView(body);
        LinearLayout.LayoutParams slp = new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, 0, 1f);
        slp.height = Math.min((int) (getResources().getDisplayMetrics().heightPixels * 0.52f), dp(360));
        box.addView(scroll, slp);

        TextView ok = makePill("确定", true);
        LinearLayout.LayoutParams olp = new LinearLayout.LayoutParams(dp(92), dp(42));
        olp.gravity = Gravity.END;
        olp.topMargin = dp(8);
        box.addView(ok, olp);
        ok.setOnClickListener(new View.OnClickListener() {
            @Override public void onClick(View v) { dialog.dismiss(); }
        });

        dialog.setContentView(box);
        dialog.setCanceledOnTouchOutside(true);
        dialog.show();
        Window win = dialog.getWindow();
        if (win != null) {
            win.setBackgroundDrawable(new ColorDrawable(Color.TRANSPARENT));
            WindowManager.LayoutParams lp = win.getAttributes();
            int width = (int) (getResources().getDisplayMetrics().widthPixels * 0.86f);
            lp.width = Math.min(width, dp(480));
            lp.height = WindowManager.LayoutParams.WRAP_CONTENT;
            win.setAttributes(lp);
        }
    }

    private String shortTitle(String s) {
        if (s == null || s.isEmpty()) return "上次的书";
        return s.length() > 14 ? s.substring(0, 14) + "…" : s;
    }

    private int dp(float v) {
        return Math.round(v * getResources().getDisplayMetrics().density);
    }

    private interface ChoiceListener {
        void onChoice(int index);
    }
}
