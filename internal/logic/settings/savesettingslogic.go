package settings

import (
	"context"

	"github.com/ChaunceyGu/DockerCopilot/internal/svc"
	"github.com/ChaunceyGu/DockerCopilot/internal/types"
	"github.com/ChaunceyGu/DockerCopilot/internal/utiles"

	"github.com/zeromicro/go-zero/core/logx"
)

type SaveSettingsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSaveSettingsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SaveSettingsLogic {
	return &SaveSettingsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SaveSettingsLogic) SaveSettings(req *types.SaveSettingsReq) (resp *types.Resp, err error) {
	resp = &types.Resp{}
	if e := utiles.SaveSettings(utiles.Settings{RegistryMirror: req.RegistryMirror}); e != nil {
		resp.Code = 500
		resp.Msg = "保存设置失败: " + e.Error()
		resp.Data = map[string]interface{}{}
		return resp, nil
	}
	s := utiles.LoadSettings()
	resp.Code = 200
	resp.Msg = "success"
	resp.Data = map[string]interface{}{
		"registryMirror": s.RegistryMirror,
	}
	return resp, nil
}
