package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/mctop/mctop/internal/oauth"
)

// withAuth adds a cached OAuth bearer token to the headers for an http(s)
// target, unless the caller already set Authorization. A missing login is not
// an error: the request proceeds and the server decides, so unauthenticated
// servers keep working.
func withAuth(ctx context.Context, target string, headers map[string]string) map[string]string {
	if !isHTTP(target) || hasAuthorization(headers) {
		return headers
	}
	creds, err := oauth.Load(target)
	if err != nil || creds == nil {
		return headers
	}
	token, changed, err := creds.AccessToken(ctx)
	if err != nil {
		return headers
	}
	if changed {
		_ = oauth.Save(target, creds)
	}
	if headers == nil {
		headers = make(map[string]string, 1)
	}
	headers["Authorization"] = "Bearer " + token
	return headers
}

// hintLogin nudges the user toward logging in when an http target rejects the
// request and no token was attached.
func hintLogin(target string, err error) {
	if !isHTTP(target) || hasLogin(target) {
		return
	}
	msg := err.Error()
	if strings.Contains(msg, "Unauthorized") || strings.Contains(msg, "401") || strings.Contains(msg, "Forbidden") {
		fmt.Fprintf(os.Stderr, "mctop: this server needs authorization; run: mctop login %s\n", target)
	}
}

func hasLogin(target string) bool {
	creds, err := oauth.Load(target)
	return err == nil && creds != nil
}

func isHTTP(target string) bool {
	return strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://")
}

func hasAuthorization(headers map[string]string) bool {
	for k := range headers {
		if strings.EqualFold(k, "Authorization") {
			return true
		}
	}
	return false
}
