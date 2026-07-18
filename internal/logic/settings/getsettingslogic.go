package settings

import (
	"context"

	"github.com/ChaunceyGu/DockerCopilot/internal/svc"
	"github.com/ChaunceyGu/DockerCopilot/internal/types"
	"github.com/ChaunceyGu/DockerCopilot/internal/utiles"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetSettingsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetSettingsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSettingsLogic {
	return &GetSettingsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetSettingsLogic) GetSettings() (resp *types.Resp, err error) {
	resp = &types.Resp{}
	s := utiles.LoadSettings()
	resp.Code = 200
	resp.Msg = "success"
	resp.Data = map[string]interface{}{
		"registryMirror": s.RegistryMirror,
	}
	return resp, nil
}
