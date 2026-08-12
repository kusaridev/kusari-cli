// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

//go:build windows

package connectivity

import (
	"errors"
	"syscall"

	"golang.org/x/sys/windows"
)

// Winsock reports its own error numbers (WSAE*, 10000-range), and Go does not
// translate them to the POSIX names. The syscall.E* constants still exist on
// Windows, but as synthetic values in the APPLICATION_ERROR range that no
// socket ever returns, so matching on them alone silently classifies every
// Windows connection failure as "unknown". Both spellings are checked here.

func isConnRefused(err error) bool {
	return errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, windows.WSAECONNREFUSED)
}

func isConnReset(err error) bool {
	return errors.Is(err, syscall.ECONNRESET) || errors.Is(err, windows.WSAECONNRESET)
}

func isUnreachable(err error) bool {
	return errors.Is(err, syscall.EHOSTUNREACH) || errors.Is(err, syscall.ENETUNREACH) ||
		errors.Is(err, windows.WSAEHOSTUNREACH) || errors.Is(err, windows.WSAENETUNREACH)
}
