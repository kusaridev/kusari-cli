// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package auth

import (
	"os/exec"
	"runtime"
)

func OpenBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		// Deliberately not `cmd /c start`: cmd.exe treats & | ^ as command
		// separators, and Go only quotes arguments containing whitespace or
		// quotes, so an OAuth URL (which always carries & between query
		// parameters) would be split apart before the browser ever saw it.
		// rundll32 hands the URL to the shell's protocol handler directly,
		// with no command interpreter in between.
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler"}
	case "darwin":
		cmd = "open"
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
	}
	args = append(args, url)
	return exec.Command(cmd, args...).Start()
}
