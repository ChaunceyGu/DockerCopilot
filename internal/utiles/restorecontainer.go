package utiles

import (
	"context"
	"encoding/json"
	"github.com/ChaunceyGu/DockerCopilot/internal/svc"
	dockerBackend "github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/image"
	"github.com/zeromicro/go-zero/core/logx"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func RestoreContainer(ctx *svc.ServiceContext, filename string, taskID string) error {
	var backupList []string
	basePath := os.Getenv("BACKUP_DIR") // 从环境变量中获取备份目录
	if basePath == "" {
		basePath = "/data/backups" // 如果环境变量未设置，使用默认值
	}
	fullPath := filepath.Join(basePath, filename)
	oldProgress := svc.TaskProgress{
		TaskID:     taskID,
		Percentage: 0,
		Message:    "",
		Name:       "",
		DetailMsg:  "",
		IsDone:     false,
	}
	oldProgress.Name = "恢复容器"
	content, err := os.ReadFile(fullPath)
	if err != nil {
		logx.Errorf("Failed to read file: %s", err)
		oldProgress.Percentage = 0
		oldProgress.Message = "读取文件失败或者未找到文件。请确认文件名仅由大小写字母、数字和短横线组成"
		oldProgress.DetailMsg = err.Error()
		oldProgress.IsDone = true
		oldProgress.IsError = true
		ctx.UpdateProgress(taskID, oldProgress)
	}
	var configList []dockerBackend.ContainerCreateConfig
	err = json.Unmarshal(content, &configList)
	if err != nil {
		logx.Errorf("Failed to parse json: %s", err)
		oldProgress.Percentage = 0
		oldProgress.Message = "解析文件失败"
		oldProgress.DetailMsg = err.Error()
		oldProgress.IsDone = true
		oldProgress.IsError = true
		ctx.UpdateProgress(taskID, oldProgress)
	}
	pr := newProgressReporter(ctx, taskID, "恢复容器")
	for i, containerInfo := range configList {
		info := "正在恢复第" + strconv.Itoa(i+1) + "个容器"
		oldProgress.Percentage = int(float64(i) / float64(len(configList)) * 100)
		oldProgress.Message = info
		oldProgress.DetailMsg = info
		ctx.UpdateProgress(taskID, oldProgress)
		ctx.DockerClient.NegotiateAPIVersion(context.TODO())
		if err != nil {
			backupList = append(backupList, "出现错误"+err.Error())
			logx.Errorf("Failed to inspect container: %s", err)
			return err
		}
		// 若配置了镜像加速器，改写成走加速器拉取。
		pullImage := ApplyRegistryMirror(containerInfo.Config.Image)
		reader, err := ctx.DockerClient.ImagePull(context.TODO(), pullImage, image.PullOptions{})
		if err != nil {
			backupList = append(backupList, containerInfo.Config.Image+"拉取镜像出现错误"+err.Error())
			logx.Errorf("Failed to pull image: %s", err)
			continue
		}
		err = decodePullResp(reader, pr)
		if err != nil {
			backupList = append(backupList, containerInfo.Config.Image+"拉取镜像出现错误"+err.Error())
			logx.Errorf("Failed to pull image: %s", err)
			continue
		}
		// 走了加速器则打回原始 tag，保证用原镜像名创建容器时能命中。
		if pullImage != containerInfo.Config.Image {
			if terr := ctx.DockerClient.ImageTag(context.TODO(), pullImage, containerInfo.Config.Image); terr != nil {
				backupList = append(backupList, containerInfo.Config.Image+"镜像重命名出现错误"+terr.Error())
				logx.Errorf("ImageTag %s -> %s 失败: %v", pullImage, containerInfo.Config.Image, terr)
				continue
			}
		}
		_, err = ctx.DockerClient.ContainerCreate(context.TODO(), containerInfo.Config, containerInfo.HostConfig, containerInfo.NetworkingConfig, nil, containerInfo.Name)
		if err != nil {
			logx.Errorf("Failed to create container: %s", err)
			info = "正在恢复第" + strconv.Itoa(i+1) + "个容器"
			backupList = append(backupList, containerInfo.Name+"恢复失败"+err.Error())
			continue
		} else {
			backupList = append(backupList, containerInfo.Name+"恢复成功")
		}
	}
	oldProgress.Percentage = 100
	oldProgress.DetailMsg = strings.Join(backupList, ",\n")
	oldProgress.Message = "恢复完成"
	oldProgress.IsDone = true
	ctx.UpdateProgress(taskID, oldProgress)
	return nil
}
