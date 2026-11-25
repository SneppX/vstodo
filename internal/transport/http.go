// Package transport provides local and HTTP smart transports for VSTodo.
package transport

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
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

// FetchPack requests objects reachable from the supplied tips using Git smart HTTP v0.
func (t *HTTPTransport) FetchPack(ctx context.Context, remoteURL, token string, wants []sha1.Digest) ([]byte, error) {
	return t.FetchPackWithHaves(ctx, remoteURL, token, wants, nil)
}

// FetchPackWithHaves negotiates against locally available commits before requesting
// the pack, allowing the server to omit objects already present in the client.
func (t *HTTPTransport) FetchPackWithHaves(ctx context.Context, remoteURL, token string, wants, haves []sha1.Digest) ([]byte, error) {
	if len(wants) == 0 {
		return nil, errors.New("remote: at least one object ID is required")
	}
	u, err := validateRemoteURL(remoteURL, token)
	if err != nil {
		return nil, err
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/git-upload-pack"

	var request bytes.Buffer
	for i, want := range wants {
		line := "want " + want.Hex()
		if i == 0 {
			line += " multi_ack_detailed side-band-64k ofs-delta agent=vstodo/0.1"
		}
		line += "\n"
		if _, err := fmt.Fprintf(&request, "%04x%s", len(line)+4, line); err != nil {
			return nil, err
		}
	}
	request.WriteString("0000")
	seenHaves := make(map[sha1.Digest]struct{}, len(haves))
	for _, have := range haves {
		if have == sha1.Zero {
			continue
		}
		if _, exists := seenHaves[have]; exists {
			continue
		}
		seenHaves[have] = struct{}{}
		line := "have " + have.Hex() + "\n"
		if _, err := fmt.Fprintf(&request, "%04x%s", len(line)+4, line); err != nil {
			return nil, err
		}
	}
	request.WriteString("0009done\n")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(request.Bytes()))
	if err != nil {
		return nil, fmt.Errorf("remote: create fetch request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-git-upload-pack-request")
	req.Header.Set("Accept", "application/x-git-upload-pack-result")
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
		return nil, fmt.Errorf("remote: fetch pack: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("remote: fetch returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	if contentType := resp.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "application/x-git-upload-pack-result") {
		return nil, fmt.Errorf("remote: unexpected fetch content type %q", contentType)
	}
	return parseSidebandPack(io.LimitReader(resp.Body, (1<<30)+1))
}

func parseSidebandPack(body io.Reader) ([]byte, error) {
	const maxPackSize = 1 << 30
	r := bufio.NewReader(body)
	var pack bytes.Buffer
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
		if len(packet.data) == 0 {
			continue
		}
		if strings.HasPrefix(string(packet.data), "NAK") || strings.HasPrefix(string(packet.data), "ACK ") {
			continue
		}
		switch packet.data[0] {
		case 1:
			if pack.Len()+len(packet.data)-1 > maxPackSize {
				return nil, errors.New("remote: pack exceeds 1 GiB safety limit")
			}
			_, _ = pack.Write(packet.data[1:])
		case 2:
			// Progress channel is intentionally not written to stdout.
		case 3:
			return nil, fmt.Errorf("remote: server error: %s", strings.TrimSpace(string(packet.data[1:])))
		default:
			return nil, fmt.Errorf("remote: invalid sideband channel %d", packet.data[0])
		}
	}
	if pack.Len() == 0 {
		return nil, errors.New("remote: server returned no pack data")
	}
	return pack.Bytes(), nil
}

// ListRefs requests the v0/v1 upload-pack advertisement from a remote.
func (t *HTTPTransport) ListRefs(ctx context.Context, remoteURL, token string) ([]RemoteRef, error) {
	return t.listRefs(ctx, remoteURL, token, "git-upload-pack")
}

// ListPushRefs requests the receive-pack advertisement used to negotiate pushes.
func (t *HTTPTransport) ListPushRefs(ctx context.Context, remoteURL, token string) ([]RemoteRef, error) {
	return t.listRefs(ctx, remoteURL, token, "git-receive-pack")
}

func (t *HTTPTransport) listRefs(ctx context.Context, remoteURL, token, service string) ([]RemoteRef, error) {
	u, err := validateRemoteURL(remoteURL, token)
	if err != nil {
		return nil, err
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/info/refs"
	q := u.Query()
	q.Set("service", service)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("remote: create request: %w", err)
	}
	req.Header.Set("Accept", "application/x-"+service+"-advertisement")
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
	expectedType := "application/x-" + service + "-advertisement"
	if contentType := resp.Header.Get("Content-Type"); !strings.HasPrefix(contentType, expectedType) {
		return nil, fmt.Errorf("remote: unexpected ref advertisement content type %q", contentType)
	}
	return parseServiceAdvertisement(io.LimitReader(resp.Body, 8<<20), service)
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
	return parseServiceAdvertisement(body, "git-upload-pack")
}

func parseReceivePackAdvertisement(body io.Reader) ([]RemoteRef, error) {
	return parseServiceAdvertisement(body, "git-receive-pack")
}

func parseServiceAdvertisement(body io.Reader, service string) ([]RemoteRef, error) {
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
		if !serviceSeen && strings.HasPrefix(line, "# service="+service) {
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


// validateRemoteURL rejects ambiguous URLs and prevents credentials from being
// sent over unencrypted network connections. Plain HTTP remains available for
// unauthenticated remotes and loopback-only development servers.
func validateRemoteURL(remoteURL, token string) (*url.URL, error) {
	u, err := url.Parse(remoteURL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return nil, errors.New("remote: URL must be an absolute HTTP or HTTPS URL")
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("remote: credentials, query strings, and fragments are not allowed in remote URLs")
	}
	if token != "" && u.Scheme != "https" {
		host := u.Hostname()
		ip := net.ParseIP(host)
		if !strings.EqualFold(host, "localhost") && (ip == nil || !ip.IsLoopback()) {
			return nil, errors.New("remote: refusing to send an authentication token over unencrypted HTTP; use HTTPS")
		}
	}
	return u, nil
}
