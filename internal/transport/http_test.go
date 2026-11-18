package transport

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
