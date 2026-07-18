package settings

import (
	"net/http"

	"github.com/ChaunceyGu/DockerCopilot/internal/logic/settings"
	"github.com/ChaunceyGu/DockerCopilot/internal/svc"
	"github.com/ChaunceyGu/DockerCopilot/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func SaveSettingsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.SaveSettingsReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := settings.NewSaveSettingsLogic(r.Context(), svcCtx)
		resp, err := l.SaveSettings(&req)
		if err != nil {
			httpx.WriteJson(w, resp.Code, resp)
		} else {
			httpx.WriteJson(w, resp.Code, resp)
		}
	}
}
