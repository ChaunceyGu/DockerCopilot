package utiles

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"

	ref "github.com/distribution/reference"
	"github.com/zeromicro/go-zero/core/logx"
)

// settingsPath 设置文件持久化位置（挂载在 /data 卷里，容器重建不丢）。
const settingsPath = "/data/config/settings.json"

// Settings 用户可在界面配置的运行时设置。
type Settings struct {
	// RegistryMirror 镜像加速器地址，例如 https://docker.1panel.live。
	// 为空则直连 Docker Hub。仅对 docker.io 的镜像生效，私有/自建 registry 不改写。
	RegistryMirror string `json:"registryMirror"`
}

var (
	settingsMu    sync.RWMutex
	cachedSetting *Settings
)

// LoadSettings 读取设置文件；文件不存在时返回空设置（非错误）。
func LoadSettings() Settings {
	settingsMu.RLock()
	if cachedSetting != nil {
		s := *cachedSetting
		settingsMu.RUnlock()
		return s
	}
	settingsMu.RUnlock()

	settingsMu.Lock()
	defer settingsMu.Unlock()
	if cachedSetting != nil {
		return *cachedSetting
	}
	s := Settings{}
	data, err := os.ReadFile(settingsPath)
	if err == nil {
		if err := json.Unmarshal(data, &s); err != nil {
			logx.Errorf("解析 settings.json 失败，使用默认设置: %v", err)
			s = Settings{}
		}
	}
	cachedSetting = &s
	return s
}

// SaveSettings 校验并持久化设置，同时刷新内存缓存。
func SaveSettings(s Settings) error {
	s.RegistryMirror = strings.TrimSpace(s.RegistryMirror)
	// 去掉末尾斜杠，统一格式，便于后面拼接。
	s.RegistryMirror = strings.TrimRight(s.RegistryMirror, "/")

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		return err
	}
	settingsMu.Lock()
	cachedSetting = &s
	settingsMu.Unlock()
	return nil
}

// ApplyRegistryMirror 若配置了镜像加速器，把 docker.io 的镜像引用改写成走加速器拉取。
// 返回改写后的镜像名；无需改写时原样返回。
//
// 例：redis:latest -> docker.1panel.live/library/redis:latest
//
//	用户/仓库:tag -> docker.1panel.live/用户/仓库:tag
//
// 仅改写指向 Docker Hub(docker.io) 的镜像；已经带其他 registry 前缀的不动。
func ApplyRegistryMirror(imageNameAndTag string) string {
	s := LoadSettings()
	mirror := s.RegistryMirror
	if mirror == "" {
		return imageNameAndTag
	}
	// 解析出规范化引用，拿到 registry 域名与 path。
	normalizedRef, err := ref.ParseDockerRef(imageNameAndTag)
	if err != nil {
		logx.Errorf("解析镜像名失败，跳过镜像加速器改写: %s, %v", imageNameAndTag, err)
		return imageNameAndTag
	}
	domain := ref.Domain(normalizedRef)
	// 只对 Docker Hub 生效。
	if domain != "docker.io" {
		return imageNameAndTag
	}
	// 去掉 mirror 里可能带的 scheme。
	mirrorHost := mirror
	if i := strings.Index(mirrorHost, "://"); i != -1 {
		mirrorHost = mirrorHost[i+3:]
	}
	mirrorHost = strings.TrimRight(mirrorHost, "/")

	path := ref.Path(normalizedRef) // 形如 library/redis 或 someuser/somerepo
	tag := "latest"
	if tagged, ok := normalizedRef.(ref.Tagged); ok {
		tag = tagged.Tag()
	}
	rewritten := mirrorHost + "/" + path + ":" + tag
	logx.Infof("镜像加速器改写: %s -> %s", imageNameAndTag, rewritten)
	return rewritten
}
