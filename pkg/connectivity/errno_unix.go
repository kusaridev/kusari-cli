// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

//go:build !windows

package connectivity

import (
	"errors"
	"syscall"
)

func isConnRefused(err error) bool { return errors.Is(err, syscall.ECONNREFUSED) }

func isConnReset(err error) bool { return errors.Is(err, syscall.ECONNRESET) }

func isUnreachable(err error) bool {
	return errors.Is(err, syscall.EHOSTUNREACH) || errors.Is(err, syscall.ENETUNREACH)
}
