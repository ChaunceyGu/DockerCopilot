package utiles

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ChaunceyGu/DockerCopilot/internal/svc"
	"github.com/docker/docker/api/types/image"
	"github.com/zeromicro/go-zero/core/logx"
)

// RemoveImage 删除镜像。
//
// Docker 删除镜像失败的原因基本就三类，原实现直接把英文错误抛给前端，
// 用户看到 "conflict: unable to delete xxx (must be forced) - image is being
// used by stopped container yyy" 完全不知道该干什么。这里做三件事：
//  1. 镜像被多个 tag 引用时，按 ID 删会失败，改成逐个 untag（这才是 docker rmi 的行为）；
//  2. 把冲突错误翻译成中文，并把占用镜像的容器名列出来；
//  3. 明确区分「可以强制删除」和「强制也删不掉」两种情况。
func RemoveImage(ctx *svc.ServiceContext, imageID string, force bool) error {
	c, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// 先拿镜像详情，用于判断多 tag 情况
	var repoTags []string
	if inspect, _, err := ctx.DockerClient.ImageInspectWithRaw(c, imageID); err == nil {
		repoTags = inspect.RepoTags
	}

	_, err := ctx.DockerClient.ImageRemove(c, imageID, image.RemoveOptions{
		Force:         force,
		PruneChildren: true,
	})
	if err == nil {
		return nil
	}

	msg := err.Error()
	if c.Err() != nil {
		return fmt.Errorf("等待 Docker 删除镜像超时，删除结果尚未确认。请稍后刷新镜像列表确认后再重试")
	}

	// 情况一：同一个镜像 ID 被多个仓库/标签引用，按 ID 删不掉。
	// docker rmi 的做法是逐个删标签，最后一个标签删掉时镜像才真正释放。
	if strings.Contains(msg, "multiple repositories") || strings.Contains(msg, "must be forced") {
		if len(repoTags) > 1 {
			var failed []string
			for _, tag := range repoTags {
				_, terr := ctx.DockerClient.ImageRemove(c, tag, image.RemoveOptions{
					Force:         force,
					PruneChildren: true,
				})
				if terr != nil {
					logx.Errorf("remove image tag %s error: %v", tag, terr)
					failed = append(failed, tag)
				}
			}
			if len(failed) == 0 {
				return nil
			}
			if c.Err() != nil {
				return fmt.Errorf("等待 Docker 删除镜像标签超时，部分标签可能已删除。请稍后刷新镜像列表确认后再重试")
			}
			// 逐个 untag 仍有失败的，走下面的错误翻译
			msg = fmt.Sprintf("以下标签删除失败：%s", strings.Join(failed, ", "))
		}
	}

	if strings.Contains(strings.ToLower(msg), "has dependent child images") {
		children := findChildImages(ctx, imageID)
		if children != "" {
			return fmt.Errorf("该镜像被以下子镜像依赖：%s。请先确认这些镜像不再需要，再删除它们，然后重试", children)
		}
	}
	return translateRemoveError(ctx, msg, force)
}

func findChildImages(ctx *svc.ServiceContext, imageID string) string {
	c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	images, err := ctx.DockerClient.ImageList(c, image.ListOptions{All: true})
	if err != nil {
		return ""
	}
	var children []string
	for _, img := range images {
		if strings.TrimPrefix(img.ParentID, "sha256:") != strings.TrimPrefix(imageID, "sha256:") {
			continue
		}
		id := strings.TrimPrefix(img.ID, "sha256:")
		if len(id) > 12 {
			id = id[:12]
		}
		name := strings.Join(img.RepoTags, ", ")
		if name == "" || name == "<none>:<none>" {
			name = "无标签镜像"
		}
		children = append(children, fmt.Sprintf("%s（ID: %s）", name, id))
	}
	return strings.Join(children, "、")
}

// translateRemoveError 把 Docker 的删除冲突错误转成中文可操作提示。
func translateRemoveError(ctx *svc.ServiceContext, msg string, force bool) error {
	lower := strings.ToLower(msg)

	switch {
	// 被容器占用（运行中的必须先停，已停止的可以强制删）
	case strings.Contains(lower, "is being used by"):
		users := findContainersUsingImage(ctx, msg)
		hint := "该镜像正在被容器使用，无法删除"
		if users != "" {
			hint = fmt.Sprintf("该镜像正在被容器使用：%s", users)
		}
		if strings.Contains(lower, "running container") {
			return fmt.Errorf("%s。请先停止并删除这些容器，再删除镜像", hint)
		}
		if !force {
			return fmt.Errorf("%s（容器已停止）。可使用「强制删除」，或先删除这些容器", hint)
		}
		return fmt.Errorf("%s。强制删除也失败了，请先手动删除这些容器", hint)

	// 有子镜像依赖它（通常是本地 build 出来的层），强制也删不掉
	case strings.Contains(lower, "has dependent child images"):
		return fmt.Errorf("该镜像有子镜像依赖，无法删除；未能确定子镜像名称。请检查本地构建的镜像，确认不再需要后先删除子镜像，再重试")

	// 多标签引用
	case strings.Contains(lower, "multiple repositories"):
		return fmt.Errorf("该镜像被多个标签引用，需要先删除其他标签。可尝试「强制删除」")

	// 镜像已经不存在了，视作成功场景之外的提示
	case strings.Contains(lower, "no such image"):
		return fmt.Errorf("镜像不存在，可能已被删除，请刷新列表")

	default:
		return fmt.Errorf("删除镜像失败：%s", msg)
	}
}

// findContainersUsingImage 从错误信息里提取容器 ID，再换成容器名，
// 只给 ID 用户也不知道是哪个容器。
func findContainersUsingImage(ctx *svc.ServiceContext, msg string) string {
	list, err := GetContainerList(ctx)
	if err != nil {
		return ""
	}
	var names []string
	for _, c := range list {
		if len(c.ID) < 12 {
			continue
		}
		if strings.Contains(msg, c.ID[:12]) || strings.Contains(msg, c.ID) {
			if len(c.Names) > 0 {
				names = append(names, strings.TrimPrefix(c.Names[0], "/"))
			} else {
				names = append(names, c.ID[:12])
			}
		}
	}
	return strings.Join(names, ", ")
}
