package laya

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
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/v1/systemone":
			raw, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(raw), `"model":"multilingual"`) {
				t.Errorf("model was not pinned: %s", raw)
			}
			sawModel = "multilingual"
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
	if client.Name() != "laya-multilingual" {
		t.Fatalf("name %q", client.Name())
	}
	ok, p, err := client.Decide(context.Background(), "hej")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || *p != 0.8 || sawModel != "multilingual" {
		t.Fatalf("ok=%v p=%v model=%s", ok, p, sawModel)
	}
}
