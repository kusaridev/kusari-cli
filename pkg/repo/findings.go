// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package repo

import (
	"fmt"

	"github.com/kusaridev/kusari-cli/v2/api"
)

// findingsExitCode is returned by the process when a scan completes and the
// analysis says not to proceed.
//
// Deliberately not 1: every other failure in this CLI exits 1, and a caller
// that cannot tell a security finding from a network outage has to treat both
// the same -- which means either blocking on outages or ignoring findings. 2 is
// left alone because shells conventionally use it for misuse of a builtin.
const findingsExitCode = 3

// FindingsError reports that a scan ran to completion and the analysis said not
// to proceed.
//
// This is not a failure. The scan worked and its results have already been
// written to stdout by the time this is returned; the error exists only so the
// exit code can distinguish "there are findings" from "the scan broke".
type FindingsError struct {
	// CodeMitigations is how many code findings must be addressed.
	CodeMitigations int
	// DependencyMitigations is how many dependency findings must be addressed.
	DependencyMitigations int
	// ConsoleURL links to the full results.
	ConsoleURL string
}

func (e *FindingsError) Error() string {
	return fmt.Sprintf(
		"Kusari Inspector reported %d code and %d dependency finding(s) that must be addressed: %s",
		e.CodeMitigations, e.DependencyMitigations, e.ConsoleURL)
}

// ExitCode implements the exit-code interface main uses to choose a process
// status without importing this package.
func (e *FindingsError) ExitCode() int { return findingsExitCode }

// findingsResult turns a "do not proceed" verdict into a FindingsError, and is
// only ever called after results have been printed and cached.
//
// The ordering matters: a caller that fails the scan is usually a hook or a CI
// job that needs the findings text to act on. Returning early, before the
// output is written, would hand it an exit code and nothing to show. The flag
// changes the exit status and nothing else.
func findingsResult(failOnFindings bool, analysis *api.SecurityAnalysis, consoleURL string) error {
	if !failOnFindings || analysis == nil || analysis.ShouldProceed {
		return nil
	}
	return &FindingsError{
		CodeMitigations:       len(analysis.RequiredCodeMitigations),
		DependencyMitigations: len(analysis.RequiredDependencyMitigations),
		ConsoleURL:            consoleURL,
	}
}
