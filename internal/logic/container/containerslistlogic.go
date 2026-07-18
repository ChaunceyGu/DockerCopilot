package container

import (
	"context"
	"sync"
	"time"

	"github.com/onlyLTY/dockerCopilot/internal/utiles"

	"github.com/onlyLTY/dockerCopilot/internal/svc"
	"github.com/onlyLTY/dockerCopilot/internal/types"

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
		containerInfo.RunningTime = v.Status
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
}
