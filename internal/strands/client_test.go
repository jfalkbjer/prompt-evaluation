package strands

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPingAndDecide(t *testing.T) {
	var sawModel string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok","model":"strands-decider-2B-hobson-v19"}`))
		case "/v1/systemone":
			raw, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(raw), `"model":"strands-decider-latest"`) {
				t.Errorf("model was not pinned: %s", raw)
			}
			sawModel = "strands-decider-latest"
			_, _ = w.Write([]byte(`{"answers":{"pii":{"type":"noul","noul":0.8}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := New(srv.URL)
	if err := client.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	if client.Name() != "strands-decider-2B-hobson-v19" {
		t.Fatalf("name %q", client.Name())
	}
	ok, p, err := client.Decide(context.Background(), "hej")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || *p != 0.8 || sawModel != "strands-decider-latest" {
		t.Fatalf("ok=%v p=%v model=%s", ok, p, sawModel)
	}
}
