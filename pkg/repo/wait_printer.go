// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package repo

import (
	"fmt"
	"io"
	"sync"
)

// waitPrinter prints a single shared progress line for the concurrent
// software-info pollers: a header on the first wait, then one '#' per poll
// retry across all pollers. close() terminates the line once polling is
// done. Per-file outcomes (including lookup failures) are reported in the
// results table afterwards, so the progress line doesn't attribute waits to
// individual files.
type waitPrinter struct {
	mu      sync.Mutex
	out     io.Writer
	started bool
}

func newWaitPrinter(out io.Writer) *waitPrinter {
	return &waitPrinter{out: out}
}

// tick records one poll retry, printing the header on the first wait.
func (p *waitPrinter) tick() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started {
		p.started = true
		_, _ = fmt.Fprint(p.out, "  Waiting for software info for ingested SBOMs...\n  ")
	}
	_, _ = fmt.Fprint(p.out, "#")
}

// close terminates the progress line. No-op if nothing ever waited.
func (p *waitPrinter) close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.started {
		p.started = false
		_, _ = fmt.Fprintln(p.out)
	}
}
