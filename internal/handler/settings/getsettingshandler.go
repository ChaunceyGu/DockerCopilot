package settings

import (
	"net/http"

	"github.com/ChaunceyGu/DockerCopilot/internal/logic/settings"
	"github.com/ChaunceyGu/DockerCopilot/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func GetSettingsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := settings.NewGetSettingsLogic(r.Context(), svcCtx)
		resp, err := l.GetSettings()
		if err != nil {
			httpx.WriteJson(w, resp.Code, resp)
		} else {
			httpx.WriteJson(w, resp.Code, resp)
		}
	}
}
