package selfupdate

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	LatestURL      = "https://api.github.com/repos/dnitros/pironman/releases/latest"
	BinaryAsset    = "pironman-linux-arm64"
	ChecksumsAsset = "checksums.txt"

	maxDownloadBytes = 64 << 20
)

type Release struct {
	Tag          string
	BinaryURL    string
	ChecksumsURL string
}

func Latest(client *http.Client, url string) (Release, error) {
	body, err := get(client, url)
	var status statusError
	if errors.As(err, &status) && status.code == http.StatusNotFound {
		return Release{}, fmt.Errorf("no published release at %s (none tagged yet, or the repository is private)", url)
	}
	if err != nil {
		return Release{}, fmt.Errorf("fetch latest release: %w", err)
	}
	var payload struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return Release{}, fmt.Errorf("parse latest release: %w", err)
	}
	if payload.TagName == "" {
		return Release{}, fmt.Errorf("latest release has no tag")
	}

	rel := Release{Tag: payload.TagName}
	for _, a := range payload.Assets {
		switch a.Name {
		case BinaryAsset:
			rel.BinaryURL = a.URL
		case ChecksumsAsset:
			rel.ChecksumsURL = a.URL
		}
	}
	if rel.BinaryURL == "" {
		return Release{}, fmt.Errorf("release %s has no %s asset", rel.Tag, BinaryAsset)
	}
	if rel.ChecksumsURL == "" {
		return Release{}, fmt.Errorf("release %s has no %s asset", rel.Tag, ChecksumsAsset)
	}
	return rel, nil
}

func Download(client *http.Client, rel Release) ([]byte, error) {
	checksums, err := get(client, rel.ChecksumsURL)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", ChecksumsAsset, err)
	}
	want, err := checksumFor(checksums, BinaryAsset)
	if err != nil {
		return nil, err
	}
	bin, err := get(client, rel.BinaryURL)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", BinaryAsset, err)
	}
	sum := sha256.Sum256(bin)
	if got := hex.EncodeToString(sum[:]); got != want {
		return nil, fmt.Errorf("%s checksum mismatch: got %s, want %s", BinaryAsset, got, want)
	}
	return bin, nil
}

func Replace(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".new-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write %s: %w", tmp.Name(), err)
	}
	if err := tmp.Chmod(0o755); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod %s: %w", tmp.Name(), err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync %s: %w", tmp.Name(), err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close %s: %w", tmp.Name(), err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}

func checksumFor(checksums []byte, name string) (string, error) {
	sc := bufio.NewScanner(bytes.NewReader(checksums))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name {
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", fmt.Errorf("%s has no entry for %s", ChecksumsAsset, name)
}

type statusError struct {
	url    string
	code   int
	status string
}

func (e statusError) Error() string {
	return fmt.Sprintf("GET %s: %s", e.url, e.status)
}

func get(client *http.Client, url string) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, statusError{url: url, code: resp.StatusCode, status: resp.Status}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxDownloadBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", url, err)
	}
	if len(body) > maxDownloadBytes {
		return nil, fmt.Errorf("GET %s: response larger than %d bytes", url, maxDownloadBytes)
	}
	return body, nil
}
