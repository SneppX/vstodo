package transport

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vstodo/internal/sha1"
)

func packetLine(s string) string { return fmt.Sprintf("%04x%s", len(s)+4, s) }

func TestHTTPTransportListRefsParsesV1Advertisement(t *testing.T) {
	const head = "1111111111111111111111111111111111111111"
	const main = "2222222222222222222222222222222222222222"
	body := packetLine("# service=git-upload-pack\n") + "0000" +
		packetLine(head+" HEAD\x00multi_ack side-band symref=HEAD:refs/heads/main\n") +
		packetLine(main+" refs/heads/main\n") + "0000"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/repo.git/info/refs" || r.URL.Query().Get("service") != "git-upload-pack" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		if r.Header.Get("Accept") != "application/x-git-upload-pack-advertisement" {
			t.Fatalf("unexpected Accept header: %q", r.Header.Get("Accept"))
		}
		w.Header().Set("Content-Type", "application/x-git-upload-pack-advertisement")
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()
	refs, err := NewHTTPTransport().ListRefs(context.Background(), server.URL+"/repo.git", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 2 || refs[0].Name != "HEAD" || refs[1].Name != "refs/heads/main" {
		t.Fatalf("unexpected refs: %#v", refs)
	}
	if refs[0].Hash.Hex() != head || refs[1].Hash.Hex() != main {
		t.Fatalf("unexpected object IDs: %#v", refs)
	}
}

func TestHTTPTransportListRefsUsesTokenBasicAuth(t *testing.T) {
	body := packetLine("3333333333333333333333333333333333333333 refs/heads/main\x00report-status\n") + "0000"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "x-access-token" || pass != "secret-token" {
			t.Fatalf("unexpected basic auth: %q %q %v", user, pass, ok)
		}
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()
	if _, err := NewHTTPTransport().ListRefs(context.Background(), server.URL, "secret-token"); err != nil {
		t.Fatal(err)
	}
}

func TestHTTPTransportRejectsInvalidAndCredentialBearingURLs(t *testing.T) {
	for _, raw := range []string{"not-a-url", "file:///tmp/repo", "https://user:pass@example.com/repo", "https://example.com/repo?token=x"} {
		if _, err := NewHTTPTransport().ListRefs(context.Background(), raw, ""); err == nil {
			t.Errorf("expected URL rejection for %q", raw)
		}
	}
}

func TestParseUploadPackAdvertisementRejectsMalformedPacket(t *testing.T) {
	if _, err := parseUploadPackAdvertisement(strings.NewReader("zzzz")); err == nil {
		t.Fatal("expected malformed packet-line error")
	}
}

func TestHTTPTransportFetchPackExtractsSidebandBytes(t *testing.T) {
	const want = "1111111111111111111111111111111111111111"
	wantID, err := sha1.FromHex(want)
	if err != nil {
		t.Fatal(err)
	}
	fixture := []byte("PACK fixture bytes")
	response := packetLine(string(append([]byte{1}, fixture...))) + "0000"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/repo.git/git-upload-pack" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), "want "+want+" multi_ack_detailed side-band-64k ofs-delta") || !strings.Contains(string(body), "done\n") {
			t.Fatalf("unexpected upload-pack request: %q", body)
		}
		w.Header().Set("Content-Type", "application/x-git-upload-pack-result")
		_, _ = io.WriteString(w, response)
	}))
	defer server.Close()
	got, err := NewHTTPTransport().FetchPack(context.Background(), server.URL+"/repo.git", "", []sha1.Digest{wantID})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, fixture) {
		t.Fatalf("got pack %q, want %q", got, fixture)
	}
}

func TestParseSidebandPackHandlesProgressAndErrors(t *testing.T) {
	progress := packetLine(string(append([]byte{2}, []byte("counting objects\n")...)))
	pack, err := parseSidebandPack(strings.NewReader(progress))
	if err == nil || pack != nil {
		t.Fatal("expected empty-pack error after progress-only response")
	}
	fatal := packetLine(string(append([]byte{3}, []byte("denied\n")...)))
	if _, err := parseSidebandPack(strings.NewReader(fatal)); err == nil || !strings.Contains(err.Error(), "denied") {
		t.Fatalf("expected remote fatal error, got %v", err)
	}
}
