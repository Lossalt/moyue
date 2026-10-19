package main

import (
	"bytes"
	"strings"
	"unicode"

	"golang.org/x/net/html"
)

type htmlNode = html.Node

const htmlElementNode = html.ElementNode

func parseHTMLDocument(data []byte) (*html.Node, error) {
	return html.Parse(bytes.NewReader(data))
}

func findElement(n *html.Node, tag string) *html.Node {
	if n == nil {
		return nil
	}
	if n.Type == html.ElementNode && strings.EqualFold(n.Data, tag) {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findElement(c, tag); found != nil {
			return found
		}
	}
	return nil
}

func attrValue(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val
		}
	}
	return ""
}

func nodeText(n *html.Node) string {
	if n == nil {
		return ""
	}
	if n.Type == html.TextNode {
		return n.Data
	}
	var sb strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		sb.WriteString(nodeText(c))
	}
	return sb.String()
}

const lineBreakSentinel = '\u2028'

type inlineStyle struct {
	bold, italic bool
	pre          bool
}

type contentBuilder struct {
	blocks   []Block
	cur      []Run
	curKind  BlockKind
	curAlign Align
}

func (b *contentBuilder) addText(s string, st inlineStyle) {
	if s == "" {
		return
	}
	b.cur = append(b.cur, Run{Text: s, Bold: st.bold, Italic: st.italic})
}

func (b *contentBuilder) addBreak() {
	b.cur = append(b.cur, Run{Text: string(lineBreakSentinel)})
}

func (b *contentBuilder) flush() {
	runs, text := normalizeRuns(b.cur)
	b.cur = nil
	if strings.TrimSpace(strings.ReplaceAll(text, string(lineBreakSentinel), "")) == "" {
		return
	}
	b.blocks = append(b.blocks, Block{Runs: runs, Kind: b.curKind, Align: b.curAlign})
	b.curKind = KindBody
	b.curAlign = AlignDefault
}

func (b *contentBuilder) addDivider() {
	b.blocks = append(b.blocks, Block{Runs: []Run{{Text: "· · ·"}}, Kind: KindDivider, Align: AlignCenter})
}

// parseContent converts an XHTML content document into renderable blocks.
func parseContent(data []byte) (blocks []Block, firstHeading, docTitle string) {
	doc, err := parseHTMLDocument(data)
	if err != nil {
		return nil, "", ""
	}
	if t := findElement(doc, "title"); t != nil {
		docTitle = normalizeInlineText(nodeText(t))
	}
	body := findElement(doc, "body")
	if body == nil {
		body = doc
	}
	b := &contentBuilder{}
	var walk func(n *html.Node, st inlineStyle, ctxKind BlockKind, ctxAlign Align)
	walk = func(n *html.Node, st inlineStyle, ctxKind BlockKind, ctxAlign Align) {
		if n == nil {
			return
		}
		if n.Type == html.TextNode {
			text := n.Data
			if st.pre {
				text = strings.ReplaceAll(text, "\r\n", "\n")
				text = strings.ReplaceAll(text, "\r", "\n")
				text = strings.ReplaceAll(text, "\n", string(lineBreakSentinel))
			} else {
				text = strings.ReplaceAll(text, "\r\n", " ")
				text = strings.ReplaceAll(text, "\r", " ")
				text = strings.ReplaceAll(text, "\n", " ")
				text = strings.ReplaceAll(text, "\t", " ")
			}
			b.addText(text, st)
			return
		}
		if n.Type != html.ElementNode {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c, st, ctxKind, ctxAlign)
			}
			return
		}
		tag := strings.ToLower(n.Data)
		if isHiddenNode(n) {
			return
		}
		switch tag {
		case "script", "style", "head", "svg", "canvas", "template", "nav", "rt", "rp":
			return
		case "header", "footer":
			return
		case "br":
			b.addBreak()
			return
		case "hr":
			b.flush()
			b.addDivider()
			return
		case "img":
			alt := strings.TrimSpace(attrValue(n, "alt"))
			if alt == "" {
				alt = "〔插图〕"
			}
			b.addText(" "+alt+" ", st)
			return
		case "h1", "h2", "h3", "h4", "h5", "h6":
			b.flush()
			kind := KindHeading1
			switch tag {
			case "h2":
				kind = KindHeading2
			case "h3", "h4", "h5", "h6":
				kind = KindHeading3
			}
			sub := &contentBuilder{curKind: kind, curAlign: AlignCenter}
			if kind == KindHeading3 {
				sub.curAlign = AlignDefault
			}
			collectInline(n, st, sub)
			sub.flush()
			if len(sub.blocks) > 0 {
				if firstHeading == "" {
					firstHeading = sub.blocks[0].Text()
				}
				b.blocks = append(b.blocks, sub.blocks...)
			}
			return
		case "p", "div", "section", "article", "blockquote", "li", "dd", "dt", "figure", "figcaption", "pre", "tr", "td", "th", "aside", "main", "center":
			b.flush()
			kind := ctxKind
			align := ctxAlign
			if kind == 0 {
				kind = KindBody
			}
			switch tag {
			case "blockquote":
				kind = KindQuote
			case "center":
				align = AlignCenter
			case "li":
				if kind == KindBody {
					b.addText("· ", inlineStyle{})
				}
			}
			if a := detectAlign(n); a != AlignDefault {
				align = a
			}
			st2 := st
			if tag == "pre" {
				st2.pre = true
			}
			b.curKind = kind
			b.curAlign = align
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c, st2, kind, align)
			}
			b.flush()
			return
		}
		st2 := st
		switch tag {
		case "b", "strong":
			st2.bold = true
		case "i", "em", "cite", "dfn":
			st2.italic = true
		}
		if s := attrValue(n, "style"); s != "" {
			ls := strings.ToLower(strings.ReplaceAll(s, " ", ""))
			if strings.Contains(ls, "font-weight:bold") || strings.Contains(ls, "font-weight:700") || strings.Contains(ls, "font-weight:800") || strings.Contains(ls, "font-weight:900") {
				st2.bold = true
			}
			if strings.Contains(ls, "font-style:italic") {
				st2.italic = true
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, st2, ctxKind, ctxAlign)
		}
	}
	walk(body, inlineStyle{}, KindBody, AlignDefault)
	b.flush()
	return b.blocks, firstHeading, docTitle
}

func collectInline(n *html.Node, st inlineStyle, b *contentBuilder) {
	if n == nil {
		return
	}
	if n.Type == html.TextNode {
		b.addText(n.Data, st)
		return
	}
	if n.Type != html.ElementNode {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			collectInline(c, st, b)
		}
		return
	}
	tag := strings.ToLower(n.Data)
	if isHiddenNode(n) {
		return
	}
	switch tag {
	case "script", "style", "rt", "rp", "img":
		if tag == "img" {
			alt := strings.TrimSpace(attrValue(n, "alt"))
			if alt != "" {
				b.addText(" "+alt+" ", st)
			}
		}
		return
	case "br":
		b.addBreak()
		return
	}
	st2 := st
	if tag == "b" || tag == "strong" {
		st2.bold = true
	}
	if tag == "i" || tag == "em" || tag == "cite" {
		st2.italic = true
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		collectInline(c, st2, b)
	}
}

func detectAlign(n *html.Node) Align {
	if strings.EqualFold(attrValue(n, "align"), "center") {
		return AlignCenter
	}
	if strings.EqualFold(attrValue(n, "align"), "middle") {
		return AlignCenter
	}
	style := strings.ToLower(strings.ReplaceAll(attrValue(n, "style"), " ", ""))
	if strings.Contains(style, "text-align:center") {
		return AlignCenter
	}
	class := strings.ToLower(attrValue(n, "class"))
	if strings.Contains(class, "center") || strings.Contains(class, "centered") {
		return AlignCenter
	}
	return AlignDefault
}

func normalizeForCompare(s string) string {
	s = strings.ToLower(s)
	var sb strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func normalizeInlineText(s string) string {
	runs, text := normalizeRuns([]Run{{Text: s}})
	_ = runs
	return strings.ReplaceAll(text, string(lineBreakSentinel), " ")
}

// normalizeDisplayText is for titles: it collapses line breaks and runs of
// whitespace but keeps a single meaningful space between words.
func normalizeDisplayText(s string) string {
	s = strings.ReplaceAll(s, "\u00a0", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.Join(strings.Fields(s), " ")
}

// normalizeRuns collapses HTML whitespace while preserving emphasis spans.
// Spaces between two CJK characters are dropped, since they are almost always
// artifacts of pretty-printed XHTML in Chinese books.
func normalizeRuns(in []Run) ([]Run, string) {
	type styledRune struct {
		r     rune
		b, i  bool
		space bool
		br    bool
	}
	var src []styledRune
	for _, run := range in {
		for _, r := range run.Text {
			if r == '\r' {
				continue
			}
			sr := styledRune{r: r, b: run.Bold, i: run.Italic}
			if r == lineBreakSentinel {
				sr.br = true
				sr.r = lineBreakSentinel
			} else if unicode.IsSpace(r) {
				sr.space = true
				sr.r = ' '
			}
			src = append(src, sr)
		}
	}
	// Trim.
	for len(src) > 0 && (src[0].space || src[0].br) {
		src = src[1:]
	}
	for len(src) > 0 && (src[len(src)-1].space || src[len(src)-1].br) {
		src = src[:len(src)-1]
	}
	var out []styledRune
	for idx := 0; idx < len(src); idx++ {
		s := src[idx]
		if s.space {
			j := idx
			for j < len(src) && src[j].space {
				j++
			}
			var prev, next rune
			if len(out) > 0 {
				prev = out[len(out)-1].r
			}
			if j < len(src) {
				next = src[j].r
			}
			idx = j - 1
			if prev == 0 || next == 0 || next == lineBreakSentinel || prev == lineBreakSentinel {
				continue
			}
			if isCJKPunct(prev) || isCJKPunct(next) {
				continue
			}
			if isCJK(prev) && isCJK(next) {
				continue
			}
			s.r = ' '
			out = append(out, s)
			continue
		}
		if s.br {
			if len(out) > 0 && out[len(out)-1].br {
				continue
			}
			if len(out) > 0 && out[len(out)-1].space {
				out = out[:len(out)-1]
			}
			out = append(out, s)
			continue
		}
		out = append(out, s)
	}
	for len(out) > 0 && (out[0].space || out[0].br) {
		out = out[1:]
	}
	for len(out) > 0 && (out[len(out)-1].space || out[len(out)-1].br) {
		out = out[:len(out)-1]
	}
	var runs []Run
	var text strings.Builder
	for i := 0; i < len(out); {
		j := i + 1
		for j < len(out) && out[j].b == out[i].b && out[j].i == out[i].i {
			j++
		}
		var sb strings.Builder
		for k := i; k < j; k++ {
			sb.WriteRune(out[k].r)
		}
		part := sb.String()
		text.WriteString(part)
		runs = append(runs, Run{Text: part, Bold: out[i].b, Italic: out[i].i})
		i = j
	}
	return runs, text.String()
}

func isCJK(r rune) bool {
	return (r >= 0x3400 && r <= 0x9FFF) ||
		(r >= 0xF900 && r <= 0xFAFF) ||
		(r >= 0x20000 && r <= 0x2FA1F) ||
		(r >= 0x3040 && r <= 0x30FF) ||
		(r >= 0xAC00 && r <= 0xD7AF) ||
		(r >= 0x31F0 && r <= 0x31FF)
}

func isCJKPunct(r rune) bool {
	if r >= 0x3000 && r <= 0x303F {
		return true
	}
	if r >= 0xFF00 && r <= 0xFFEF {
		return r >= 0xFF01 && r <= 0xFF65
	}
	switch r {
	case '“', '”', '‘', '’', '—', '…', '·', '《', '》', '〈', '〉', '「', '」', '『', '』', '【', '】', '（', '）', '、', '。', '，', '！', '？', '；', '：':
		return true
	}
	return false
}

func isHiddenNode(n *html.Node) bool {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, "hidden") {
			return true
		}
		if strings.EqualFold(a.Key, "style") {
			st := strings.ToLower(strings.ReplaceAll(a.Val, " ", ""))
			if strings.Contains(st, "display:none") || strings.Contains(st, "visibility:hidden") {
				return true
			}
		}
	}
	return false
}
