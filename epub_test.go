package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type testFile struct {
	name string
	data string
}

func makeEPUB(t *testing.T, files []testFile) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "test.epub")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	// mimetype first and uncompressed, as required by the spec.
	mw, err := zw.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = mw.Write([]byte("application/epub+zip"))
	for _, tf := range files {
		w, err := zw.Create(tf.name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(tf.data))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return p
}

func containerXML(opf string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="` + opf + `" media-type="application/oebps-package+xml"/></rootfiles>
</container>`
}

func TestLoadEPUB3NavAndPercentPath(t *testing.T) {
	opf := `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="id">
 <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
  <dc:title>山雨集</dc:title><dc:creator>林川</dc:creator><dc:language>zh-CN</dc:language>
 </metadata>
 <manifest>
  <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
  <item id="c1" href="text/ch1.xhtml" media-type="application/xhtml+xml"/>
  <item id="c2" href="text/ch%202.xhtml" media-type="application/xhtml+xml"/>
 </manifest>
 <spine><itemref idref="c1"/><itemref idref="c2"/></spine>
</package>`
	nav := `<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">
<head><title>目录</title></head><body>
<nav epub:type="toc"><ol>
 <li><a href="text/ch1.xhtml">第一章 山雨</a></li>
 <li><a href="text/ch%202.xhtml">第二章 灯下</a></li>
</ol></nav></body></html>`
	ch1 := `<html><head><title>第一章</title></head><body>
<h1>第一章 山雨</h1>
<p>山雨 欲来，风满楼。</p>
<p>他<strong>推开</strong>木窗，看见<em>远山</em>如墨。</p>
<p>第一行<br/>第二行</p>
</body></html>`
	ch2 := `<html><body><h2>第二章 灯下</h2><p>灯下读书，纸页有声。</p></body></html>`
	p := makeEPUB(t, []testFile{
		{"META-INF/container.xml", containerXML("OEBPS/content.opf")},
		{"OEBPS/content.opf", opf},
		{"OEBPS/nav.xhtml", nav},
		{"OEBPS/text/ch1.xhtml", ch1},
		{"OEBPS/text/ch 2.xhtml", ch2},
	})
	book, err := LoadEPUB(p)
	if err != nil {
		t.Fatal(err)
	}
	if book.Title != "山雨集" || book.Author != "林川" {
		t.Fatalf("metadata: %q %q", book.Title, book.Author)
	}
	if len(book.Chapters) != 2 {
		t.Fatalf("chapters: %d", len(book.Chapters))
	}
	if book.Chapters[0].Title != "第一章 山雨" || book.Chapters[1].Title != "第二章 灯下" {
		t.Fatalf("titles: %q %q", book.Chapters[0].Title, book.Chapters[1].Title)
	}
	if len(book.Chapters[0].Blocks) == 0 || isHeadingKind(book.Chapters[0].Blocks[0].Kind) {
		t.Fatal("duplicate leading heading was not removed")
	}
	all := ""
	for _, b := range book.Chapters[0].Blocks {
		all += b.Text() + "|"
	}
	if !strings.Contains(all, "山雨欲来，风满楼。") {
		t.Fatalf("whitespace normalization failed: %s", all)
	}
	if !strings.Contains(all, "\u2028") {
		t.Fatalf("br was not preserved: %q", all)
	}
	foundBold, foundItalic := false, false
	for _, b := range book.Chapters[0].Blocks {
		for _, r := range b.Runs {
			foundBold = foundBold || r.Bold
			foundItalic = foundItalic || r.Italic
		}
	}
	if !foundBold || !foundItalic {
		t.Fatalf("emphasis missing bold=%v italic=%v", foundBold, foundItalic)
	}
}

func TestLoadEPUB2NCX(t *testing.T) {
	opf := `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="2.0">
 <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>旧书</dc:title><dc:creator>某人</dc:creator></metadata>
 <manifest>
  <item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>
  <item id="c1" href="c1.xhtml" media-type="application/xhtml+xml"/>
  <item id="c2" href="c2.xhtml" media-type="application/xhtml+xml"/>
 </manifest>
 <spine toc="ncx"><itemref idref="c1"/><itemref idref="c2"/></spine>
</package>`
	ncx := `<?xml version="1.0" encoding="UTF-8"?>
<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1"><navMap>
 <navPoint id="n1"><navLabel><text>卷一</text></navLabel><content src="c1.xhtml"/>
  <navPoint id="n2"><navLabel><text>第一章</text></navLabel><content src="c1.xhtml#p1"/></navPoint>
 </navPoint>
 <navPoint id="n3"><navLabel><text>第二章</text></navLabel><content src="c2.xhtml"/></navPoint>
</navMap></ncx>`
	p := makeEPUB(t, []testFile{
		{"META-INF/container.xml", containerXML("OEBPS/content.opf")},
		{"OEBPS/content.opf", opf},
		{"OEBPS/toc.ncx", ncx},
		{"OEBPS/c1.xhtml", `<html><body><h1>第一章</h1><p>正文一。</p></body></html>`},
		{"OEBPS/c2.xhtml", `<html><body><h1>第二章</h1><p>正文二。</p></body></html>`},
	})
	book, err := LoadEPUB(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(book.Chapters) != 2 {
		t.Fatalf("chapters %d", len(book.Chapters))
	}
	if book.Chapters[0].Title != "卷一" {
		t.Fatalf("ncx title: %q", book.Chapters[0].Title)
	}
	if book.Chapters[1].Title != "第二章" || book.Chapters[1].Level != 0 {
		t.Fatalf("second title: %q level %d", book.Chapters[1].Title, book.Chapters[1].Level)
	}
}

func TestGBKContent(t *testing.T) {
	gbk := []byte{
		0x3c, 0x68, 0x74, 0x6d, 0x6c, 0x3e, 0x3c, 0x68, 0x65, 0x61, 0x64, 0x3e, 0x3c, 0x6d, 0x65, 0x74, 0x61, 0x20, 0x63, 0x68, 0x61, 0x72, 0x73, 0x65, 0x74, 0x3d, 0x22, 0x67, 0x62, 0x6b, 0x22, 0x3e, 0x3c, 0x2f, 0x68, 0x65, 0x61, 0x64, 0x3e, 0x3c, 0x62, 0x6f, 0x64, 0x79, 0x3e, 0x3c, 0x70, 0x3e,
	}
	// "中文测试" in GBK
	gbk = append(gbk, 0xd6, 0xd0, 0xce, 0xc4, 0xb2, 0xe2, 0xca, 0xd4)
	gbk = append(gbk, []byte("</p></body></html>")...)
	opf := `<package xmlns="http://www.idpf.org/2007/opf" version="2.0"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>编码</dc:title></metadata><manifest><item id="c1" href="c1.xhtml" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="c1"/></spine></package>`
	dir := t.TempDir()
	p := filepath.Join(dir, "gbk.epub")
	f, _ := os.Create(p)
	zw := zip.NewWriter(f)
	for _, tf := range []struct {
		n string
		d []byte
	}{
		{"mimetype", []byte("application/epub+zip")},
		{"META-INF/container.xml", []byte(containerXML("content.opf"))},
		{"content.opf", []byte(opf)},
		{"c1.xhtml", gbk},
	} {
		w, _ := zw.Create(tf.n)
		_, _ = w.Write(tf.d)
	}
	_ = zw.Close()
	_ = f.Close()
	book, err := LoadEPUB(p)
	if err != nil {
		t.Fatal(err)
	}
	if got := book.Chapters[0].Blocks[0].Text(); got != "中文测试" {
		t.Fatalf("gbk decode: %q", got)
	}
}

func TestBuildDocumentOffsets(t *testing.T) {
	book := &Book{
		Title:  "测试书",
		Author: "作者",
		Chapters: []Chapter{
			{Title: "第一章", Blocks: []Block{{Runs: []Run{{Text: "你好。"}}, Kind: KindBody}}},
			{Title: "第二章", Blocks: []Block{{Runs: []Run{{Text: "再见。"}}, Kind: KindBody}}},
		},
	}
	d := BuildDocument(book)
	if d.Length != int32(len(d.Text)) {
		t.Fatal("length mismatch")
	}
	if len(d.Chapters) != 2 || d.Chapters[0].Start >= d.Chapters[1].Start {
		t.Fatal("bad chapter offsets")
	}
	if d.ChapterAt(d.Chapters[1].Start) != 1 {
		t.Fatal("ChapterAt failed")
	}
	plain := d.PlainText()
	if !strings.Contains(plain, "你好") || !strings.Contains(plain, "再见") {
		t.Fatalf("plain text: %q", plain)
	}
}

func TestNavDepth(t *testing.T) {
	nav := `<html><body><nav epub:type="toc"><ol>
<li><a href="a.xhtml">第一章</a>
 <ol><li><a href="a.xhtml#s1">第一节</a></li></ol>
</li>
<li><a href="b.xhtml">第二章</a></li>
</ol></nav></body></html>`
	items := parseNav([]byte(nav), "OEBPS/nav.xhtml")
	if len(items) != 3 {
		t.Fatalf("items %d", len(items))
	}
	if items[0].Level != 0 || items[1].Level != 1 || items[2].Level != 0 {
		t.Fatalf("levels %d %d %d", items[0].Level, items[1].Level, items[2].Level)
	}
}

func TestPrettyPrintedWhitespaceAndHidden(t *testing.T) {
	html := `<html><body><p>
	  第一行
	  第二行
	  <span style="display:none">隐藏</span>
	</p><p>甲<br/>乙</p></body></html>`
	blocks, _, _ := parseContent([]byte(html))
	if len(blocks) != 2 {
		t.Fatalf("blocks %d", len(blocks))
	}
	first := blocks[0].Text()
	if strings.ContainsRune(first, lineBreakSentinel) {
		t.Fatalf("source newline became a hard break: %q", first)
	}
	if strings.Contains(first, "隐藏") {
		t.Fatalf("hidden text leaked: %q", first)
	}
	if !strings.Contains(blocks[1].Text(), string(lineBreakSentinel)) {
		t.Fatalf("br not preserved: %q", blocks[1].Text())
	}
}

func TestLoadEPUBWithoutTOC(t *testing.T) {
	opf := `<package xmlns="http://www.idpf.org/2007/opf" version="2.0"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>无目录</dc:title></metadata><manifest><item id="c1" href="c1.xhtml" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="c1"/></spine></package>`
	p := makeEPUB(t, []testFile{
		{"META-INF/container.xml", containerXML("content.opf")},
		{"content.opf", opf},
		{"c1.xhtml", `<html><body><h1>开篇</h1><p>正文。</p></body></html>`},
	})
	book, err := LoadEPUB(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(book.Chapters) != 1 || book.Chapters[0].Title != "开篇" {
		t.Fatalf("chapters: %+v", book.Chapters)
	}
}

func TestProtectLineBreaks(t *testing.T) {
	got := protectLineBreaks("甲，乙。丙")
	if !strings.Contains(got, "\u2060，") || !strings.Contains(got, "\u2060。") {
		t.Fatalf("word joiners missing: %q", got)
	}
}
