// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package main

import (
	"errors"
	"os"

	"github.com/kusaridev/kusari-cli/v2/kusari/cmd"
)

// exitCoder is implemented by errors that carry their own process exit status.
// It is declared here rather than imported so main stays independent of the
// packages that define such errors.
type exitCoder interface {
	ExitCode() int
}

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cmd.SetVersionInfo(version, commit, date)
	if err := cmd.Execute(); err != nil {
		// A scan that completed but reported findings exits with its own status,
		// so callers can tell a security finding from a broken scan.
		var coder exitCoder
		if errors.As(err, &coder) {
			os.Exit(coder.ExitCode())
		}
		os.Exit(1)
	}
}
