package container

import (
	"context"

	"github.com/ChaunceyGu/DockerCopilot/internal/svc"
	"github.com/ChaunceyGu/DockerCopilot/internal/types"
	"github.com/ChaunceyGu/DockerCopilot/internal/utiles"

	"github.com/zeromicro/go-zero/core/logx"
)

type CheckUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCheckUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CheckUpdateLogic {
	return &CheckUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// CheckUpdate 手动检查单个容器的镜像是否有更新
func (l *CheckUpdateLogic) CheckUpdate(req *types.IdReq) (resp *types.Resp, err error) {
	resp = &types.Resp{}

	// 1. 获取容器信息
	containerInspect, err := utiles.GetContainerInspect(l.svcCtx, req.Id)
	if err != nil {
		resp.Code = 500
		resp.Msg = "获取容器信息失败"
		resp.Data = map[string]interface{}{"error": err.Error()}
		return resp, nil
	}

	// 2. 构造镜像信息结构
	imageID := containerInspect.Image
	if imageID == "" {
		resp.Code = 400
		resp.Msg = "容器未关联镜像"
		resp.Data = map[string]interface{}{}
		return resp, nil
	}

	// 获取镜像详情
	imageInfo, err := utiles.GetImageInfo(l.svcCtx, imageID)
	if err != nil {
		resp.Code = 500
		resp.Msg = "获取镜像详情失败"
		resp.Data = map[string]interface{}{"error": err.Error()}
		return resp, nil
	}

	// 3. 调用更新检查
	l.svcCtx.HubImageInfo.CheckUpdate([]types.Image{imageInfo})

	// 4. 读取检查结果
	updateStatus, exists := l.svcCtx.HubImageInfo.Get(imageID)
	haveUpdate := exists && updateStatus.NeedUpdate

	resp.Code = 200
	resp.Msg = "检查完成"
	resp.Data = map[string]interface{}{
		"imageID":    imageID,
		"haveUpdate": haveUpdate,
	}
	return resp, nil
}
