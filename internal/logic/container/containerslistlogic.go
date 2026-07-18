package container

import (
	"context"
	"sync"
	"time"

	"github.com/ChaunceyGu/DockerCopilot/internal/utiles"

	"github.com/ChaunceyGu/DockerCopilot/internal/svc"
	"github.com/ChaunceyGu/DockerCopilot/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ContainersListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

type Info struct {
	Id          string `json:"id"`
	Status      string `json:"status"`
	Name        string `json:"name"`
	UsingImage  string `json:"usingImage"`
	CreateImage string `json:"createImage"`
	CreateTime  string `json:"createTime"`
	RunningTime string `json:"runningTime"`
	HaveUpdate  bool   `json:"haveUpdate"`
}

func NewContainersListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ContainersListLogic {
	return &ContainersListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ContainersListLogic) ContainersList() (resp *types.Resp, err error) {
	// 获取所有容器（包括停止的容器）
	resp = &types.Resp{}
	list, err := utiles.GetContainerList(l.svcCtx)
	if err != nil {
		resp.Code = 500
		resp.Msg = err.Error()
		resp.Data = map[string]interface{}{}
		return resp, err
	}
	resp.Msg = "success"
	list = utiles.CheckImageUpdate(l.svcCtx, list)

	// 每个容器都要 inspect 一次拿 CreateImage。之前是串行，容器一多列表就很慢。
	// 这里用带上限的 worker 池并发 inspect，结果按原顺序写回。
	containerInfoList := make([]Info, len(list))
	const maxConcurrency = 8
	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup

	for i, v := range list {
		var containerInfo Info
		containerInfo.Id = v.ID
		containerInfo.Status = v.State
		if len(v.Names) > 0 {
			containerInfo.Name = v.Names[0][1:]
		} else {
			containerInfo.Name = "get container name error"
			l.Error("get container name error" + v.ID)
		}
		if v.Image != "" {
			containerInfo.UsingImage = v.Image
		} else {
			containerInfo.UsingImage = v.ImageID
			l.Error("image dont have name" + v.ID)
		}
		t := time.Unix(v.Created, 0)
		containerInfo.CreateTime = t.Format("2006-01-02 15:04:05")
		containerInfo.RunningTime = parseRunningTime(v.Status)
		containerInfo.HaveUpdate = v.Update
		containerInfoList[i] = containerInfo

		wg.Add(1)
		sem <- struct{}{}
		go func(idx string, slot int) {
			defer wg.Done()
			defer func() { <-sem }()
			containerInspect, err := utiles.GetContainerInspect(l.svcCtx, idx)
			if err != nil {
				l.Error("get image name error" + idx)
				return
			}
			// inspect 失败或 Config 为空时保持 CreateImage 为空，避免 nil 解引用。
			if containerInspect.Config != nil {
				containerInfoList[slot].CreateImage = containerInspect.Config.Image
			}
		}(v.ID, i)
	}
	wg.Wait()

	resp.Data = containerInfoList
	return resp, nil

// parseRunningTime 把 Docker Status 字符串解析成中文格式 "X天X小时X分钟"。
// 例如:
//   "Up 5 days" -> "5天"
//   "Up 2 hours 30 minutes" -> "2小时30分钟"
//   "Up About a minute" -> "1分钟"
//   "Up 45 seconds" -> "0分钟"  (不到1分钟统一显示0分钟)
//   "Exited (0) 3 hours ago" -> "已停止"
func parseRunningTime(status string) string {
	status = strings.TrimSpace(status)
	
	// 如果不是 Up 开头，说明容器停止或其他状态
	if !strings.HasPrefix(status, "Up ") {
		return "已停止"
	}
	
	// 移除 "Up " 前缀
	s := strings.TrimPrefix(status, "Up ")
	s = strings.ToLower(s)
	
	// 处理 "About a minute" 这种模糊表达
	if strings.Contains(s, "about") && strings.Contains(s, "minute") {
		return "1分钟"
	}
	
	var days, hours, minutes int
	
	// 正则提取天数
	dayRe := regexp.MustCompile(`(\d+)\s*days?`)
	if m := dayRe.FindStringSubmatch(s); len(m) > 1 {
		days, _ = strconv.Atoi(m[1])
	}
	
	// 正则提取小时
	hourRe := regexp.MustCompile(`(\d+)\s*hours?`)
	if m := hourRe.FindStringSubmatch(s); len(m) > 1 {
		hours, _ = strconv.Atoi(m[1])
	}
	
	// 正则提取分钟
	minRe := regexp.MustCompile(`(\d+)\s*minutes?`)
	if m := minRe.FindStringSubmatch(s); len(m) > 1 {
		minutes, _ = strconv.Atoi(m[1])
	}
	
	// 秒级的不显示(少于1分钟统一记为0分钟)
	
	// 组装结果
	var parts []string
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%d天", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%d小时", hours))
	}
	if minutes > 0 {
		parts = append(parts, fmt.Sprintf("%d分钟", minutes))
	}
	
	if len(parts) == 0 {
		return "0分钟"
	}
	return strings.Join(parts, "")
}


}
