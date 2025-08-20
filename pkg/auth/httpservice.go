// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package auth

import (
	"html/template"
	"net/http"
)

func handleCallbackv2(w http.ResponseWriter, r *http.Request, expectedState string, callbackRes chan callbackResult, redirectUrl string) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	errorParam := r.URL.Query().Get("error")

	if errorParam != "" {
		callbackRes <- callbackResult{Error: NewAuthError(ErrAuthFlow, "OAuth error: "+errorParam)}
		http.Error(w, "Authentication failed", http.StatusBadRequest)
		return
	}

	if state != expectedState {
		callbackRes <- callbackResult{Error: NewAuthError(ErrAuthFlow, "invalid state parameter")}
		http.Error(w, "Invalid state", http.StatusBadRequest)
		return
	}

	if code == "" {
		callbackRes <- callbackResult{Error: NewAuthError(ErrAuthFlow, "no authorization code received")}
		http.Error(w, "No code received", http.StatusBadRequest)
		return
	}

	tmpl := template.Must(template.New("success").Parse(getPostLoginHtml(redirectUrl)))
	if err := tmpl.Execute(w, nil); err != nil {
		http.Error(w, "Internal template error", http.StatusInternalServerError)
		return
	}

	callbackRes <- callbackResult{Code: code}
}
