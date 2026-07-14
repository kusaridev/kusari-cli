// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package repo

import (
	"strings"
	"testing"
)

func TestWaitPrinter(t *testing.T) {
	t.Run("header once then a hash per tick", func(t *testing.T) {
		out := &strings.Builder{}
		p := newWaitPrinter(out)
		p.tick()
		p.tick()
		p.tick()
		p.close()
		expected := "  Waiting for software info for ingested SBOMs...\n  ###\n"
		if out.String() != expected {
			t.Errorf("expected %q, got %q", expected, out.String())
		}
	})

	t.Run("close without waiting prints nothing", func(t *testing.T) {
		out := &strings.Builder{}
		p := newWaitPrinter(out)
		p.close()
		if out.String() != "" {
			t.Errorf("expected no output, got %q", out.String())
		}
	})

	t.Run("close is idempotent", func(t *testing.T) {
		out := &strings.Builder{}
		p := newWaitPrinter(out)
		p.tick()
		p.close()
		p.close()
		expected := "  Waiting for software info for ingested SBOMs...\n  #\n"
		if out.String() != expected {
			t.Errorf("expected %q, got %q", expected, out.String())
		}
	})
}
