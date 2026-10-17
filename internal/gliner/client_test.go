package gliner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPingDecideAndChoose(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			_, _ = w.Write([]byte(`{"status":"ok","model":"GLiNER2.5-multi-Decide"}`))
		case "/v1/systemone":
			var body struct {
				Questions map[string]struct {
					Type string `json:"type"`
				} `json:"questions"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.Questions["pii"].Type == "choice" {
				_, _ = w.Write([]byte(`{"answers":{"pii":{"type":"choice","choice":"secret","confidence":0.7,"probabilities":{"secret":0.7,"none":0.3}}}}`))
				return
			}
			_, _ = w.Write([]byte(`{"answers":{"pii":{"type":"noul","noul":0.2}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := New(srv.URL)
	if err := client.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	if client.Name() != "GLiNER2.5-multi-Decide" {
		t.Fatalf("name %q", client.Name())
	}
	ok, p, err := client.Decide(context.Background(), "hej")
	if err != nil {
		t.Fatal(err)
	}
	if ok || *p != 0.2 {
		t.Fatalf("ok=%v p=%v", ok, *p)
	}
	got, err := client.Choose(context.Background(), "PIN 4821")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "secret" || got.Probability != 0.7 || got.Confidence != 0.7 {
		t.Fatalf("choice %+v", got)
	}
}
