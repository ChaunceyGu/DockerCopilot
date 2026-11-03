package module

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/ChaunceyGu/DockerCopilot/internal/types"
	ref "github.com/distribution/reference"
	"github.com/zeromicro/go-zero/core/logx"
)

type ImageCheckList struct {
	NeedUpdate bool
}

type ImageUpdateData struct {
	mu   sync.RWMutex
	Data map[string]ImageCheckList
}

func NewImageCheck() *ImageUpdateData {
	return &ImageUpdateData{Data: map[string]ImageCheckList{}}
}

func (i *ImageUpdateData) Get(imageID string) (ImageCheckList, bool) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	v, ok := i.Data[imageID]
	return v, ok
}

// CheckUpdate is used by scheduled checks. Each image has an independent deadline.
func (i *ImageUpdateData) CheckUpdate(images []types.Image, mirror string) {
	for _, image := range images {
		ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
		_, err := i.CheckImage(ctx, image, mirror)
		cancel()
		if err != nil {
			logx.Errorf("检查镜像 %s:%s 更新失败: %v", image.ImageName, image.ImageTag, err)
		}
	}
}

func (i *ImageUpdateData) CheckImage(ctx context.Context, image types.Image, mirror string) (bool, error) {
	if len(image.RepoDigests) == 0 {
		return false, fmt.Errorf("本地镜像没有仓库摘要，无法判断是否有更新（本地构建的镜像通常没有摘要）")
	}
	named, err := ref.ParseDockerRef(image.ImageName + ":" + image.ImageTag)
	if err != nil {
		return false, fmt.Errorf("无法解析镜像名称：%w", err)
	}
	tagged, ok := named.(ref.NamedTagged)
	if !ok || image.ImageTag == "None" || image.ImageTag == "<none>" {
		return false, fmt.Errorf("镜像没有有效标签，无法检查更新")
	}
	registry := "https://" + ref.Domain(named)
	if ref.Domain(named) == "docker.io" {
		registry = "https://registry-1.docker.io"
		if strings.TrimSpace(mirror) != "" {
			registry = strings.TrimRight(strings.TrimSpace(mirror), "/")
			if !strings.Contains(registry, "://") {
				registry = "https://" + registry
			}
		}
	}
	base, err := url.Parse(registry)
	if err != nil || base.Host == "" || (base.Scheme != "https" && base.Scheme != "http") || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return false, fmt.Errorf("镜像站地址无效：%s", registry)
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/v2/" + ref.Path(named) + "/manifests/" + tagged.Tag()
	remote, err := fetchManifestDigest(ctx, base.String(), ref.Path(named))
	if err != nil {
		return false, fmt.Errorf("访问镜像仓库 %s 失败：%w。请检查网络和镜像站设置", base.Host, err)
	}
	for _, local := range image.RepoDigests {
		parts := strings.SplitN(local, "@", 2)
		if len(parts) == 2 && parts[1] == remote {
			i.set(image.ID, false)
			return false, nil
		}
	}
	i.set(image.ID, true)
	return true, nil
}

func (i *ImageUpdateData) set(id string, needUpdate bool) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.Data[id] = ImageCheckList{NeedUpdate: needUpdate}
}

var registryClient = &http.Client{Timeout: 15 * time.Second}

func fetchManifestDigest(ctx context.Context, manifestURL, repository string) (string, error) {
	request := func(token string) (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodHead, manifestURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json, application/vnd.docker.distribution.manifest.list.v2+json, application/vnd.oci.image.manifest.v1+json, application/vnd.oci.image.index.v1+json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		return registryClient.Do(req)
	}
	resp, err := request("")
	if err != nil {
		return "", err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		challenge := resp.Header.Get("WWW-Authenticate")
		resp.Body.Close()
		token, err := registryToken(ctx, challenge, repository)
		if err != nil {
			return "", err
		}
		resp, err = request(token)
		if err != nil {
			return "", err
		}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("仓库返回 %s", resp.Status)
	}
	digest := resp.Header.Get("Docker-Content-Digest")
	if digest == "" {
		return "", fmt.Errorf("仓库未返回镜像摘要，可能不支持检查更新")
	}
	return digest, nil
}

func registryToken(ctx context.Context, challenge, repository string) (string, error) {
	if !strings.HasPrefix(strings.ToLower(challenge), "bearer ") {
		return "", fmt.Errorf("仓库需要认证，当前不支持此认证方式")
	}
	values := map[string]string{}
	for _, part := range strings.Split(challenge[len("Bearer "):], ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if ok {
			values[strings.ToLower(key)] = strings.Trim(value, "\"")
		}
	}
	endpoint, err := url.Parse(values["realm"])
	if err != nil || endpoint.Host == "" || (endpoint.Scheme != "https" && endpoint.Scheme != "http") || endpoint.User != nil {
		return "", fmt.Errorf("仓库返回了无效的认证地址")
	}
	q := endpoint.Query()
	if values["service"] != "" {
		q.Set("service", values["service"])
	}
	scope := values["scope"]
	if scope == "" {
		scope = "repository:" + repository + ":pull"
	}
	q.Set("scope", scope)
	endpoint.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return "", err
	}
	resp, err := registryClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("仓库认证返回 %s", resp.Status)
	}
	var result struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("无法读取仓库认证响应：%w", err)
	}
	if result.Token == "" {
		result.Token = result.AccessToken
	}
	if result.Token == "" {
		return "", fmt.Errorf("仓库没有返回认证令牌")
	}
	return result.Token, nil
}
