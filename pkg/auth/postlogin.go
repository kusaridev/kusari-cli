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
