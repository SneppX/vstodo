// Package transport provides local and HTTP smart transports for VSTodo.
package transport

import (
	"bufio"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"vstodo/internal/sha1"
)

// RemoteRef is an advertised reference on an HTTP Git remote.
type RemoteRef struct {
	Name string
	Hash sha1.Digest
}

// HTTPTransport reads references from Git smart-HTTP upload-pack endpoints.
type HTTPTransport struct {
	Client *http.Client
}

// NewHTTPTransport returns an HTTP transport with bounded request timeouts.
func NewHTTPTransport() *HTTPTransport {
	return &HTTPTransport{Client: &http.Client{Timeout: 20 * time.Second}}
}

// ListRefs requests the v0/v1 upload-pack advertisement from a remote.
func (t *HTTPTransport) ListRefs(ctx context.Context, remoteURL, token string) ([]RemoteRef, error) {
	u, err := url.Parse(remoteURL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return nil, errors.New("remote: URL must be an absolute HTTP or HTTPS URL")
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("remote: credentials, query strings, and fragments are not allowed in remote URLs")
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/info/refs"
	q := u.Query()
	q.Set("service", "git-upload-pack")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("remote: create request: %w", err)
	}
	req.Header.Set("Accept", "application/x-git-upload-pack-advertisement")
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
		return nil, fmt.Errorf("remote: request refs: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("remote: server returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return parseUploadPackAdvertisement(io.LimitReader(resp.Body, 8<<20))
}

type pktLine struct {
	data  []byte
	flush bool
}

func readPktLine(r *bufio.Reader) (pktLine, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(r, header); err != nil {
		return pktLine{}, err
	}
	n, err := strconv.ParseUint(string(header), 16, 16)
	if err != nil {
		return pktLine{}, fmt.Errorf("remote: invalid packet-line length %q", header)
	}
	switch n {
	case 0:
		return pktLine{flush: true}, nil
	case 1, 2:
		return pktLine{}, fmt.Errorf("remote: unsupported packet-line control %q", header)
	}
	if n < 4 || n > 65520 {
		return pktLine{}, fmt.Errorf("remote: packet-line length %d is out of range", n)
	}
	data := make([]byte, int(n)-4)
	if _, err := io.ReadFull(r, data); err != nil {
		return pktLine{}, fmt.Errorf("remote: truncated packet-line: %w", err)
	}
	return pktLine{data: data}, nil
}

func parseUploadPackAdvertisement(body io.Reader) ([]RemoteRef, error) {
	r := bufio.NewReader(body)
	var refs []RemoteRef
	serviceSeen := false
	for {
		packet, err := readPktLine(r)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if packet.flush {
			continue
		}
		line := strings.TrimSuffix(string(packet.data), "\n")
		if !serviceSeen && strings.HasPrefix(line, "# service=git-upload-pack") {
			serviceSeen = true
			continue
		}
		if strings.HasPrefix(line, "ERR ") {
			return nil, fmt.Errorf("remote: %s", strings.TrimPrefix(line, "ERR "))
		}
		line = strings.SplitN(line, "\x00", 2)[0]
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if len(fields[0]) != 40 {
			return nil, fmt.Errorf("remote: invalid object ID %q", fields[0])
		}
		if _, err := hex.DecodeString(fields[0]); err != nil {
			return nil, fmt.Errorf("remote: invalid object ID %q", fields[0])
		}
		id, err := sha1.FromHex(fields[0])
		if err != nil {
			return nil, err
		}
		refs = append(refs, RemoteRef{Name: fields[1], Hash: id})
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].Name < refs[j].Name })
	return refs, nil
}
