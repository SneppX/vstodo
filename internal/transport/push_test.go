package transport

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vstodo/internal/sha1"
)

func TestHTTPTransportPushPackSendsUpdateAndParsesStatus(t *testing.T) {
	oldID, _ := sha1.FromHex(strings.Repeat("1", 40))
	newID, _ := sha1.FromHex(strings.Repeat("2", 40))
	pack := []byte("PACK test payload")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/repo.git/git-receive-pack" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		if r.Header.Get("Content-Type") != "application/x-git-receive-pack-request" {
			t.Fatalf("unexpected content type: %q", r.Header.Get("Content-Type"))
		}
		user, pass, ok := r.BasicAuth()
		if !ok || user != "x-access-token" || pass != "secret" {
			t.Fatalf("unexpected auth: %q %q %v", user, pass, ok)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), oldID.Hex()+" "+newID.Hex()+" refs/heads/main\x00report-status") || !strings.HasSuffix(string(body), string(pack)) {
			t.Fatalf("unexpected push body: %q", body)
		}
		w.Header().Set("Content-Type", "application/x-git-receive-pack-result")
		_, _ = io.WriteString(w, packetLine("unpack ok\n")+packetLine("ok refs/heads/main\n")+"0000")
	}))
	defer server.Close()
	if err := NewHTTPTransport().PushPack(context.Background(), server.URL+"/repo.git", "secret", oldID, newID, "refs/heads/main", pack); err != nil {
		t.Fatal(err)
	}
}

func TestParseReceivePackStatusRejectsRemoteFailures(t *testing.T) {
	for _, response := range []string{
		packetLine("unpack corrupt\n") + "0000",
		packetLine("unpack ok\n") + packetLine("ng refs/heads/main non-fast-forward\n") + "0000",
		packetLine("unpack ok\n") + "0000",
	} {
		if err := parseReceivePackStatus(strings.NewReader(response), "refs/heads/main"); err == nil {
			t.Fatalf("expected push status failure for %q", response)
		}
	}
}

func TestValidPushBranchRefRejectsUnsafeNames(t *testing.T) {
	for _, name := range []string{"refs/tags/v1", "refs/heads/../escape", "refs/heads/a//b", "refs/heads/.hidden", "refs/heads/name.lock", "refs/heads/trailing.", "refs/heads/a@{b}", "refs/heads/bad name"} {
		if validPushBranchRef(name) {
			t.Errorf("accepted invalid branch ref %q", name)
		}
	}
	for _, name := range []string{"refs/heads/main", "refs/heads/release/v1", "refs/heads/feature/topic"} {
		if !validPushBranchRef(name) {
			t.Errorf("rejected valid branch ref %q", name)
		}
	}
}
