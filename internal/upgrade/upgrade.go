// Package upgrade replaces the running mctop binary with the latest GitHub
// release: it finds the matching asset, verifies its checksum, and swaps the
// executable in place.
package upgrade

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"runtime"
	"strings"

	"github.com/minio/selfupdate"
)

const releaseAPI = "https://api.github.com/repos/mctop-org/mctop/releases/latest"

type release struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

// Run updates the binary to the latest release. current is this build's version.
// It returns the latest tag and whether an update was applied; updated is false
// when already current.
func Run(ctx context.Context, current string) (tag string, updated bool, err error) {
	if runtime.GOOS == "windows" {
		return "", false, fmt.Errorf("self-update is not supported on windows; reinstall from the releases page")
	}

	rel, err := latest(ctx)
	if err != nil {
		return "", false, err
	}
	if rel.TagName == "v"+current || rel.TagName == current {
		return rel.TagName, false, nil
	}

	archive := fmt.Sprintf("mctop_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	assetURL, sumsURL := assetURLs(rel, archive)
	if assetURL == "" {
		return "", false, fmt.Errorf("no release asset for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	tarball, err := download(ctx, assetURL)
	if err != nil {
		return "", false, err
	}
	if sumsURL == "" {
		return "", false, fmt.Errorf("release is missing checksums, refusing to update")
	}
	if err := verify(ctx, sumsURL, archive, tarball); err != nil {
		return "", false, err
	}

	binary, err := extractBinary(tarball)
	if err != nil {
		return "", false, err
	}
	if err := selfupdate.Apply(bytes.NewReader(binary), selfupdate.Options{}); err != nil {
		return "", false, fmt.Errorf("apply update: %w", err)
	}
	return rel.TagName, true, nil
}

func latest(ctx context.Context) (*release, error) {
	resp, err := get(ctx, releaseAPI, "application/vnd.github+json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github release lookup failed: %s", resp.Status)
	}
	var rel release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

func assetURLs(rel *release, archive string) (asset, sums string) {
	for _, a := range rel.Assets {
		switch a.Name {
		case archive:
			asset = a.URL
		case "checksums.txt":
			sums = a.URL
		}
	}
	return asset, sums
}

func download(ctx context.Context, url string) ([]byte, error) {
	resp, err := get(ctx, url, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// get issues a request with the User-Agent header the GitHub API requires.
func get(ctx context.Context, url, accept string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "mctop")
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	return http.DefaultClient.Do(req)
}

// verify confirms the tarball's sha256 matches the entry for archive in the
// release checksums, so a tampered or truncated download is never installed.
func verify(ctx context.Context, sumsURL, archive string, tarball []byte) error {
	sums, err := download(ctx, sumsURL)
	if err != nil {
		return err
	}
	want := ""
	for _, line := range strings.Split(string(sums), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == archive {
			want = fields[0]
			break
		}
	}
	if want == "" {
		return fmt.Errorf("no checksum listed for %s", archive)
	}
	got := sha256.Sum256(tarball)
	if hex.EncodeToString(got[:]) != want {
		return fmt.Errorf("checksum mismatch, refusing to update")
	}
	return nil
}

func extractBinary(tarball []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(tarball))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if path.Base(hdr.Name) == "mctop" {
			return io.ReadAll(tr)
		}
	}
	return nil, fmt.Errorf("mctop binary not found in release archive")
}
