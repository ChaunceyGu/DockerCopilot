package utiles

import (
	"context"
	"github.com/ChaunceyGu/DockerCopilot/internal/svc"
	"github.com/docker/docker/api/types"
	"time"
)

func GetContainerInspect(ctx *svc.ServiceContext, id string) (types.ContainerJSON, error) {
	// 加超时，避免 Docker daemon 卡住时 inspect 永久 hang。
	c, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	inspectedContainer, err := ctx.DockerClient.ContainerInspect(c, id)
	if err != nil {
		return types.ContainerJSON{}, err
	}
	return inspectedContainer, nil
}
