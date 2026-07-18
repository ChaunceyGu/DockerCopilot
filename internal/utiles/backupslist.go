package utiles

import (
	"github.com/ChaunceyGu/DockerCopilot/internal/svc"
	"os"
	"path/filepath"
)

func BackupList(ctx *svc.ServiceContext) ([]string, error) {
	var backupList []string
	dir := os.Getenv("BACKUP_DIR") // 从环境变量中获取备份目录
	if dir == "" {
		dir = "/data/backups" // 如果环境变量未设置，使用默认值
	}
	// 备份目录可能尚未创建（首次运行、还没备份过），先确保存在，
	// 否则 ReadDir 会返回 "no such file or directory"。
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			backupList = append(backupList, entry.Name())
		} else if !entry.IsDir() && filepath.Ext(entry.Name()) == ".yaml" {
			backupList = append(backupList, entry.Name())
		}
	}

	return backupList, nil
}
