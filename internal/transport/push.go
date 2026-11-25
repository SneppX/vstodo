package transport

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"vstodo/internal/sha1"
)

// PushPack sends a pack and one branch reference update using Git smart HTTP.
// The caller must verify fast-forward safety before invoking this operation.
func (t *HTTPTransport) PushPack(ctx context.Context, remoteURL, token string, oldID, newID sha1.Digest, refName string, pack []byte) error {
	if len(pack) == 0 {
		return errors.New("remote: cannot push an empty pack")
	}
	if !validPushBranchRef(refName) {
		return fmt.Errorf("remote: invalid branch ref %q", refName)
	}
	u, err := validateRemoteURL(remoteURL, token)
	if err != nil {
		return err
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/git-receive-pack"

	command := oldID.Hex() + " " + newID.Hex() + " " + refName + "\x00report-status side-band-64k agent=vstodo/0.1 ofs-delta\n"
	var requestBody bytes.Buffer
	if _, err := fmt.Fprintf(&requestBody, "%04x%s", len(command)+4, command); err != nil {
		return err
	}
	requestBody.WriteString("0000")
	requestBody.Write(pack)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(requestBody.Bytes()))
	if err != nil {
		return fmt.Errorf("remote: create push request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-git-receive-pack-request")
	req.Header.Set("Accept", "application/x-git-receive-pack-result")
	req.Header.Set("User-Agent", "vstodo/0.1")
	if token != "" {
		req.SetBasicAuth("x-access-token", token)
	}
	client := t.Client
	if client == nil {
		client = NewHTTPTransport().Client
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("remote: push pack: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("remote: push returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	if contentType := resp.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "application/x-git-receive-pack-result") {
		return fmt.Errorf("remote: unexpected push content type %q", contentType)
	}
	return parseReceivePackStatus(resp.Body, refName)
}

func validPushBranchRef(refName string) bool {
	const prefix = "refs/heads/"
	if !strings.HasPrefix(refName, prefix) || strings.HasSuffix(refName, "/") || strings.HasSuffix(refName, ".") || strings.Contains(refName, "//") || strings.Contains(refName, "..") || strings.Contains(refName, "@{") {
		return false
	}
	suffix := strings.TrimPrefix(refName, prefix)
	if suffix == "" || suffix == "@" {
		return false
	}
	for _, component := range strings.Split(suffix, "/") {
		if component == "" || component == "." || component == ".." || strings.HasPrefix(component, ".") || strings.HasSuffix(component, ".lock") {
			return false
		}
		for _, r := range component {
			if r <= 0x20 || r == 0x7f || strings.ContainsRune("~^:?*[\\", r) {
				return false
			}
		}
	}
	return true
}

func parseReceivePackStatus(body io.Reader, refName string) error {
	r := bufio.NewReader(body)
	unpackOK, refOK := false, false
	for {
		packet, err := readPktLine(r)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if packet.flush || len(packet.data) == 0 {
			continue
		}
		data := packet.data
		if data[0] >= 1 && data[0] <= 3 {
			channel := data[0]
			data = data[1:]
			if channel == 2 {
				continue
			}
			if channel == 3 {
				return fmt.Errorf("remote: push failed: %s", strings.TrimSpace(string(data)))
			}
		}
		line := strings.TrimSpace(string(data))
		switch {
		case line == "unpack ok":
			unpackOK = true
		case strings.HasPrefix(line, "unpack "):
			return fmt.Errorf("remote: unpack failed: %s", strings.TrimPrefix(line, "unpack "))
		case strings.HasPrefix(line, "ok ") && strings.TrimPrefix(line, "ok ") == refName:
			refOK = true
		case strings.HasPrefix(line, "ng "):
			return fmt.Errorf("remote: %s", line)
		}
	}
	if !unpackOK {
		return errors.New("remote: server did not confirm pack unpacking")
	}
	if !refOK {
		return fmt.Errorf("remote: server did not confirm update of %s", refName)
	}
	return nil
}
