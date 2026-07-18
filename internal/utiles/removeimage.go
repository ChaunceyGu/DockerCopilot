package utiles

import (
	"context"
	"github.com/ChaunceyGu/DockerCopilot/internal/svc"
	"github.com/docker/docker/api/types/image"
)

func RemoveImage(ctx *svc.ServiceContext, imageID string, force bool) error {
	_, err := ctx.DockerClient.ImageRemove(context.Background(), imageID, image.RemoveOptions{Force: force})
	if err != nil {
		return err
	}
	return nil
}
