// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package api

// InspectorScan is a scan from workspace-api's public inspector scan routes.
// The list leaves Analysis out; the single-scan route fills it once Status is
// "success".
type InspectorScan struct {
	ID           string    `json:"id"`
	Status       string    `json:"status"`
	StatusDetail string    `json:"status_detail"`
	Analysis     *Analysis `json:"analysis,omitempty"`
}

type InspectorScanList struct {
	Items []InspectorScan `json:"items"`
}
