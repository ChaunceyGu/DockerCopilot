package module

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ChaunceyGu/DockerCopilot/internal/types"
	"github.com/docker/docker/api/types/image"
)

func TestCheckImageMirrorAuthenticationAndFailure(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/library/redis/manifests/latest":
			if r.Header.Get("Authorization") != "Bearer test-token" {
				w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer realm="%s/token",service="RegistryService"`, server.URL))
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Header().Set("Docker-Content-Digest", "sha256:remote")
		case "/token":
			if r.URL.Query().Get("service") != "RegistryService" || r.URL.Query().Get("scope") != "repository:library/redis:pull" {
				t.Errorf("unexpected token query: %s", r.URL.RawQuery)
			}
			fmt.Fprint(w, `{"token":"test-token"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	img := types.Image{Summary: image.Summary{ID: "id", RepoDigests: []string{"redis@sha256:remote"}}, ImageName: "redis", ImageTag: "latest"}
	checks := NewImageCheck()
	updated, err := checks.CheckImage(context.Background(), img, server.URL)
	if err != nil || updated {
		t.Fatalf("expected current image, got updated=%v err=%v", updated, err)
	}
	img.RepoDigests[0] = "redis@sha256:old"
	updated, err = checks.CheckImage(context.Background(), img, server.URL)
	if err != nil || !updated {
		t.Fatalf("expected update, got updated=%v err=%v", updated, err)
	}
	img.RepoDigests = nil
	_, err = checks.CheckImage(context.Background(), img, server.URL)
	if err == nil || !strings.Contains(err.Error(), "没有仓库摘要") {
		t.Fatalf("expected missing digest error, got %v", err)
	}
	if cached, ok := checks.Get("id"); !ok || !cached.NeedUpdate {
		t.Fatal("failed check changed previous result")
	}
}

func TestCheckImageRespectsCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(150 * time.Millisecond)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	img := types.Image{Summary: image.Summary{ID: "id", RepoDigests: []string{"redis@sha256:old"}}, ImageName: "redis", ImageTag: "latest"}
	_, err := NewImageCheck().CheckImage(ctx, img, server.URL)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}
