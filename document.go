package main

import (
	"fmt"
	"strings"
	"unicode/utf16"
)

type PositionedRun struct {
	Start, End   int32
	Bold, Italic bool
}

type PositionedBlock struct {
	Start, End int32
	Kind       BlockKind
	Align      Align
	ImageIndex int
	Runs       []PositionedRun
}

// Document is the ready-to-render form of a Book. All offsets are UTF-16 code
// units, matching the character positions used by RichEdit.
type Document struct {
	Text      []uint16
	Length    int32
	Blocks    []PositionedBlock
	Chapters  []Chapter
	TitlePage int
}

func BuildDocument(book *Book) *Document {
	d := &Document{Text: make([]uint16, 0, 1<<20)}

	// Title page ---------------------------------------------------------
	if book.CoverIndex >= 0 && book.CoverIndex < len(book.Images) {
		d.appendBlockEx(KindImage, AlignCenter, []Run{{Text: "〔封面 · 点击查看〕"}}, book.CoverIndex)
	}
	d.appendBlock(KindTitlePage, AlignCenter, []Run{{Text: book.Title}})
	d.appendBlock(KindAuthor, AlignCenter, []Run{{Text: book.Author}})
	d.appendBlock(KindDivider, AlignCenter, []Run{{Text: "· · ·"}})
	meta := fmt.Sprintf("EPUB · 共 %d 章", len(book.Chapters))
	d.appendBlock(KindMeta, AlignCenter, []Run{{Text: meta}})

	// Chapters -----------------------------------------------------------
	for i := range book.Chapters {
		ch := book.Chapters[i]
		ch.Start = d.Length
		d.appendBlock(KindChapterTitle, AlignCenter, []Run{{Text: ch.Title}})
		for _, b := range ch.Blocks {
			if strings.TrimSpace(strings.ReplaceAll(b.Text(), string(lineBreakSentinel), "")) == "" {
				continue
			}
			d.appendBlockEx(b.Kind, b.Align, b.Runs, b.ImageIndex)
		}
		ch.End = d.Length
		d.Chapters = append(d.Chapters, ch)
	}

	// Generous end padding lets any chapter title scroll to the top of the
	// viewport, even the final one. It also gives the last page room to breathe.
	if len(book.Chapters) > 0 {
		d.appendBlock(KindDivider, AlignCenter, []Run{{Text: "· 全书完 ·"}})
		for i := 0; i < 22; i++ {
			// A full-width space keeps trailing blank paragraphs addressable;
			// RichEdit can otherwise trim empty paragraphs at the very end.
			d.Text = append(d.Text, 0x3000, '\r')
		}
	}
	d.Length = int32(len(d.Text))
	return d
}

func (d *Document) appendBlock(kind BlockKind, align Align, runs []Run) {
	d.appendBlockEx(kind, align, runs, -1)
}

func (d *Document) appendBlockEx(kind BlockKind, align Align, runs []Run, imageIndex int) {
	start := int32(len(d.Text))
	pb := PositionedBlock{Start: start, Kind: kind, Align: align, ImageIndex: imageIndex}
	for _, r := range runs {
		t := strings.ReplaceAll(r.Text, string(lineBreakSentinel), "\v")
		t = strings.ReplaceAll(t, "\r", "")
		t = protectLineBreaks(t)
		if t == "" {
			continue
		}
		rs := int32(len(d.Text))
		d.Text = append(d.Text, utf16.Encode([]rune(t))...)
		re := int32(len(d.Text))
		if r.Bold || r.Italic {
			pb.Runs = append(pb.Runs, PositionedRun{Start: rs, End: re, Bold: r.Bold, Italic: r.Italic})
		}
	}
	d.Text = append(d.Text, '\r')
	pb.End = int32(len(d.Text))
	d.Length = pb.End
	d.Blocks = append(d.Blocks, pb)
}

// ImageAt returns the image shown by the block containing pos, or -1.
func (d *Document) ImageAt(pos int32) int {
	for _, pb := range d.Blocks {
		if pb.Kind == KindImage && pos >= pb.Start && pos < pb.End {
			return pb.ImageIndex
		}
	}
	return -1
}

func (d *Document) ChapterAt(pos int32) int {
	lo, hi := 0, len(d.Chapters)-1
	idx := -1
	for lo <= hi {
		mid := (lo + hi) / 2
		if d.Chapters[mid].Start <= pos {
			idx = mid
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	if idx < 0 {
		return 0
	}
	return idx
}

// PlainText is useful for tests and for a future export feature.
func (d *Document) PlainText() string {
	return strings.ReplaceAll(string(utf16.Decode(d.Text)), "\r", "\n")
}

// protectLineBreaks inserts a zero-width word joiner before closing CJK
// punctuation. Windows' RichEdit uses it to avoid starting a line with a
// comma or full stop; fonts without the glyph simply ignore it.
func protectLineBreaks(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '，', '。', '、', '；', '：', '？', '！', '）', '〕', '〉', '》', '」', '』', '】', '”', '’', '…', '—':
			b.WriteRune('\u2060')
		}
		b.WriteRune(r)
	}
	return b.String()
}
