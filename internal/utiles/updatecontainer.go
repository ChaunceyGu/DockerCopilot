package utiles

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	dockerMsgType "github.com/docker/docker/pkg/jsonmessage"
	"github.com/onlyLTY/dockerCopilot/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
	"io"
	"time"
)

const (
	// pullTimeout：拉取镜像可能很久（大镜像 + 慢网络），给足超时。
	pullTimeout = 30 * time.Minute
	// dockerOpTimeout：stop/inspect/create/start/remove 等操作的超时，
	// 避免 Docker daemon 卡住时请求永久 hang。
	dockerOpTimeout = 60 * time.Second
)

// progressReporter 统一封装任务进度上报。
// 之前的代码到处做「读旧进度 -> 改字段 -> 写回」，一旦中途 return 就会留下过期状态，
// 而且 Name/TaskID 容易丢。这里每次都写全字段，简单且不会串味。
type progressReporter struct {
	svcCtx *svc.ServiceContext
	taskID string
	name   string
}

func newProgressReporter(svcCtx *svc.ServiceContext, taskID, name string) *progressReporter {
	return &progressReporter{svcCtx: svcCtx, taskID: taskID, name: name}
}

// set 上报进行中的进度。
func (p *progressReporter) set(percentage int, message, detailMsg string) {
	p.svcCtx.UpdateProgress(p.taskID, svc.TaskProgress{
		TaskID:     p.taskID,
		Name:       p.name,
		Percentage: percentage,
		Message:    message,
		DetailMsg:  detailMsg,
		IsDone:     false,
	})
}

// fail 上报失败并结束任务（保留当前百分比，便于前端知道卡在哪一步）。
func (p *progressReporter) fail(percentage int, message string, err error) {
	detail := message
	if err != nil {
		detail = err.Error()
	}
	p.svcCtx.UpdateProgress(p.taskID, svc.TaskProgress{
		TaskID:     p.taskID,
		Name:       p.name,
		Percentage: percentage,
		Message:    message,
		DetailMsg:  detail,
		IsDone:     true,
	})
}

// done 上报成功完成。
func (p *progressReporter) done(message string) {
	p.svcCtx.UpdateProgress(p.taskID, svc.TaskProgress{
		TaskID:     p.taskID,
		Name:       p.name,
		Percentage: 100,
		Message:    message,
		DetailMsg:  message,
		IsDone:     true,
	})
}

func UpdateContainer(serviceContext *svc.ServiceContext, id string, name string, imageNameAndTag string, delOldContainer bool, taskID string) error {
	pr := newProgressReporter(serviceContext, taskID, name)
	cli := serviceContext.DockerClient

	pr.set(0, "正在连接Docker", "正在连接Docker")

	// ---- 1. 拉取新镜像（整体进度 10% -> 30%）----
	pr.set(10, "正在拉取新镜像", "正在拉取新镜像")
	pullCtx, cancelPull := context.WithTimeout(context.Background(), pullTimeout)
	defer cancelPull()
	cli.NegotiateAPIVersion(pullCtx)
	reader, err := cli.ImagePull(pullCtx, imageNameAndTag, image.PullOptions{})
	if err != nil {
		pr.fail(10, "拉取镜像失败", err)
		logx.Errorf("Failed to pull image: %s", err)
		return err
	}
	if err = decodePullResp(reader, pr); err != nil {
		pr.fail(10, "拉取镜像失败", err)
		logx.Errorf("Failed to pull image: %s", err)
		return err
	}
	pr.set(30, "拉取镜像成功", "拉取镜像成功")

	// ---- 2. 停止旧容器（30% -> 40%）----
	pr.set(30, "正在停止容器", "正在停止容器")
	timeout := 10
	stopOptions := container.StopOptions{Signal: "SIGINT", Timeout: &timeout}
	stopCtx, cancelStop := context.WithTimeout(context.Background(), dockerOpTimeout)
	err = cli.ContainerStop(stopCtx, id, stopOptions)
	cancelStop()
	if err != nil {
		pr.fail(30, "停止容器失败", err)
		return err
	}
	pr.set(40, "容器停止成功", "容器停止成功")

	// ---- 3. 重命名旧容器为备份名（40% -> 60%）----
	pr.set(40, "正在重命名旧容器", "正在重命名旧容器")
	currentDate := time.Now().Format("2006-01-02-15-04-05")
	backupName := name + "-" + currentDate
	renameCtx, cancelRename := context.WithTimeout(context.Background(), dockerOpTimeout)
	err = cli.ContainerRename(renameCtx, id, backupName)
	cancelRename()
	if err != nil {
		pr.fail(40, "重命名旧容器失败", err)
		return err
	}
	pr.set(60, "重命名旧容器成功", "重命名旧容器成功")

	// 从这里开始，旧容器已经被改名并停止。之后任何失败都要回滚：
	// 把旧容器改回原名并重新启动，避免用户的服务凭空消失。
	rollback := func(stage string, cause error) {
		logx.Errorf("更新失败(%s)，开始回滚旧容器: %v", stage, cause)
		rbCtx, cancel := context.WithTimeout(context.Background(), dockerOpTimeout)
		defer cancel()
		if rerrName := cli.ContainerRename(rbCtx, id, name); rerrName != nil {
			logx.Errorf("回滚：旧容器改回原名失败: %v", rerrName)
		}
		if rerrStart := cli.ContainerStart(rbCtx, id, container.StartOptions{}); rerrStart != nil {
			logx.Errorf("回滚：重新启动旧容器失败: %v", rerrStart)
		}
	}

	// ---- 4. 创建新容器（60% -> 80%）----
	pr.set(60, "正在创建新容器", "正在创建新容器")
	inspectCtx, cancelInspect := context.WithTimeout(context.Background(), dockerOpTimeout)
	inspectedContainer, err := cli.ContainerInspect(inspectCtx, id)
	cancelInspect()
	if err != nil {
		logx.Error("获取容器信息失败" + err.Error())
		rollback("获取容器信息", err)
		pr.fail(60, "获取容器信息失败，已回滚旧容器", err)
		return err
	}
	inspectedContainer.Config.Hostname = ""
	inspectedContainer.Config.Image = imageNameAndTag
	inspectedContainer.Image = imageNameAndTag
	config := inspectedContainer.Config
	hostConfig := inspectedContainer.HostConfig
	networkingConfig := &network.NetworkingConfig{
		EndpointsConfig: inspectedContainer.NetworkSettings.Networks,
	}
	createCtx, cancelCreate := context.WithTimeout(context.Background(), dockerOpTimeout)
	newContainer, err := cli.ContainerCreate(createCtx, config, hostConfig, networkingConfig, nil, name)
	cancelCreate()
	if err != nil {
		rollback("创建新容器", err)
		pr.fail(60, "创建新容器失败，已回滚旧容器", err)
		return err
	}
	pr.set(80, "创建新容器成功", "创建新容器成功")

	// ---- 5. 启动新容器（80% -> 100%）----
	pr.set(80, "正在启动新容器", "正在启动新容器")
	startCtx, cancelStart := context.WithTimeout(context.Background(), dockerOpTimeout)
	err = cli.ContainerStart(startCtx, newContainer.ID, container.StartOptions{})
	cancelStart()
	if err != nil {
		// 新容器起不来：删掉刚创建的新容器，再回滚旧容器。
		rmCtx, cancelRm := context.WithTimeout(context.Background(), dockerOpTimeout)
		if rerr := cli.ContainerRemove(rmCtx, newContainer.ID, container.RemoveOptions{Force: true}); rerr != nil {
			logx.Errorf("回滚：删除启动失败的新容器出错: %v", rerr)
		}
		cancelRm()
		rollback("启动新容器", err)
		pr.fail(80, "启动新容器失败，已回滚旧容器", err)
		return err
	}

	// ---- 6. 删除旧容器（可选）----
	if delOldContainer {
		pr.set(90, "正在删除旧容器", "正在删除旧容器")
		rmCtx, cancelRm := context.WithTimeout(context.Background(), dockerOpTimeout)
		err = cli.ContainerRemove(rmCtx, id, container.RemoveOptions{})
		cancelRm()
		if err != nil {
			// 新容器已经起来了，删旧容器失败不致命：提示用户手动清理，不算更新失败。
			logx.Errorf("删除旧容器失败（新容器已启动，可手动删除 %s）: %v", backupName, err)
			pr.done("更新成功（旧容器 " + backupName + " 删除失败，请手动清理）")
			return nil
		}
	}

	pr.done("更新成功")
	return nil
}

// decodePullResp 解析 docker pull 的流式响应，并把逐层进度聚合成整体百分比。
// 之前的实现把百分比死死钉在 25%，界面看起来就像卡住了；这里按各层已完成/总量
// 估算一个 0~100% 的比例，再映射到整体进度的 10%~30% 区间，让进度条平滑推进。
func decodePullResp(reader io.Reader, pr *progressReporter) error {
	decoder := json.NewDecoder(reader)
	// 记录每一层的下载/解压进度，用于聚合出一个总体百分比。
	type layer struct{ current, total int64 }
	layers := make(map[string]*layer)

	for {
		var msg dockerMsgType.JSONMessage
		if err := decoder.Decode(&msg); err != nil {
			if err == io.EOF {
				return nil
			}
			logx.Errorf("Failed to decode pull image response: %s", err)
			return fmt.Errorf("拉取镜像失败: %w", err)
		}
		if msg.Error != nil {
			logx.Errorf("Error: %s", msg.Error)
			return fmt.Errorf("拉取镜像失败: %w", msg.Error)
		}

		// 累积每层进度。只有带 Progress 的消息才有 total，用它估算整体比例。
		if msg.ID != "" && msg.Progress != nil && msg.Progress.Total > 0 {
			l, ok := layers[msg.ID]
			if !ok {
				l = &layer{}
				layers[msg.ID] = l
			}
			l.current = msg.Progress.Current
			l.total = msg.Progress.Total
		}

		var sumCur, sumTotal int64
		for _, l := range layers {
			sumCur += l.current
			sumTotal += l.total
		}
		// 拉取阶段占整体的 10%~30%。没有可用 total 时先停在起点，避免瞎跳。
		pct := 10
		if sumTotal > 0 {
			pct = 10 + int(float64(sumCur)/float64(sumTotal)*20)
			if pct > 30 {
				pct = 30
			}
		}

		detail := msg.Status
		if msg.Progress != nil && msg.Progress.String() != "" {
			detail = fmt.Sprintf("%s %s", msg.Status, msg.Progress.String())
		}
		pr.set(pct, "正在拉取新镜像", detail)
		logx.Infof("拉取镜像进度\t %s: %s", msg.Status, msg.Progress)
	}
}
