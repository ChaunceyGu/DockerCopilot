package container

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ChaunceyGu/DockerCopilot/internal/svc"
	"github.com/docker/docker/client"
	"github.com/zeromicro/go-zero/rest/pathvar"
)

func TestCheckUpdateAcceptsEmptyAndNullBody(t *testing.T) {
	docker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"container not found"}`, http.StatusNotFound)
	}))
	defer docker.Close()
	cli, err := client.NewClientWithOpts(client.WithHost(docker.URL), client.WithVersion("1.44"))
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()
	handler := CheckUpdateHandler(&svc.ServiceContext{DockerClient: cli})
	for _, body := range []string{"", "null"} {
		t.Run(body, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/container/example/check-update", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = pathvar.WithVars(req, map[string]string{"id": "example"})
			response := httptest.NewRecorder()
			handler(response, req)
			if response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), "获取容器信息失败") {
				t.Fatalf("request did not reach Docker: status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}
