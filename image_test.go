package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestDecodeRenderedPNG(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 1, 1))
	src.Set(0, 0, color.RGBA{R: 200, G: 80, B: 40, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatal(err)
	}
	img, err := decodeRenderedImage(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if img.W != 1 || img.H != 1 || len(img.Pix) != 4 {
		t.Fatalf("rendered: %+v", img)
	}
}

func TestDocumentImageAt(t *testing.T) {
	book := &Book{
		Title:      "画集",
		Author:     "作者",
		CoverIndex: 0,
		Images:     []ImageAsset{{Alt: "封面"}, {Alt: "插图"}},
		Chapters: []Chapter{{
			Title: "第一章",
			Blocks: []Block{
				{Kind: KindBody, Runs: []Run{{Text: "前文"}}},
				{Kind: KindImage, ImageIndex: 1, Runs: []Run{{Text: "〔插图 2〕"}}},
			},
		}},
	}
	d := BuildDocument(book)
	if len(d.Blocks) < 2 || d.Blocks[0].Kind != KindImage {
		t.Fatalf("cover block missing: %+v", d.Blocks)
	}
	if got := d.ImageAt(d.Blocks[0].Start); got != 0 {
		t.Fatalf("cover ImageAt=%d", got)
	}
	found := false
	for _, pb := range d.Blocks {
		if pb.Kind == KindImage && pb.ImageIndex == 1 {
			found = true
			if got := d.ImageAt(pb.Start + 1); got != 1 {
				t.Fatalf("inline ImageAt=%d", got)
			}
		}
	}
	if !found {
		t.Fatal("inline image block missing")
	}
}
