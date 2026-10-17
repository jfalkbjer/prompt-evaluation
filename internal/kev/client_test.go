package kev

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestShortRun(t *testing.T) {
	if got := shortRun("jaredpalmer/kev-0.8b"); got != "kev-0.8b" {
		t.Fatalf("got %q", got)
	}
	if got := shortRun("jaredpalmer/kev-4b@v1.0"); got != "kev-4b" {
		t.Fatalf("got %q", got)
	}
}

func TestDecideThreshold(t *testing.T) {
	tests := []struct {
		name string
		noul float64
		want bool
	}{
		{name: "below", noul: 0.49, want: false},
		{name: "boundary", noul: 0.5, want: true},
		{name: "above", noul: 0.91, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/systemone" || r.Method != http.MethodPost {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
				w.Header().Set("content-type", "application/json")
				_, _ = w.Write([]byte(`{"answers":{"pii":{"type":"noul","noul":` + strconv.FormatFloat(tt.noul, 'f', -1, 64) + `}}}`))
			}))
			defer srv.Close()
			got, p, err := New(srv.URL).Decide(context.Background(), "hej")
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("positive = %v, want %v (p=%v)", got, tt.want, *p)
			}
			if *p != tt.noul {
				t.Fatalf("probability = %v, want %v", *p, tt.noul)
			}
		})
	}
}
