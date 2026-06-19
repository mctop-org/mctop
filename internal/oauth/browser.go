package oauth

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"
)

// awaitCode serves the one redirect the authorization server makes back to us:
// it opens the user's browser at authURL, waits for the /callback request, and
// returns the authorization code once state is confirmed.
func awaitCode(ctx context.Context, listener net.Listener, state, authURL string) (string, error) {
	type result struct {
		code string
		err  error
	}
	done := make(chan result, 1)

	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/callback" {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query()
		if e := q.Get("error"); e != "" {
			finish(w, "Login failed. You can close this tab.")
			done <- result{err: fmt.Errorf("authorization failed: %s", e)}
			return
		}
		if q.Get("state") != state {
			finish(w, "Login failed. You can close this tab.")
			done <- result{err: fmt.Errorf("state mismatch, possible CSRF")}
			return
		}
		finish(w, "Logged in. You can close this tab and return to the terminal.")
		done <- result{code: q.Get("code")}
	})}
	go srv.Serve(listener)

	fmt.Println("Opening your browser to log in. If it does not open, visit:")
	fmt.Println("  " + authURL)
	openBrowser(authURL)

	select {
	case <-ctx.Done():
		srv.Close()
		return "", ctx.Err()
	case res := <-done:
		// Shut down gracefully so the success page finishes sending before the
		// process exits, rather than the browser seeing a dropped connection.
		shutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
		return res.code, res.err
	}
}

func finish(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, "<!doctype html><title>mctop</title><body style=\"font-family:system-ui;padding:3rem\"><p>%s</p>", msg)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// openBrowser tries to open url in the default browser, ignoring failure since
// awaitCode also prints the url for manual use.
func openBrowser(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "windows":
		cmd, args = "rundll32", []string{"url.dll,FileProtocolHandler"}
	default:
		cmd = "xdg-open"
	}
	_ = exec.Command(cmd, append(args, url)...).Start()
}
