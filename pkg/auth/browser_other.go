// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

//go:build !windows

package auth

import (
	"os/exec"
	"runtime"
)

// OpenBrowser launches the platform's URL opener. Start rather than Run: these
// helpers hand off to a browser and return, and waiting on them would block the
// login flow. A missing opener still surfaces here, because Start fails when
// the binary cannot be found.
func OpenBrowser(url string) error {
	opener := "xdg-open"
	if runtime.GOOS == "darwin" {
		opener = "open"
	}
	return exec.Command(opener, url).Start()
}
