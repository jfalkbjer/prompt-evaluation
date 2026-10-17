package decider

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPingAndDecide(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			_, _ = w.Write([]byte(`{"ok":true,"model":"Mapika/decider-4b","device":"mps"}`))
		case "/v1/systemone":
			raw, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(raw), `"model":"decider"`) {
				t.Errorf("model was not pinned: %s", raw)
			}
			_, _ = w.Write([]byte(`{"model":"decider-v2.1","answers":{"pii":{"type":"noul","noul":0.9}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := New(srv.URL)
	if err := client.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	if client.Name() != "decider-4b" {
		t.Fatalf("name %q", client.Name())
	}
	ok, p, err := client.Decide(context.Background(), "hej")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || *p != 0.9 {
		t.Fatalf("ok=%v p=%v", ok, *p)
	}
}

func TestPingNotReady(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":false,"model":"Mapika/decider-4b"}`))
	}))
	defer srv.Close()

	if err := New(srv.URL).Ping(context.Background()); err == nil {
		t.Fatal("expected an error while the model is loading")
	}
}
