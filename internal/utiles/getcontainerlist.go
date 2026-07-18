package utiles

import (
	"context"
	"github.com/docker/docker/api/types/container"
	"github.com/onlyLTY/dockerCopilot/internal/svc"
	MyType "github.com/onlyLTY/dockerCopilot/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
	"time"
)

func GetContainerList(ctx *svc.ServiceContext) ([]MyType.Container, error) {
	// 获取所有容器（包括停止的容器），加超时避免 daemon 卡住时永久 hang。
	c, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dockerContainerList, err := ctx.DockerClient.ContainerList(c, container.ListOptions{
		All: true, // 设置为true来获取所有容器
	})
	if err != nil {
		logx.Errorf("get container list error: %v", err)
		return nil, err
	}
	var containerList []MyType.Container
	for _, dockerContainerInfo := range dockerContainerList {
		containerInfo := MyType.Container{
			Container: dockerContainerInfo,
		}
		containerList = append(containerList, containerInfo)
	}
	return containerList, nil
}

func CheckImageUpdate(ctx *svc.ServiceContext, containerListData []MyType.Container) []MyType.Container {
	for i, v := range containerListData {
		// 通过并发安全的 Get 读取，避免与后台 CheckUpdate 的写操作产生 data race。
		if info, ok := ctx.HubImageInfo.Get(v.ImageID); ok && info.NeedUpdate {
			containerListData[i].Update = true
		}
	}
	return containerListData
}
