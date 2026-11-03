package container

import (
	"context"
	"time"

	"github.com/ChaunceyGu/DockerCopilot/internal/svc"
	"github.com/ChaunceyGu/DockerCopilot/internal/types"
	"github.com/ChaunceyGu/DockerCopilot/internal/utiles"
	ref "github.com/distribution/reference"
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
	ctx, cancel := context.WithTimeout(l.ctx, 35*time.Second)
	defer cancel()
	containerInspect, err := l.svcCtx.DockerClient.ContainerInspect(ctx, req.Id)
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
	imageInspect, _, err := l.svcCtx.DockerClient.ImageInspectWithRaw(ctx, imageID)
	if err != nil {
		resp.Code = 500
		resp.Msg = "获取镜像详情失败"
		resp.Data = map[string]interface{}{"error": err.Error()}
		return resp, nil
	}

	// 4. 构造 types.Image 结构
	if containerInspect.Config == nil {
		resp.Code = 400
		resp.Msg = "容器没有镜像配置信息"
		resp.Data = map[string]interface{}{}
		return resp, nil
	}
	imageRef, parseErr := ref.ParseDockerRef(containerInspect.Config.Image)
	if parseErr != nil {
		resp.Code = 400
		resp.Msg = "无法解析容器配置中的镜像名称"
		resp.Data = map[string]interface{}{}
		return resp, nil
	}
	tagged, ok := imageRef.(ref.NamedTagged)
	if !ok {
		resp.Code = 400
		resp.Msg = "容器使用固定摘要的镜像，无法按标签检查更新"
		resp.Data = map[string]interface{}{}
		return resp, nil
	}

	imgInfo := types.Image{
		Summary: image.Summary{
			ID:          imageInspect.ID,
			RepoTags:    imageInspect.RepoTags,
			RepoDigests: imageInspect.RepoDigests,
		},
		ImageName: ref.TrimNamed(imageRef).Name(),
		ImageTag:  tagged.Tag(),
	}

	// 5. 调用更新检查
	haveUpdate, checkErr := l.svcCtx.HubImageInfo.CheckImage(ctx, imgInfo, utiles.LoadSettings().RegistryMirror)
	if checkErr != nil {
		resp.Code = 502
		resp.Msg = checkErr.Error()
		resp.Data = map[string]interface{}{}
		return resp, nil
	}

	resp.Code = 200
	resp.Msg = "检查完成"
	resp.Data = map[string]interface{}{
		"imageID":    imageID,
		"haveUpdate": haveUpdate,
	}
	return resp, nil
}
