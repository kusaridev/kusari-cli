// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package auth

import "fmt"

func getPostLoginHtml(redirectUrl string) string {
	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
  <head>
    <meta http-equiv="refresh" content="0;url=%s" />
    <title></title>
  </head>
 <body></body>
</html>
`, redirectUrl)
}

func getSuccessHtml() string {
	return `
<!DOCTYPE html>
<html>
  <head>
    <title>Authentication Successful</title>
    <style>
      body {
        font-family: -apple-system, BlinkMacSystemFont,"DM Sans", "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
        display: flex;
        justify-content: center;
        align-items: center;
        height: 100vh;
        margin: 0;
        background-color: #0E0816;
      }
      .container {
        text-align: center;
        padding: 2rem;
        background: #190F27;
        border-radius: 8px;
        border: 1px solid rgba(255, 255, 255, 0.2);
        box-shadow: 0 2px 8px rgba(0,0,0,0.1);
      }
      h1 {
        color: rgba(255, 255, 255, 0.7);
        margin-bottom: 1rem;
      }
      p {
        color: rgba(255, 255, 255, 0.5);
      }
    </style>
  </head>
  <body>
    <div class="container">
      <h1>Authentication Successful!</h1>
      <p>You can close this window and return to the CLI.</p>
    </div>
  </body>
</html>
`
}
