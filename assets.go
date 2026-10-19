package main

import _ "embed"

// The three typefaces are embedded into the executable as OpenType/TrueType
// blobs. They are all licensed under the SIL Open Font License 1.1.

//go:embed fonts/InkSerif.ttf
var fontSerif []byte

//go:embed fonts/InkSans.ttf
var fontSans []byte

//go:embed fonts/LXGWWenKaiScreen.ttf
var fontKai []byte

const (
	familySerif = "Noto Serif SC"
	familySans  = "Noto Sans SC"
	familyKai   = "LXGW WenKai Screen"
)
