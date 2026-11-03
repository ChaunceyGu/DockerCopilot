package utiles

import (
	"context"
	"fmt"
	"github.com/ChaunceyGu/DockerCopilot/internal/svc"
	MyType "github.com/ChaunceyGu/DockerCopilot/internal/types"
	ref "github.com/distribution/reference"
	"github.com/docker/docker/api/types/image"
	"github.com/zeromicro/go-zero/core/logx"
	"strings"
)

func GetImagesList(ctx *svc.ServiceContext) ([]MyType.Image, error) {
	var imagesList []MyType.Image
	dockerImages, err := ctx.DockerClient.ImageList(context.Background(), image.ListOptions{})
	if err != nil {
		// 不能用 log.Fatalf：它会 os.Exit(1)，Docker daemon 的一次瞬时抖动
		// 就会把整个管理服务杀掉。返回 error，让调用方记录日志后继续。
		logx.Errorf("Unable to fetch docker images: %s", err)
		return nil, err
	}

	for _, img := range dockerImages {
		i := MyType.Image{
			Summary:    img,
			ImageName:  "",
			ImageTag:   "",
			InUsed:     false,
			SizeFormat: "",
		}
		imagesList = append(imagesList, i)
	}
	//看不明白就不要看了，这内存反复地申请，如果你看明白了 给这改成指针吧，啥？我为啥不直接写指针，我懒癌犯了就这样，欢迎pr
	imagesList, err = checkImageInUsed(ctx, splitImageNameAndTag(calculateImageSize(imagesList)))
	if err != nil {
		return imagesList, err
	}
	return imagesList, nil
}

func splitImageNameAndTag(imagesList []MyType.Image) []MyType.Image {
	for i, imageInfo := range imagesList {
		if len(imageInfo.RepoTags) != 0 {
			named, err := ref.ParseDockerRef(imageInfo.RepoTags[0])
			if tagged, ok := named.(ref.NamedTagged); err == nil && ok {
				imagesList[i].ImageName = ref.FamiliarName(ref.TrimNamed(named))
				imagesList[i].ImageTag = tagged.Tag()
			} else {
				imagesList[i].ImageName = imageInfo.RepoTags[0]
				imagesList[i].ImageTag = "None"
			}
		} else if len(imageInfo.RepoDigests) != 0 {
			imagesList[i].ImageName = strings.Split(imageInfo.RepoDigests[0], "@")[0]
			imagesList[i].ImageTag = "None"
		} else {
			imagesList[i].ImageName = "None"
			imagesList[i].ImageTag = "None"
		}
	}
	return imagesList
}
func checkImageInUsed(svc *svc.ServiceContext, imageList []MyType.Image) ([]MyType.Image, error) {
	list, err := GetContainerList(svc)
	if err != nil {
		return imageList, err
	}
	// 这里可以用mapreduce 我懒等pr
	for _, v := range list {
		for i, imageInfo := range imageList {
			if v.ImageID == imageInfo.ID {
				imageList[i].InUsed = true
				break
			}
		}
	}
	return imageList, nil
}
func calculateImageSize(imagesList []MyType.Image) []MyType.Image {
	for i := range imagesList {
		if imagesList[i].Size >= 1024*1024*1024 {
			imagesList[i].SizeFormat = // Convert size to gigabytes
				fmt.Sprintf("%d Gb", imagesList[i].Size/1024/1024/1024)
		} else {
			imagesList[i].SizeFormat = // Convert size to megabytes
				fmt.Sprintf("%d Mb", imagesList[i].Size/1024/1024)
		}
	}
	return imagesList
}
