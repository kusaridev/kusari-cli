// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

//go:build windows

package auth

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// OpenBrowser hands url to the shell's default handler through ShellExecute --
// the same call Explorer and `start` make, so it honours whatever browser
// association the user has configured.
//
// Both shell-based alternatives misfire here. `cmd /c start <url>` treats & as
// a command separator and Go only quotes arguments containing whitespace, so an
// OAuth URL is chopped at its first query parameter. `rundll32
// url.dll,FileProtocolHandler <url>` avoids that, but it is a legacy handler
// that starts successfully and then quietly does nothing when no association
// resolves -- common on Windows Server -- leaving the caller with a nil error
// and the user with no browser.
//
// ShellExecute reports failure directly: any result of 32 or less is an error,
// which x/sys surfaces as a non-nil error. That is what lets the login flow
// notice and tell the user to open the URL themselves.
func OpenBrowser(url string) error {
	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	target, err := windows.UTF16PtrFromString(url)
	if err != nil {
		return err
	}
	if err := windows.ShellExecute(0, verb, target, nil, nil, windows.SW_SHOWNORMAL); err != nil {
		return fmt.Errorf("the shell could not open %s: %w", url, err)
	}
	return nil
}
