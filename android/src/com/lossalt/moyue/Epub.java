package com.lossalt.moyue;

import org.w3c.dom.Document;
import org.w3c.dom.Element;
import org.w3c.dom.Node;
import org.w3c.dom.NodeList;

import java.io.ByteArrayOutputStream;
import java.io.File;
import java.io.FileInputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.StringReader;
import java.net.URLDecoder;
import java.nio.charset.Charset;
import java.nio.charset.StandardCharsets;
import java.util.ArrayDeque;
import java.util.ArrayList;
import java.util.Deque;
import java.util.HashMap;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Locale;
import java.util.Map;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
import java.util.zip.ZipEntry;
import java.util.zip.ZipFile;

import javax.xml.parsers.DocumentBuilder;
import javax.xml.parsers.DocumentBuilderFactory;
import org.xml.sax.InputSource;

/** Minimal, dependency-free EPUB 2 / EPUB 3 reader. */
public final class Epub {
    public static final int BLOCK_BODY = 0;
    public static final int BLOCK_HEADING = 1;
    public static final int BLOCK_QUOTE = 2;
    public static final int BLOCK_DIVIDER = 3;
    public static final int BLOCK_IMAGE = 4;

    public static final class Block {
        public final int type;
        public final String text;
        public final String imageSrc;
        public final String imageAlt;
        public final int imageIndex;
        Block(int type, String text) { this(type, text, null, null, -1); }
        Block(int type, String text, String imageSrc, String imageAlt, int imageIndex) {
            this.type = type;
            this.text = text;
            this.imageSrc = imageSrc;
            this.imageAlt = imageAlt;
            this.imageIndex = imageIndex;
        }
    }

    public static final class ImageAsset {
        public final String href;
        public final String mediaType;
        public final String alt;
        public final byte[] data;
        ImageAsset(String href, String mediaType, String alt, byte[] data) {
            this.href = href;
            this.mediaType = mediaType;
            this.alt = alt;
            this.data = data;
        }
    }

    private static final class ImageRef {
        final String src;
        final String alt;
        ImageRef(String src, String alt) { this.src = src; this.alt = alt; }
    }

    public static final class Chapter {
        public String title;
        public int level;
        public final List<Block> blocks = new ArrayList<>();
        public int startRow;
    }

    public static final class Book {
        public String title = "";
        public String author = "";
        public final List<Chapter> chapters = new ArrayList<>();
        public final List<ImageAsset> images = new ArrayList<>();
        public int coverIndex = -1;
    }

    private static final class TocItem {
        String title;
        String path;
        int level;
    }

    private static final class Item {
        String id;
        String href;
        String mediaType;
        String properties;
    }

    private static final long MAX_ENTRY = 64L * 1024 * 1024;
    private static final Pattern XML_ENCODING = Pattern.compile("(?i)<\\?xml[^>]*encoding\\s*=\\s*[\"']([^\"']+)");
    private static final Pattern META_CHARSET = Pattern.compile("(?i)<meta[^>]+charset\\s*=\\s*[\"']?\\s*([a-zA-Z0-9._:-]+)");
    private static final Pattern H1 = Pattern.compile("(?is)<h[1-6][^>]*>(.*?)</h[1-6]>");
    private static final Pattern TITLE_TAG = Pattern.compile("(?is)<title[^>]*>(.*?)</title>");
    private static final Pattern IMG_TAG = Pattern.compile("(?is)<img\\b([^>]*)>");
    private static final Pattern ALT_ATTR = Pattern.compile("(?is)alt\\s*=\\s*[\"']([^\"']*)[\"']");
    private static final Pattern SRC_ATTR = Pattern.compile("(?is)src\\s*=\\s*[\"']([^\"']*)[\"']");
    private static final Pattern HREF_ATTR = Pattern.compile("(?is)href\\s*=\\s*[\"']([^\"']+)[\"']");
    private static final Pattern ANCHOR = Pattern.compile("(?is)<a\\b([^>]*)>(.*?)</a>");
    private static final Pattern NUM_ENTITY = Pattern.compile("&#(x?)([0-9a-fA-F]+);");
    private static final Pattern NAMED_ENTITY = Pattern.compile("&([a-zA-Z][a-zA-Z0-9]{1,31});");

    private Epub() {}

    public static Book parse(File file) throws Exception {
        ZipFile zip;
        try {
            zip = new ZipFile(file);
        } catch (Exception e) {
            throw new IOException("这不是一个有效的 EPUB/ZIP 文件");
        }
        try {
            Map<String, ZipEntry> entries = new LinkedHashMap<>();
            Map<String, String> lower = new HashMap<>();
            for (ZipEntry ze : java.util.Collections.list(zip.entries())) {
                if (ze.isDirectory()) continue;
                String name = normalizePath(ze.getName());
                if (name.isEmpty()) continue;
                entries.put(name, ze);
                lower.put(name.toLowerCase(Locale.ROOT), name);
            }
            ZipReader reader = new ZipReader(zip, entries, lower);
            byte[] enc = reader.readOrNull("META-INF/encryption.xml");
            if (enc != null) {
                String txt = new String(enc, StandardCharsets.UTF_8).toLowerCase(Locale.ROOT);
                if (txt.contains("encrypteddata") || txt.contains("encryptedkey")) {
                    throw new IOException("这本 EPUB 含有 DRM/加密内容，暂不支持");
                }
            }

            byte[] containerRaw = reader.read("META-INF/container.xml");
            Document container = parseXml(decode(containerRaw));
            Element rootfile = firstByLocal(container, "rootfile");
            if (rootfile == null) throw new IOException("EPUB 缺少 OPF 文件声明");
            String opfPath = normalizePath(rootfile.getAttribute("full-path"));
            if (opfPath.isEmpty()) throw new IOException("EPUB 缺少 OPF 文件声明");

            byte[] opfRaw = reader.read(opfPath);
            Document opf = parseXml(decode(opfRaw));

            Book book = new Book();
            Element metadata = firstByLocal(opf, "metadata");
            if (metadata != null) {
                book.title = cleanText(textOf(firstByLocal(metadata, "title")));
                StringBuilder authors = new StringBuilder();
                for (Element e : descendants(metadata, "creator")) {
                    String a = cleanText(textOf(e));
                    if (a.isEmpty()) continue;
                    if (authors.length() > 0) authors.append("、");
                    authors.append(a);
                }
                book.author = authors.toString();
            }
            if (book.title.isEmpty()) {
                String n = file.getName();
                int dot = n.lastIndexOf('.');
                book.title = dot > 0 ? n.substring(0, dot) : n;
            }
            if (book.author.isEmpty()) book.author = "佚名";

            Map<String, Item> items = new HashMap<>();
            Element manifest = firstByLocal(opf, "manifest");
            if (manifest != null) {
                for (Element e : descendants(manifest, "item")) {
                    Item it = new Item();
                    it.id = e.getAttribute("id");
                    it.href = resolve(opfPath, e.getAttribute("href"), false);
                    it.mediaType = e.getAttribute("media-type");
                    it.properties = e.getAttribute("properties");
                    if (!it.id.isEmpty()) items.put(it.id, it);
                }
            }

            Element spine = firstByLocal(opf, "spine");
            if (spine == null) throw new IOException("EPUB 没有可阅读的章节（spine 为空）");
            String spineToc = spine.getAttribute("toc");

            Item navItem = null;
            Item ncxItem = null;
            for (Item it : items.values()) {
                String props = it.properties == null ? "" : it.properties.toLowerCase(Locale.ROOT);
                if (props.contains("nav") && navItem == null) navItem = it;
                if ("application/x-dtbncx+xml".equalsIgnoreCase(it.mediaType)
                        || it.href.toLowerCase(Locale.ROOT).endsWith(".ncx")) {
                    if (ncxItem == null) ncxItem = it;
                }
            }

            List<TocItem> toc = new ArrayList<>();
            if (navItem != null) {
                byte[] raw = reader.readOrNull(navItem.href);
                if (raw != null) toc = parseNav(decode(raw), navItem.href);
            }
            if (toc.isEmpty()) {
                String ncxPath = null;
                if (spineToc != null && !spineToc.isEmpty()) {
                    Item it = items.get(spineToc);
                    if (it != null) ncxPath = it.href;
                }
                if (ncxPath == null && ncxItem != null) ncxPath = ncxItem.href;
                if (ncxPath != null) {
                    byte[] raw = reader.readOrNull(ncxPath);
                    if (raw != null) toc = parseNcx(decode(raw), ncxPath);
                }
            }
            Map<String, TocItem> tocByPath = new HashMap<>();
            for (TocItem t : toc) {
                if (t.path == null || t.path.isEmpty()) continue;
                if (!tocByPath.containsKey(t.path)) tocByPath.put(t.path, t);
            }

            Map<String, Item> itemsByHref = new HashMap<>();
            for (Item it : items.values()) {
                if (it.href != null) itemsByHref.put(it.href, it);
            }

            List<TocItem> spineRefs = new ArrayList<>();
            for (Element e : descendants(spine, "itemref")) {
                TocItem r = new TocItem();
                r.path = e.getAttribute("idref");
                r.level = "no".equalsIgnoreCase(e.getAttribute("linear")) ? -1 : 0;
                spineRefs.add(r);
            }
            if (spineRefs.isEmpty()) throw new IOException("EPUB 没有可阅读的章节（spine 为空）");

            String coverHref = "";
            if (metadata != null) {
                for (Element e : descendants(metadata, "meta")) {
                    if (!"cover".equalsIgnoreCase(e.getAttribute("name").trim())) continue;
                    Item it = items.get(e.getAttribute("content").trim());
                    if (it != null && isImagePath(it.href)) { coverHref = it.href; break; }
                }
            }
            if (coverHref.isEmpty()) {
                for (Item it : items.values()) {
                    String props = it.properties == null ? "" : it.properties.toLowerCase(Locale.ROOT);
                    if (props.contains("cover-image") && isImagePath(it.href)) { coverHref = it.href; break; }
                }
            }
            if (coverHref.isEmpty()) {
                Element guide = firstByLocal(opf, "guide");
                if (guide != null) {
                    for (Element e : descendants(guide, "reference")) {
                        String type = e.getAttribute("type").toLowerCase(Locale.ROOT);
                        if (!type.contains("cover")) continue;
                        String href = resolve(opfPath, e.getAttribute("href"), true);
                        if (isImagePath(href)) { coverHref = href; break; }
                        byte[] raw = reader.readOrNull(href);
                        if (raw != null) {
                            String img = firstImageInHtml(decode(raw), href);
                            if (!img.isEmpty()) { coverHref = img; break; }
                        }
                    }
                }
            }
            Map<String, Integer> imageByHref = new HashMap<>();
            long[] imageBytes = new long[]{0};
            if (!coverHref.isEmpty()) {
                book.coverIndex = loadImage(reader, itemsByHref, book, imageByHref, coverHref, "封面", imageBytes);
            }

            Map<String, Boolean> seen = new HashMap<>();
            int chapterNo = 0;
            for (TocItem ref : spineRefs) {
                if (ref.level < 0) continue;
                Item it = items.get(ref.path);
                if (it == null) continue;
                if (it.properties != null && it.properties.toLowerCase(Locale.ROOT).contains("nav")) continue;
                if (seen.containsKey(it.href)) continue;
                seen.put(it.href, Boolean.TRUE);
                byte[] raw = reader.readOrNull(it.href);
                if (raw == null) continue;
                ParsedContent pc = parseContent(decode(raw));
                if (pc.blocks.isEmpty()) continue;
                for (int bi = 0; bi < pc.blocks.size(); bi++) {
                    Block b = pc.blocks.get(bi);
                    if (b.type != BLOCK_IMAGE) continue;
                    String src = resolve(it.href, b.imageSrc, true);
                    int idx = loadImage(reader, itemsByHref, book, imageByHref, src, b.imageAlt, imageBytes);
                    if (idx < 0) {
                        pc.blocks.set(bi, new Block(BLOCK_BODY, "〔插图无法显示〕"));
                    } else {
                        pc.blocks.set(bi, new Block(BLOCK_IMAGE, imagePlaceholder(idx, b.imageAlt), b.imageSrc, b.imageAlt, idx));
                    }
                }

                chapterNo++;
                TocItem t = tocByPath.get(it.href);
                String title = t == null ? "" : t.title;
                if (title == null || title.trim().isEmpty()) title = pc.heading;
                if (title == null || title.trim().isEmpty()) title = pc.docTitle;
                if (title == null || title.trim().isEmpty()) title = "第 " + chapterNo + " 章";
                title = cleanText(title);

                if (!pc.blocks.isEmpty() && pc.blocks.get(0).type == BLOCK_HEADING) {
                    String first = normalizeForCompare(pc.blocks.get(0).text);
                    String cmp = normalizeForCompare(title);
                    if (!first.isEmpty() && (first.equals(cmp) || first.contains(cmp) || cmp.contains(first))) {
                        pc.blocks.remove(0);
                    }
                }
                if (pc.blocks.isEmpty()) {
                    pc.blocks.add(new Block(BLOCK_BODY, title));
                }
                Chapter ch = new Chapter();
                ch.title = title;
                ch.level = t == null ? 0 : Math.max(0, t.level);
                ch.blocks.addAll(pc.blocks);
                book.chapters.add(ch);
            }
            if (book.chapters.isEmpty()) {
                throw new IOException("没有找到可显示的正文（EPUB 可能使用了不支持的版式）");
            }
            return book;
        } finally {
            try { zip.close(); } catch (IOException ignored) {}
        }
    }

    private static final class ParsedContent {
        final List<Block> blocks = new ArrayList<>();
        final List<ImageRef> images = new ArrayList<>();
        String heading = "";
        String docTitle = "";
    }

    private static ParsedContent parseContent(String html) {
        ParsedContent out = new ParsedContent();
        Matcher tm = TITLE_TAG.matcher(html);
        if (tm.find()) out.docTitle = cleanText(stripTags(tm.group(1)));
        Matcher hm = H1.matcher(html);
        if (hm.find()) out.heading = cleanText(stripTags(hm.group(1)));

        String body = html;
        Matcher bm = Pattern.compile("(?is)<body\\b[^>]*>(.*?)</body>").matcher(html);
        if (bm.find()) body = bm.group(1);
        body = body.replaceAll("(?is)<head\\b.*?</head>", " ");
        body = body.replaceAll("(?is)<(script|style|svg|nav|header|footer)\\b.*?</\\1>", " ");
        body = body.replaceAll("(?is)<!--.*?-->", " ");
        body = replaceImages(body, out.images);

        body = body.replaceAll("(?is)<h[1-6]\\b[^>]*>", "\u0000\u0003");
        body = body.replaceAll("(?is)</h[1-6]>", "\u0000");
        body = body.replaceAll("(?is)<blockquote\\b[^>]*>", "\u0000\u0004");
        body = body.replaceAll("(?is)</blockquote>", "\u0000");
        body = body.replaceAll("(?is)<li\\b[^>]*>", "\u0000· ");
        body = body.replaceAll("(?is)</li>", "\u0000");
        body = body.replaceAll("(?is)</(p|div|section|article|tr|dd|dt|figcaption)>", "\u0000");
        body = body.replaceAll("(?is)<hr\\b[^>]*>", "\u0000\u0005\u0000");
        body = body.replaceAll("(?is)<br\\s*/?>", "\u0001");
        body = body.replaceAll("(?is)<[^>]+>", "");
        body = decodeEntities(body);

        String[] chunks = body.split("\u0000", -1);
        for (String chunk : chunks) {
            if (chunk == null) continue;
            if (chunk.startsWith("\u0006")) {
                int end = chunk.indexOf('\u0007');
                if (end > 1) {
                    try {
                        int ref = Integer.parseInt(chunk.substring(1, end));
                        if (ref >= 0 && ref < out.images.size()) {
                            ImageRef ir = out.images.get(ref);
                            out.blocks.add(new Block(BLOCK_IMAGE, "〔" + ir.alt + "〕", ir.src, ir.alt, -1));
                        }
                    } catch (NumberFormatException ignored) {}
                }
                continue;
            }
            int type = BLOCK_BODY;
            if (chunk.startsWith("\u0003")) { type = BLOCK_HEADING; chunk = chunk.substring(1); }
            else if (chunk.startsWith("\u0004")) { type = BLOCK_QUOTE; chunk = chunk.substring(1); }
            else if (chunk.startsWith("\u0005")) { type = BLOCK_DIVIDER; chunk = chunk.substring(1); }
            String text = normalizeText(chunk);
            if (text.isEmpty()) continue;
            if (type == BLOCK_DIVIDER) {
                out.blocks.add(new Block(BLOCK_DIVIDER, "· · ·"));
            } else {
                out.blocks.add(new Block(type, text));
            }
        }
        return out;
    }

    private static String replaceImages(String body, List<ImageRef> refs) {
        Matcher m = IMG_TAG.matcher(body);
        StringBuffer sb = new StringBuffer();
        while (m.find()) {
            String attrs = m.group(1) == null ? "" : m.group(1);
            Matcher am = ALT_ATTR.matcher(attrs);
            String alt = am.find() ? cleanText(decodeEntities(am.group(1))) : "";
            if (alt.isEmpty()) alt = "插图";
            Matcher sm = SRC_ATTR.matcher(attrs);
            String src = sm.find() ? decodeEntities(sm.group(1)).trim() : "";
            refs.add(new ImageRef(src, alt));
            m.appendReplacement(sb, Matcher.quoteReplacement("\u0000\u0006" + (refs.size() - 1) + "\u0007\u0000"));
        }
        m.appendTail(sb);
        return sb.toString();
    }


    private static int loadImage(ZipReader reader, Map<String, Item> itemsByHref, Book book,
                                 Map<String, Integer> indexByHref, String href, String alt,
                                 long[] imageBytes) {
        href = normalizePath(href);
        if (href.isEmpty()) return -1;
        Integer cached = indexByHref.get(href);
        if (cached != null) return cached;
        byte[] data = reader.readOrNull(href);
        if (data == null || data.length == 0 || data.length > 32L * 1024 * 1024) return -1;
        if (imageBytes[0] + data.length > 96L * 1024 * 1024) return -1;
        Item item = itemsByHref.get(href);
        String media = item == null ? "" : item.mediaType;
        media = mediaTypeForImage(href, media);
        if (alt == null || alt.trim().isEmpty()) alt = "插图";
        int index = book.images.size();
        book.images.add(new ImageAsset(href, media, alt, data));
        imageBytes[0] += data.length;
        indexByHref.put(href, index);
        return index;
    }

    private static boolean isImagePath(String href) {
        if (href == null) return false;
        String h = href.toLowerCase(Locale.ROOT);
        return h.endsWith(".jpg") || h.endsWith(".jpeg") || h.endsWith(".png")
                || h.endsWith(".gif") || h.endsWith(".webp") || h.endsWith(".bmp")
                || h.endsWith(".avif");
    }

    private static String mediaTypeForImage(String href, String declared) {
        if (declared != null) {
            String d = declared.trim().toLowerCase(Locale.ROOT);
            if (d.startsWith("image/")) return d;
        }
        String h = href == null ? "" : href.toLowerCase(Locale.ROOT);
        if (h.endsWith(".jpg") || h.endsWith(".jpeg")) return "image/jpeg";
        if (h.endsWith(".png")) return "image/png";
        if (h.endsWith(".gif")) return "image/gif";
        if (h.endsWith(".webp")) return "image/webp";
        if (h.endsWith(".bmp")) return "image/bmp";
        return "application/octet-stream";
    }

    private static String firstImageInHtml(String html, String baseFile) {
        Matcher m = IMG_TAG.matcher(html);
        if (!m.find()) return "";
        Matcher sm = SRC_ATTR.matcher(m.group(1));
        if (!sm.find()) return "";
        return resolve(baseFile, decodeEntities(sm.group(1)), true);
    }

    private static String imagePlaceholder(int index, String alt) {
        alt = cleanText(alt);
        if (alt.isEmpty() || "插图".equals(alt)) return "〔插图 " + (index + 1) + "〕";
        if (alt.length() > 16) alt = alt.substring(0, 16) + "…";
        return "〔插图 " + (index + 1) + " · " + alt + "〕";
    }

    private static String normalizeText(String s) {
        s = s.replace('\r', ' ').replace('\n', ' ');
        s = s.replace('\u0001', '\n');
        s = s.replace('\u00A0', ' ');
        s = s.replaceAll("[ \\t\\f]+", " ");
        s = s.replaceAll(" {2,}", " ");
        s = s.replaceAll("(?<=[\\u4e00-\\u9fff])[ \\t]+(?=[\\u4e00-\\u9fff])", "");
        s = s.replaceAll("[ \\t]+(?=[，。！？；：、）》」』】…—])", "");
        s = s.replaceAll("(?<=[（《「『【“‘])[ \\t]+", "");
        s = s.replaceAll("\\n{3,}", "\n\n");
        s = s.replaceAll(" *\\n *", "\n");
        return s.trim();
    }

    private static String cleanText(String s) {
        if (s == null) return "";
        s = s.replace('\u00A0', ' ').replace('\u3000', ' ');
        s = s.replaceAll("[\\r\\n\\t\\f]+", " ");
        s = s.replaceAll(" {2,}", " ");
        return s.trim();
    }

    private static String stripTags(String s) {
        return decodeEntities(s.replaceAll("(?is)<[^>]+>", " "));
    }

    private static String decodeEntities(String s) {
        Matcher m = NUM_ENTITY.matcher(s);
        StringBuffer sb = new StringBuffer();
        while (m.find()) {
            try {
                int cp = Integer.parseInt(m.group(2), m.group(1).isEmpty() ? 10 : 16);
                if (cp >= 0 && cp <= 0x10FFFF) {
                    m.appendReplacement(sb, Matcher.quoteReplacement(new String(Character.toChars(cp))));
                } else {
                    m.appendReplacement(sb, "");
                }
            } catch (Exception e) {
                m.appendReplacement(sb, "");
            }
        }
        m.appendTail(sb);
        s = sb.toString();
        String[][] named = {
                {"amp", "&"}, {"lt", "<"}, {"gt", ">"}, {"quot", "\""}, {"apos", "'"},
                {"nbsp", " "}, {"mdash", "—"}, {"ndash", "–"}, {"hellip", "…"},
                {"ldquo", "“"}, {"rdquo", "”"}, {"lsquo", "‘"}, {"rsquo", "’"},
                {"middot", "·"}, {"bull", "•"}, {"copy", "©"}, {"reg", "®"}, {"trade", "™"}
        };
        for (String[] pair : named) s = s.replace("&" + pair[0] + ";", pair[1]);
        Matcher nm = NAMED_ENTITY.matcher(s);
        sb = new StringBuffer();
        while (nm.find()) nm.appendReplacement(sb, "");
        nm.appendTail(sb);
        return sb.toString();
    }

    private static final class ZipReader {
        final ZipFile zip;
        final Map<String, ZipEntry> entries;
        final Map<String, String> lower;
        ZipReader(ZipFile zip, Map<String, ZipEntry> entries, Map<String, String> lower) {
            this.zip = zip; this.entries = entries; this.lower = lower;
        }
        ZipEntry find(String name) {
            String n = normalizePath(name);
            ZipEntry e = entries.get(n);
            if (e != null) return e;
            String real = lower.get(n.toLowerCase(Locale.ROOT));
            return real == null ? null : entries.get(real);
        }
        byte[] read(String name) throws IOException {
            byte[] b = readOrNull(name);
            if (b == null) throw new IOException("EPUB 中缺少文件: " + name);
            return b;
        }
        byte[] readOrNull(String name) {
            ZipEntry e = find(name);
            if (e == null) return null;
            if (e.getSize() > MAX_ENTRY) return null;
            try (InputStream in = zip.getInputStream(e)) {
                ByteArrayOutputStream out = new ByteArrayOutputStream();
                byte[] buf = new byte[16 * 1024];
                long total = 0;
                int n;
                while ((n = in.read(buf)) > 0) {
                    total += n;
                    if (total > MAX_ENTRY) return null;
                    out.write(buf, 0, n);
                }
                return out.toByteArray();
            } catch (IOException e2) {
                return null;
            }
        }
    }

    private static Document parseXml(String text) throws Exception {
        String s = sanitizeXml(text);
        DocumentBuilderFactory f = DocumentBuilderFactory.newInstance();
        f.setNamespaceAware(true);
        f.setExpandEntityReferences(false);
        try {
            f.setFeature("http://apache.org/xml/features/nonvalidating/load-external-dtd", false);
            f.setFeature("http://xml.org/sax/features/external-general-entities", false);
            f.setFeature("http://xml.org/sax/features/external-parameter-entities", false);
        } catch (Exception ignored) {}
        DocumentBuilder b = f.newDocumentBuilder();
        return b.parse(new InputSource(new StringReader(s)));
    }

    private static String sanitizeXml(String s) {
        s = s.replace("&nbsp;", "&#160;")
                .replace("&mdash;", "&#8212;")
                .replace("&ndash;", "&#8211;")
                .replace("&hellip;", "&#8230;")
                .replace("&ldquo;", "&#8220;")
                .replace("&rdquo;", "&#8221;")
                .replace("&lsquo;", "&#8216;")
                .replace("&rsquo;", "&#8217;")
                .replace("&middot;", "&#183;");
        StringBuilder sb = new StringBuilder(s.length());
        for (int i = 0; i < s.length(); i++) {
            char c = s.charAt(i);
            if (c == '\t' || c == '\n' || c == '\r' || c >= 0x20) sb.append(c);
        }
        return sb.toString();
    }

    private static Element firstByLocal(Document doc, String local) {
        NodeList list = doc.getElementsByTagNameNS("*", local);
        return list.getLength() == 0 ? null : (Element) list.item(0);
    }

    private static Element firstByLocal(Element root, String local) {
        if (root == null) return null;
        NodeList list = root.getElementsByTagNameNS("*", local);
        return list.getLength() == 0 ? null : (Element) list.item(0);
    }

    private static List<Element> descendants(Element root, String local) {
        List<Element> out = new ArrayList<>();
        if (root == null) return out;
        NodeList list = root.getElementsByTagNameNS("*", local);
        for (int i = 0; i < list.getLength(); i++) out.add((Element) list.item(i));
        return out;
    }

    private static String textOf(Element e) {
        return e == null ? "" : e.getTextContent();
    }

    private static String localName(Node n) {
        String s = n.getLocalName();
        if (s == null) s = n.getNodeName();
        int colon = s.indexOf(':');
        return colon >= 0 ? s.substring(colon + 1) : s;
    }

    private static List<TocItem> parseNav(String xhtml, String navPath) {
        List<TocItem> out = new ArrayList<>();
        try {
            Document doc = parseXml(xhtml);
            NodeList navs = doc.getElementsByTagNameNS("*", "nav");
            Element nav = null;
            for (int i = 0; i < navs.getLength(); i++) {
                Element e = (Element) navs.item(i);
                String type = e.getAttributeNS("http://www.idpf.org/2007/ops", "type");
                if (type == null || type.isEmpty()) type = e.getAttribute("epub:type");
                if (type == null || type.isEmpty()) type = e.getAttribute("type");
                if (type != null && type.toLowerCase(Locale.ROOT).contains("toc")) { nav = e; break; }
                if (nav == null) nav = e;
            }
            if (nav != null) walkNav(nav, 0, navPath, out);
            if (!out.isEmpty()) return out;
        } catch (Exception ignored) {}
        // Fallback for slightly malformed XHTML.
        Matcher nm = Pattern.compile("(?is)<nav\\b[^>]*>(.*?)</nav>").matcher(xhtml);
        if (nm.find()) {
            Matcher am = ANCHOR.matcher(nm.group(1));
            while (am.find()) {
                Matcher hm = HREF_ATTR.matcher(am.group(1));
                if (!hm.find()) continue;
                String title = cleanText(stripTags(am.group(2)));
                if (title.isEmpty()) continue;
                TocItem t = new TocItem();
                t.title = title;
                t.path = resolve(navPath, hm.group(1), true);
                t.level = 0;
                out.add(t);
            }
        }
        return out;
    }

    private static void walkNav(Node n, int depth, String navPath, List<TocItem> out) {
        for (Node c = n.getFirstChild(); c != null; c = c.getNextSibling()) {
            if (c.getNodeType() != Node.ELEMENT_NODE) continue;
            String name = localName(c).toLowerCase(Locale.ROOT);
            if ("a".equals(name)) {
                String href = ((Element) c).getAttribute("href");
                String title = cleanText(c.getTextContent());
                if (!title.isEmpty() && !href.isEmpty()) {
                    TocItem t = new TocItem();
                    t.title = title;
                    t.path = resolve(navPath, href, true);
                    t.level = Math.max(0, depth - 1);
                    out.add(t);
                }
            } else if ("ol".equals(name) || "ul".equals(name)) {
                walkNav(c, depth + 1, navPath, out);
            } else {
                walkNav(c, depth, navPath, out);
            }
        }
    }

    private static List<TocItem> parseNcx(String xml, String ncxPath) {
        List<TocItem> out = new ArrayList<>();
        try {
            Document doc = parseXml(xml);
            Element navMap = firstByLocal(doc, "navMap");
            if (navMap != null) walkNcx(navMap, 0, ncxPath, out);
        } catch (Exception ignored) {}
        return out;
    }

    private static void walkNcx(Node parent, int level, String ncxPath, List<TocItem> out) {
        for (Node c = parent.getFirstChild(); c != null; c = c.getNextSibling()) {
            if (c.getNodeType() != Node.ELEMENT_NODE) continue;
            if (!"navpoint".equals(localName(c).toLowerCase(Locale.ROOT))) continue;
            Element point = (Element) c;
            Element label = firstByLocal(point, "navLabel");
            Element text = label == null ? null : firstByLocal(label, "text");
            Element content = firstByLocal(point, "content");
            String title = cleanText(text == null ? "" : text.getTextContent());
            String src = content == null ? "" : content.getAttribute("src");
            if (!title.isEmpty() && !src.isEmpty()) {
                TocItem t = new TocItem();
                t.title = title;
                t.path = resolve(ncxPath, src, true);
                t.level = level;
                out.add(t);
            }
            walkNcx(point, level + 1, ncxPath, out);
        }
    }

    private static String resolve(String basePath, String href, boolean dropFragment) {
        if (href == null) return "";
        href = href.trim();
        if (href.isEmpty()) return "";
        if (dropFragment) {
            int h = href.indexOf('#');
            if (h >= 0) href = href.substring(0, h);
            int q = href.indexOf('?');
            if (q >= 0) href = href.substring(0, q);
        }
        try {
            href = URLDecoder.decode(href.replace("+", "%2B"), "UTF-8");
        } catch (Exception ignored) {}
        href = href.replace('\\', '/');
        if (href.startsWith("/")) href = href.substring(1);
        String dir = "";
        int slash = basePath.lastIndexOf('/');
        if (slash >= 0) dir = basePath.substring(0, slash);
        return normalizePath(dir + "/" + href);
    }

    private static String normalizePath(String p) {
        if (p == null) return "";
        p = p.replace('\\', '/');
        Deque<String> stack = new ArrayDeque<>();
        for (String part : p.split("/")) {
            if (part.isEmpty() || ".".equals(part)) continue;
            if ("..".equals(part)) {
                if (!stack.isEmpty()) stack.removeLast();
            } else {
                stack.addLast(part);
            }
        }
        if (stack.isEmpty()) return "";
        StringBuilder sb = new StringBuilder();
        for (String part : stack) {
            if (sb.length() > 0) sb.append('/');
            sb.append(part);
        }
        return sb.toString();
    }

    private static String decode(byte[] data) {
        if (data.length >= 2) {
            if ((data[0] & 0xFF) == 0xFF && (data[1] & 0xFF) == 0xFE) {
                return new String(data, 2, data.length - 2, StandardCharsets.UTF_16LE);
            }
            if ((data[0] & 0xFF) == 0xFE && (data[1] & 0xFF) == 0xFF) {
                return new String(data, 2, data.length - 2, StandardCharsets.UTF_16BE);
            }
        }
        if (data.length >= 3 && (data[0] & 0xFF) == 0xEF && (data[1] & 0xFF) == 0xBB && (data[2] & 0xFF) == 0xBF) {
            return new String(data, 3, data.length - 3, StandardCharsets.UTF_8);
        }
        String head = new String(data, 0, Math.min(data.length, 2048), StandardCharsets.ISO_8859_1);
        String label = "";
        Matcher m = XML_ENCODING.matcher(head);
        if (m.find()) label = m.group(1);
        else {
            m = META_CHARSET.matcher(head);
            if (m.find()) label = m.group(1);
        }
        if (label == null || label.trim().isEmpty()) return new String(data, StandardCharsets.UTF_8);
        label = label.trim();
        if (label.equalsIgnoreCase("utf-8") || label.equalsIgnoreCase("utf8")) {
            return new String(data, StandardCharsets.UTF_8);
        }
        if (label.equalsIgnoreCase("utf-16") || label.equalsIgnoreCase("utf-16le")) {
            return new String(data, StandardCharsets.UTF_16LE);
        }
        if (label.equalsIgnoreCase("utf-16be")) {
            return new String(data, StandardCharsets.UTF_16BE);
        }
        try {
            return new String(data, Charset.forName(label));
        } catch (Exception e) {
            return new String(data, StandardCharsets.UTF_8);
        }
    }

    private static String normalizeForCompare(String s) {
        if (s == null) return "";
        StringBuilder sb = new StringBuilder();
        for (int i = 0; i < s.length(); i++) {
            char c = s.charAt(i);
            if (Character.isLetterOrDigit(c)) sb.append(Character.toLowerCase(c));
        }
        return sb.toString();
    }
}
