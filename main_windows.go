//go:build windows

package main

import "runtime"

func main() {
	runtime.LockOSThread()
	setDPIAwareness()
	embeddedOK = registerEmbeddedFonts()
	gApp = newApp()
	if !gApp.create() {
		return
	}
	gApp.run()
}
