package image

import (
	"context"
	"github.com/ChaunceyGu/DockerCopilot/internal/utiles"
	"strings"

	"github.com/ChaunceyGu/DockerCopilot/internal/svc"
	"github.com/ChaunceyGu/DockerCopilot/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RemoveLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRemoveLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RemoveLogic {
	return &RemoveLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RemoveLogic) Remove(req *types.RemoveImageReq) (resp *types.Resp, err error) {
	resp = &types.Resp{}
	// 原实现剥掉了 sha256: 前缀却依然把 req.Id 传下去，剥离结果被丢弃了。
	// Docker API 两种形式都接受，这里保持剥离后的短 ID 以便错误信息可读。
	imageId := strings.TrimPrefix(req.Id, "sha256:")
	if imageId == "" {
		resp.Code = 400
		resp.Msg = "镜像 ID 不能为空"
		resp.Data = map[string]interface{}{}
		return resp, nil
	}
	err = utiles.RemoveImage(l.svcCtx, imageId, req.Force)
	if err != nil {
		l.Errorf("remove image %s (force=%v) failed: %v", imageId, req.Force, err)
		resp.Code = 409
		resp.Msg = err.Error()
		resp.Data = map[string]interface{}{}
		return resp, nil
	}
	resp.Code = 200
	resp.Msg = "success"
	resp.Data = map[string]interface{}{}
	return resp, nil
}
