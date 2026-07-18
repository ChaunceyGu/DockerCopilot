package container

import (
	"context"
	"strings"

	"github.com/ChaunceyGu/DockerCopilot/internal/svc"
	"github.com/ChaunceyGu/DockerCopilot/internal/types"
	"github.com/ChaunceyGu/DockerCopilot/internal/utiles"
	"github.com/docker/docker/api/types/image"

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

	// 2. 获取容器使用的镜像ID
	imageID := containerInspect.Image
	if imageID == "" {
		resp.Code = 400
		resp.Msg = "容器未关联镜像"
		resp.Data = map[string]interface{}{}
		return resp, nil
	}

	// 3. 通过 Docker API 获取镜像详情
	imageInspect, _, err := l.svcCtx.DockerClient.ImageInspectWithRaw(context.Background(), imageID)
	if err != nil {
		resp.Code = 500
		resp.Msg = "获取镜像详情失败"
		resp.Data = map[string]interface{}{"error": err.Error()}
		return resp, nil
	}

	// 4. 构造 types.Image 结构
	var imageName, imageTag string
	if len(imageInspect.RepoTags) > 0 {
		parts := strings.Split(imageInspect.RepoTags[0], ":")
		if len(parts) == 2 {
			imageName = parts[0]
			imageTag = parts[1]
		}
	} else if len(imageInspect.RepoDigests) > 0 {
		imageName = strings.Split(imageInspect.RepoDigests[0], "@")[0]
		imageTag = "None"
	}

	if imageName == "" {
		resp.Code = 400
		resp.Msg = "无法解析镜像名称"
		resp.Data = map[string]interface{}{}
		return resp, nil
	}

	imgInfo := types.Image{
		Summary: image.Summary{
			ID:          imageInspect.ID,
			RepoTags:    imageInspect.RepoTags,
			RepoDigests: imageInspect.RepoDigests,
		},
		ImageName: imageName,
		ImageTag:  imageTag,
	}

	// 5. 调用更新检查
	l.svcCtx.HubImageInfo.CheckUpdate([]types.Image{imgInfo})

	// 6. 读取检查结果
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
