package main

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"regexp"
	"strings"
	"unicode/utf16"

	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/transform"
)

// ---------------------------------------------------------------------------
// Public book model
// ---------------------------------------------------------------------------

type BlockKind int

const (
	KindBody BlockKind = iota
	KindTitlePage
	KindAuthor
	KindMeta
	KindChapterTitle
	KindHeading1
	KindHeading2
	KindHeading3
	KindQuote
	KindDivider
	KindImage
)

type Align int

const (
	AlignDefault Align = iota
	AlignCenter
)

type Run struct {
	Text   string `json:"text"`
	Bold   bool   `json:"bold,omitempty"`
	Italic bool   `json:"italic,omitempty"`
}

type Block struct {
	Runs       []Run     `json:"runs"`
	Kind       BlockKind `json:"kind"`
	Align      Align     `json:"align,omitempty"`
	ImageSrc   string    `json:"-"`
	ImageAlt   string    `json:"-"`
	ImageIndex int       `json:"-"`
}

func (b Block) Text() string {
	var sb strings.Builder
	for _, r := range b.Runs {
		sb.WriteString(r.Text)
	}
	return sb.String()
}

type Chapter struct {
	Index  int     `json:"index"`
	Title  string  `json:"title"`
	Level  int     `json:"level"`
	Href   string  `json:"href"`
	Blocks []Block `json:"blocks"`
	// Start and End are UTF-16 offsets into the rendered RichEdit document.
	Start int32 `json:"-"`
	End   int32 `json:"-"`
}

type ImageAsset struct {
	Href      string
	MediaType string
	Alt       string
	Data      []byte
}

type Book struct {
	Path       string
	Title      string
	Author     string
	Language   string
	Chapters   []Chapter
	CoverHref  string
	CoverIndex int
	Images     []ImageAsset
}

// ---------------------------------------------------------------------------
// EPUB container
// ---------------------------------------------------------------------------

type stringPair struct{ a, b string }

type epubArchive struct {
	zr      *zip.ReadCloser
	files   map[string]*zip.File
	lower   map[string]string
	total   uint64
	readMax uint64
}

func openEpubArchive(name string) (*epubArchive, error) {
	zr, err := zip.OpenReader(name)
	if err != nil {
		if errors.Is(err, zip.ErrFormat) {
			return nil, errors.New("这不是一个有效的 EPUB/ZIP 文件")
		}
		return nil, fmt.Errorf("无法打开文件: %w", err)
	}
	a := &epubArchive{zr: zr, files: make(map[string]*zip.File), lower: make(map[string]string), readMax: 128 << 20}
	for _, f := range zr.File {
		p := normalizeZipPath(f.Name)
		if p == "" || strings.HasSuffix(p, "/") {
			continue
		}
		a.total += f.UncompressedSize64
		a.files[p] = f
		a.lower[strings.ToLower(p)] = p
	}
	if a.total > 512<<20 {
		zr.Close()
		return nil, errors.New("EPUB 解压后体积过大，已停止读取")
	}
	return a, nil
}

func (a *epubArchive) Close() error {
	if a.zr != nil {
		return a.zr.Close()
	}
	return nil
}

func normalizeZipPath(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	p = strings.TrimPrefix(p, "/")
	p = path.Clean(p)
	if p == "." || p == ".." || strings.HasPrefix(p, "../") {
		return ""
	}
	return p
}

func (a *epubArchive) find(name string) (*zip.File, string, bool) {
	name = normalizeZipPath(name)
	if f, ok := a.files[name]; ok {
		return f, name, true
	}
	if real, ok := a.lower[strings.ToLower(name)]; ok {
		return a.files[real], real, true
	}
	return nil, "", false
}

func (a *epubArchive) read(name string) ([]byte, error) {
	f, _, ok := a.find(name)
	if !ok {
		return nil, fmt.Errorf("EPUB 中缺少文件: %s", name)
	}
	if f.UncompressedSize64 > a.readMax {
		return nil, fmt.Errorf("文件 %s 过大", name)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, io.LimitReader(rc, int64(a.readMax)+1)); err != nil {
		return nil, err
	}
	if int64(buf.Len()) > int64(a.readMax) {
		return nil, fmt.Errorf("文件 %s 超过读取上限", name)
	}
	return buf.Bytes(), nil
}

// ---------------------------------------------------------------------------
// Charset decoding
// ---------------------------------------------------------------------------

var xmlEncodingRe = regexp.MustCompile(`(?i)<\?xml[^>]*encoding\s*=\s*["']([^"']+)["']`)
var htmlCharsetRe = regexp.MustCompile(`(?i)<meta[^>]+charset\s*=\s*["']?\s*([a-zA-Z0-9._:-]+)`)

func decodeMarkup(raw []byte) (string, error) {
	if len(raw) >= 2 {
		if raw[0] == 0xFF && raw[1] == 0xFE {
			return decodeUTF16(raw[2:], binary.LittleEndian), nil
		}
		if raw[0] == 0xFE && raw[1] == 0xFF {
			return decodeUTF16(raw[2:], binary.BigEndian), nil
		}
	}
	if bytes.HasPrefix(raw, []byte{0xEF, 0xBB, 0xBF}) {
		return string(raw[3:]), nil
	}
	label := ""
	if m := xmlEncodingRe.FindSubmatch(raw); len(m) == 2 {
		label = string(m[1])
	} else if m := htmlCharsetRe.FindSubmatch(raw); len(m) == 2 {
		label = string(m[1])
	}
	if label == "" || strings.EqualFold(label, "utf-8") || strings.EqualFold(label, "utf8") {
		return string(raw), nil
	}
	if strings.EqualFold(label, "utf-16") || strings.EqualFold(label, "utf-16le") {
		return decodeUTF16(raw, binary.LittleEndian), nil
	}
	if strings.EqualFold(label, "utf-16be") {
		return decodeUTF16(raw, binary.BigEndian), nil
	}
	enc, err := htmlindex.Get(label)
	if err != nil {
		// Unknown label: UTF-8 is the most useful fallback.
		return string(raw), nil
	}
	out, _, err := transform.Bytes(enc.NewDecoder(), raw)
	if err != nil {
		return "", fmt.Errorf("无法按 %s 解码文本: %w", label, err)
	}
	return string(out), nil
}

func decodeUTF16(b []byte, order binary.ByteOrder) string {
	if len(b) < 2 {
		return string(b)
	}
	u := make([]uint16, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		u = append(u, order.Uint16(b[i:i+2]))
	}
	return string(utf16.Decode(u))
}

func sanitizeXML(s string) string {
	repl := strings.NewReplacer(
		"&nbsp;", "&#160;",
		"&mdash;", "&#8212;",
		"&ndash;", "&#8211;",
		"&hellip;", "&#8230;",
		"&ldquo;", "&#8220;",
		"&rdquo;", "&#8221;",
		"&lsquo;", "&#8216;",
		"&rsquo;", "&#8217;",
		"&middot;", "&#183;",
		"&bull;", "&#8226;",
		"&copy;", "&#169;",
		"&reg;", "&#174;",
		"&trade;", "&#8482;",
	)
	return repl.Replace(s)
}

// ---------------------------------------------------------------------------
// OPF / NCX / navigation
// ---------------------------------------------------------------------------

type opfPackage struct {
	Metadata struct {
		Title    string    `xml:"title"`
		Creators []string  `xml:"creator"`
		Language string    `xml:"language"`
		Meta     []opfMeta `xml:"meta"`
	} `xml:"metadata"`
	Guide struct {
		References []opfGuideRef `xml:"reference"`
	} `xml:"guide"`
	Manifest struct {
		Items []opfItem `xml:"item"`
	} `xml:"manifest"`
	Spine struct {
		Toc  string   `xml:"toc,attr"`
		Refs []opfRef `xml:"itemref"`
	} `xml:"spine"`
}

type opfItem struct {
	ID         string `xml:"id,attr"`
	Href       string `xml:"href,attr"`
	MediaType  string `xml:"media-type,attr"`
	Properties string `xml:"properties,attr"`
}

type opfRef struct {
	IDRef  string `xml:"idref,attr"`
	Linear string `xml:"linear,attr"`
}

type opfMeta struct {
	Name    string `xml:"name,attr"`
	Content string `xml:"content,attr"`
}

type opfGuideRef struct {
	Type string `xml:"type,attr"`
	Href string `xml:"href,attr"`
}

type tocItem struct {
	Title string
	Href  string
	Level int
}

func parseContainer(data []byte) (string, error) {
	var c struct {
		Rootfiles []struct {
			FullPath string `xml:"full-path,attr"`
		} `xml:"rootfiles>rootfile"`
	}
	if err := xml.Unmarshal([]byte(sanitizeXML(string(data))), &c); err != nil {
		return "", fmt.Errorf("container.xml 格式错误: %w", err)
	}
	if len(c.Rootfiles) == 0 || strings.TrimSpace(c.Rootfiles[0].FullPath) == "" {
		return "", errors.New("EPUB 缺少 OPF 文件声明")
	}
	return normalizeZipPath(c.Rootfiles[0].FullPath), nil
}

func parseOPF(data []byte) (*opfPackage, error) {
	var p opfPackage
	dec := xml.NewDecoder(bytes.NewReader([]byte(sanitizeXML(string(data)))))
	dec.Strict = false
	dec.AutoClose = xml.HTMLAutoClose
	dec.Entity = xml.HTMLEntity
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("OPF 格式错误: %w", err)
	}
	return &p, nil
}

func parseNCX(data []byte, ncxPath string) []tocItem {
	var ncx struct {
		NavMap struct {
			Points []ncxPoint `xml:"navPoint"`
		} `xml:"navMap"`
	}
	dec := xml.NewDecoder(bytes.NewReader([]byte(sanitizeXML(string(data)))))
	dec.Strict = false
	dec.AutoClose = xml.HTMLAutoClose
	dec.Entity = xml.HTMLEntity
	if err := dec.Decode(&ncx); err != nil {
		return nil
	}
	var out []tocItem
	var walk func(points []ncxPoint, level int)
	walk = func(points []ncxPoint, level int) {
		for _, p := range points {
			title := normalizeDisplayText(p.NavLabel.Text)
			if title != "" && p.Content.Src != "" {
				out = append(out, tocItem{Title: title, Href: resolvePath(ncxPath, p.Content.Src, true), Level: level})
			}
			walk(p.Points, level+1)
		}
	}
	walk(ncx.NavMap.Points, 0)
	return out
}

type ncxPoint struct {
	NavLabel struct {
		Text string `xml:"text"`
	} `xml:"navLabel"`
	Content struct {
		Src string `xml:"src,attr"`
	} `xml:"content"`
	Points []ncxPoint `xml:"navPoint"`
}

func parseNav(data []byte, navPath string) []tocItem {
	doc, err := parseHTMLDocument(data)
	if err != nil {
		return nil
	}
	body := findElement(doc, "body")
	if body == nil {
		body = doc
	}
	var nav *htmlNode
	var walkFind func(n *htmlNode)
	walkFind = func(n *htmlNode) {
		if nav != nil || n == nil {
			return
		}
		if n.Type == htmlElementNode && strings.EqualFold(n.Data, "nav") {
			typ := strings.ToLower(attrValue(n, "epub:type") + " " + attrValue(n, "type"))
			if strings.Contains(typ, "toc") || nav == nil {
				nav = n
				if strings.Contains(typ, "toc") {
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walkFind(c)
		}
	}
	walkFind(body)
	if nav == nil {
		return nil
	}
	var out []tocItem
	var walk func(n *htmlNode, depth int)
	walk = func(n *htmlNode, depth int) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == htmlElementNode && strings.EqualFold(c.Data, "a") {
				href := attrValue(c, "href")
				title := normalizeDisplayText(nodeText(c))
				level := depth - 1
				if level < 0 {
					level = 0
				}
				if href != "" && title != "" {
					out = append(out, tocItem{Title: title, Href: resolvePath(navPath, href, true), Level: level})
				}
			} else if c.Type == htmlElementNode && (strings.EqualFold(c.Data, "ol") || strings.EqualFold(c.Data, "ul")) {
				walk(c, depth+1)
			} else if c.Type == htmlElementNode {
				walk(c, depth)
			}
		}
	}
	walk(nav, 0)
	return out
}

// resolvePath resolves an EPUB href relative to the document that contains it.
// Fragments are dropped because chapters are addressed at file granularity.
func resolvePath(baseFile, href string, dropFragment bool) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	if u, err := url.Parse(href); err == nil {
		href = u.Path
	} else if i := strings.IndexAny(href, "#?"); i >= 0 && dropFragment {
		href = href[:i]
	}
	if unescaped, err := url.PathUnescape(href); err == nil {
		href = unescaped
	}
	href = strings.TrimPrefix(href, "/")
	return normalizeZipPath(path.Join(path.Dir(baseFile), href))
}

// ---------------------------------------------------------------------------
// LoadEPUB
// ---------------------------------------------------------------------------

func LoadEPUB(name string) (*Book, error) {
	a, err := openEpubArchive(name)
	if err != nil {
		return nil, err
	}
	defer a.Close()

	if enc, err := a.read("META-INF/encryption.xml"); err == nil {
		txt := strings.ToLower(string(enc))
		if strings.Contains(txt, "encrypteddata") || strings.Contains(txt, "encryptedkey") {
			return nil, errors.New("这本 EPUB 含有 DRM/加密内容，暂不支持")
		}
	}

	containerRaw, err := a.read("META-INF/container.xml")
	if err != nil {
		return nil, errors.New("EPUB 缺少 META-INF/container.xml")
	}
	containerText, err := decodeMarkup(containerRaw)
	if err != nil {
		return nil, err
	}
	opfPath, err := parseContainer([]byte(containerText))
	if err != nil {
		return nil, err
	}
	opfRaw, err := a.read(opfPath)
	if err != nil {
		return nil, err
	}
	opfText, err := decodeMarkup(opfRaw)
	if err != nil {
		return nil, err
	}
	opf, err := parseOPF([]byte(opfText))
	if err != nil {
		return nil, err
	}
	if len(opf.Spine.Refs) == 0 {
		return nil, errors.New("EPUB 没有可阅读的章节（spine 为空）")
	}

	items := make(map[string]opfItem, len(opf.Manifest.Items))
	var navItem, ncxItem *opfItem
	for i := range opf.Manifest.Items {
		it := opf.Manifest.Items[i]
		it.Href = resolvePath(opfPath, it.Href, false)
		items[it.ID] = it
		props := strings.ToLower(it.Properties)
		if strings.Contains(props, "nav") {
			x := it
			navItem = &x
		}
		if strings.EqualFold(it.MediaType, "application/x-dtbncx+xml") || strings.HasSuffix(strings.ToLower(it.Href), ".ncx") {
			x := it
			ncxItem = &x
		}
	}

	itemsByHref := make(map[string]opfItem, len(items))
	for _, it := range items {
		itemsByHref[it.Href] = it
	}

	var toc []tocItem
	if navItem != nil {
		if raw, err := a.read(navItem.Href); err == nil {
			if s, err := decodeMarkup(raw); err == nil {
				toc = parseNav([]byte(s), navItem.Href)
			}
		}
	}
	if len(toc) == 0 {
		ncxPath := ""
		if opf.Spine.Toc != "" {
			if it, ok := items[opf.Spine.Toc]; ok {
				ncxPath = it.Href
			}
		}
		if ncxPath == "" && ncxItem != nil {
			ncxPath = ncxItem.Href
		}
		if ncxPath != "" {
			if raw, err := a.read(ncxPath); err == nil {
				if s, err := decodeMarkup(raw); err == nil {
					toc = parseNCX([]byte(s), ncxPath)
				}
			}
		}
	}
	tocByFile := make(map[string]tocItem)
	for _, t := range toc {
		if t.Href == "" {
			continue
		}
		if _, ok := tocByFile[t.Href]; !ok {
			tocByFile[t.Href] = t
		}
	}

	book := &Book{
		Path:       name,
		Title:      strings.TrimSpace(opf.Metadata.Title),
		Author:     strings.TrimSpace(strings.Join(opf.Metadata.Creators, "、")),
		Language:   strings.TrimSpace(opf.Metadata.Language),
		CoverIndex: -1,
	}
	if book.Title == "" {
		book.Title = strings.TrimSuffix(path.Base(name), path.Ext(name))
	}
	if book.Author == "" {
		book.Author = "佚名"
	}

	// Locate an optional cover. EPUB 3 usually marks the image item with
	// properties="cover-image"; EPUB 2 uses <meta name="cover" content="id">
	// or a guide reference to a cover XHTML page.
	coverHref := ""
	for _, m := range opf.Metadata.Meta {
		if !strings.EqualFold(strings.TrimSpace(m.Name), "cover") {
			continue
		}
		if it, ok := items[strings.TrimSpace(m.Content)]; ok && isImagePath(it.Href) {
			coverHref = it.Href
			break
		}
	}
	if coverHref == "" {
		for _, it := range items {
			if strings.Contains(strings.ToLower(it.Properties), "cover-image") && isImagePath(it.Href) {
				coverHref = it.Href
				break
			}
		}
	}
	if coverHref == "" {
		for _, ref := range opf.Guide.References {
			if !strings.Contains(strings.ToLower(ref.Type), "cover") || strings.TrimSpace(ref.Href) == "" {
				continue
			}
			href := resolvePath(opfPath, ref.Href, false)
			if isImagePath(href) {
				coverHref = href
				break
			}
			if raw, err := a.read(href); err == nil {
				if text, err := decodeMarkup(raw); err == nil {
					if img := firstImageInHTML([]byte(text), href); img != "" {
						coverHref = img
						break
					}
				}
			}
		}
	}

	imageByHref := make(map[string]int)
	imageBytes := 0
	loadImage := func(href, alt string) int {
		href = normalizeZipPath(href)
		if href == "" {
			return -1
		}
		if idx, ok := imageByHref[href]; ok {
			if alt != "" && book.Images[idx].Alt == "" {
				book.Images[idx].Alt = alt
			}
			return idx
		}
		f, real, ok := a.find(href)
		if !ok || f.UncompressedSize64 > 32<<20 {
			return -1
		}
		data, err := a.read(real)
		if err != nil || len(data) == 0 || imageBytes+len(data) > 96<<20 {
			return -1
		}
		media := ""
		if it, ok := itemsByHref[real]; ok {
			media = it.MediaType
		}
		media = mediaTypeForImage(real, media)
		if alt == "" {
			alt = "插图"
		}
		imageBytes += len(data)
		idx := len(book.Images)
		book.Images = append(book.Images, ImageAsset{Href: real, MediaType: media, Alt: alt, Data: data})
		imageByHref[real] = idx
		return idx
	}
	if coverHref != "" {
		if idx := loadImage(coverHref, "封面"); idx >= 0 {
			book.CoverIndex = idx
			book.CoverHref = book.Images[idx].Href
		}
	}

	seen := make(map[string]bool)
	chapterNo := 0
	for _, ref := range opf.Spine.Refs {
		if strings.EqualFold(strings.TrimSpace(ref.Linear), "no") {
			continue
		}
		it, ok := items[ref.IDRef]
		if !ok {
			continue
		}
		if strings.Contains(strings.ToLower(it.Properties), "nav") {
			continue
		}
		if seen[it.Href] {
			continue
		}
		seen[it.Href] = true
		raw, err := a.read(it.Href)
		if err != nil {
			continue
		}
		text, err := decodeMarkup(raw)
		if err != nil {
			continue
		}
		blocks, heading, docTitle := parseContent([]byte(text))
		if len(blocks) == 0 {
			continue
		}
		for bi := range blocks {
			if blocks[bi].Kind != KindImage {
				continue
			}
			src := resolvePath(it.Href, blocks[bi].ImageSrc, false)
			idx := loadImage(src, blocks[bi].ImageAlt)
			if idx < 0 {
				blocks[bi].Kind = KindBody
				blocks[bi].Align = AlignCenter
				blocks[bi].Runs = []Run{{Text: "〔插图无法显示〕"}}
				continue
			}
			blocks[bi].ImageIndex = idx
			blocks[bi].Runs = []Run{{Text: imagePlaceholder(idx, blocks[bi].ImageAlt)}}
		}
		chapterNo++
		t, hasTOC := tocByFile[it.Href]
		title := strings.TrimSpace(t.Title)
		if title == "" {
			title = strings.TrimSpace(heading)
		}
		if title == "" {
			title = strings.TrimSpace(docTitle)
		}
		if title == "" {
			title = fmt.Sprintf("第 %d 章", chapterNo)
		}
		// Avoid showing the same heading twice (once as TOC title and once in
		// the body). Only drop a leading heading, never normal prose.
		if len(blocks) > 0 && isHeadingKind(blocks[0].Kind) {
			first := normalizeForCompare(blocks[0].Text())
			cmp := normalizeForCompare(title)
			if first != "" && (first == cmp || strings.Contains(cmp, first) || strings.Contains(first, cmp)) {
				blocks = blocks[1:]
			}
		}
		level := 0
		if hasTOC {
			level = t.Level
		}
		book.Chapters = append(book.Chapters, Chapter{
			Index:  len(book.Chapters),
			Title:  title,
			Level:  level,
			Href:   it.Href,
			Blocks: blocks,
		})
	}
	if len(book.Chapters) == 0 {
		return nil, errors.New("没有找到可显示的正文（EPUB 可能使用了不支持的版式）")
	}

	return book, nil
}

func isHeadingKind(k BlockKind) bool {
	return k == KindHeading1 || k == KindHeading2 || k == KindHeading3
}

func isImagePath(href string) bool {
	switch strings.ToLower(path.Ext(href)) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".avif":
		return true
	}
	return false
}

func mediaTypeForImage(href, declared string) string {
	declared = strings.ToLower(strings.TrimSpace(declared))
	if strings.HasPrefix(declared, "image/") {
		return declared
	}
	switch strings.ToLower(path.Ext(href)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	case ".svg":
		return "image/svg+xml"
	}
	return "application/octet-stream"
}

func firstImageInHTML(data []byte, baseFile string) string {
	doc, err := parseHTMLDocument(data)
	if err != nil {
		return ""
	}
	img := findElement(doc, "img")
	if img == nil {
		return ""
	}
	return resolvePath(baseFile, attrValue(img, "src"), false)
}

func imagePlaceholder(index int, alt string) string {
	alt = strings.TrimSpace(alt)
	if alt == "" || alt == "插图" {
		return fmt.Sprintf("〔插图 %d · 点击查看〕", index+1)
	}
	r := []rune(alt)
	if len(r) > 16 {
		alt = string(r[:16]) + "…"
	}
	return fmt.Sprintf("〔插图 %d · %s〕", index+1, alt)
}
