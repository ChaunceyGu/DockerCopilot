# Docker Copilot

<a href="https://www.gnu.org/licenses/agpl-3.0.en.html">
  <img alt="License: AGPLv3" src="https://shields.io/badge/License-AGPL%20v3-blue.svg">
</a>
<img alt="Platform" src="https://img.shields.io/badge/platform-amd64%20%7C%20arm64-lightgrey">
<img alt="Docker Pulls" src="https://img.shields.io/docker/pulls/chaunceygu178/dockercopilot">

一个主打**便捷、优雅**的 Docker 容器管理工具。通过浏览器即可完成容器的启停、更新、备份与恢复，无需再敲一长串 `docker` 命令。前端界面已嵌入后端二进制，单容器即可运行，支持 `amd64` 与 `arm64` 全平台。

> 本项目基于 [onlyLTY/dockerCopilot](https://github.com/onlyLTY/dockerCopilot) 二次开发，在其基础上重构了前端界面、修复了若干问题并改进了构建流程。遵循 AGPLv3 协议开源。

---

## 目录

- [功能特性](#功能特性)
- [快速开始](#快速开始)
  - [方式一：Docker Compose（推荐）](#方式一docker-compose推荐)
  - [方式二：docker run](#方式二docker-run)
- [访问界面](#访问界面)
- [配置说明](#配置说明)
  - [环境变量](#环境变量)
  - [数据卷](#数据卷)
  - [端口](#端口)
- [升级](#升级)
- [备份与恢复](#备份与恢复)
- [常见问题](#常见问题)
- [本地开发](#本地开发)
- [许可证](#许可证)
- [致谢](#致谢)

---

## 功能特性

- **容器管理** — 启动、停止、重启、重命名容器，一目了然地查看运行状态。
- **一键更新** — 检测容器所用镜像是否有新版本，支持一键拉取并重建容器，保留原有配置。
- **指定镜像更新** — 可手动指定镜像名与 tag 进行更新。
- **镜像管理** — 删除无 TAG（dangling）镜像、清理未使用镜像，释放磁盘空间。
- **备份与恢复** — 一键备份容器的完整创建配置（JSON），随时恢复；误删容器也能找回。
- **导出为 Compose** — 将现有容器配置导出为 `docker-compose.yaml`，方便迁移与版本管理。
- **进度实时查看** — 更新、恢复等耗时操作提供实时进度反馈。
- **自定义图标** — 为容器/镜像上传专属图标，界面更清爽。
- **深色模式** — 内置浅色 / 深色 / 跟随系统三种主题。
- **全平台支持** — 官方镜像同时提供 `linux/amd64` 与 `linux/arm64`（适配树莓派、各类 ARM NAS）。

---

## 快速开始

镜像已发布在 Docker Hub：[`chaunceygu178/dockercopilot`](https://hub.docker.com/r/chaunceygu178/dockercopilot)

### 方式一：Docker Compose（推荐）

新建 `docker-compose.yml`：

```yaml
services:
  dockercopilot:
    image: chaunceygu178/dockercopilot:latest
    container_name: dockercopilot
    restart: always
    privileged: true
    network_mode: bridge
    ports:
      - "12712:12712"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - ./data:/data
    environment:
      - TZ=Asia/Shanghai
      - DOCKER_HOST=unix:///var/run/docker.sock
      # secretKey 是登录密码：至少 8 位，且不能是纯数字，请务必修改
      - secretKey=请改成你自己的密码
```

启动：

```bash
docker compose up -d
```

查看日志：

```bash
docker compose logs -f
```

### 方式二：docker run

```bash
docker run -d \
  --name dockercopilot \
  --restart always \
  --privileged \
  --network bridge \
  -p 12712:12712 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v ./data:/data \
  -e TZ=Asia/Shanghai \
  -e DOCKER_HOST=unix:///var/run/docker.sock \
  -e secretKey=请改成你自己的密码 \
  chaunceygu178/dockercopilot:latest
```

---

## 访问界面

启动后，在浏览器打开：

```
http://<服务器IP>:12712/manager
```

首次进入需要输入 `secretKey`（即上面在环境变量里设置的密码）登录。

> 注意：地址结尾的 `/manager` 不能省略。

---

## 配置说明

### 环境变量

| 变量名 | 是否必填 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `secretKey` | **必填** | 空 | 登录密码 / JWT 密钥。**要求至少 8 位且不能是纯数字**，否则程序会启动失败。 |
| `DOCKER_HOST` | 否 | `unix:///var/run/docker.sock` | Docker 守护进程地址，一般无需改动。 |
| `BACKUP_DIR` | 否 | `/data/backups` | 容器备份文件的存放目录（容器内路径）。 |
| `TZ` | 否 | `Asia/Shanghai` | 时区，影响日志与备份文件的时间戳。 |
| `WORKDIR` | 否 | `/app` | 程序工作目录，用于自更新，一般无需改动。 |

### 数据卷

| 宿主机路径 | 容器内路径 | 说明 |
| --- | --- | --- |
| `/var/run/docker.sock` | `/var/run/docker.sock` | **必须挂载**。程序通过它与 Docker 通信来管理容器。 |
| `./data` | `/data` | 持久化目录。备份文件（`/data/backups`）、自定义图标与配置都存在这里，请务必挂载，否则容器重建后数据会丢失。 |

### 端口

| 容器端口 | 说明 |
| --- | --- |
| `12712` | Web 管理界面 + API。可通过 `-p 宿主端口:12712` 映射到任意宿主机端口。 |

---

## 升级

拉取最新镜像并重建容器即可，数据都在挂载的 `./data` 目录里，不会丢失：

```bash
# Compose 方式
docker compose pull
docker compose up -d

# docker run 方式
docker pull chaunceygu178/dockercopilot:latest
docker stop dockercopilot && docker rm dockercopilot
# 然后用原来的 docker run 命令重新创建
```

也可以直接在 Web 界面的「关于 / 版本」处，使用内置的一键更新功能。

如需锁定版本，把 `:latest` 换成具体的版本 tag，例如 `chaunceygu178/dockercopilot:v2.1.2`。

---

## 备份与恢复

- **备份**：在「容器」页面选择容器执行备份，配置会以 JSON 形式保存到 `/data/backups`（即宿主机的 `./data/backups`）。
- **恢复**：在「备份」页面选择备份文件即可重建容器，原有的端口、挂载、环境变量等配置都会还原。
- **导出 Compose**：可将容器配置导出为 `docker-compose.yaml`，方便迁移到别的机器或纳入 Git 管理。

> 由于备份数据存在挂载卷里，即使删除并重建 Docker Copilot 容器本身，历史备份依然保留。

---

## 常见问题

**Q：启动后容器一直重启 / 日志报「secretKey」相关错误？**
A：`secretKey` 必须至少 8 位且不能是纯数字。请检查环境变量是否设置正确。

**Q：界面打不开 / 404？**
A：确认访问地址带上了 `/manager` 后缀，且端口映射正确。

**Q：提示 `open /data/backups: no such file or directory`？**
A：请确认已挂载 `./data:/data` 数据卷。新版本已会在启动时自动创建该目录。

**Q：容器列表为空或无法管理容器？**
A：确认已挂载 `/var/run/docker.sock`，并且容器以 `privileged` 方式运行（部分系统需要）。

**Q：ARM 设备（树莓派 / ARM NAS）能用吗？**
A：可以，官方镜像已提供 `arm64` 版本，`docker pull` 会自动匹配你的架构。

---

## 本地开发

**环境要求**

- Go 1.21+（构建后端）
- Node.js 18+ / 22（构建前端）

**目录结构**

- `frontend/` — React + Vite 前端源码
- `internal/` — Go 后端（handler / logic / utiles 等）
- `docker/Dockerfile` — 多阶段构建（前端 → 后端 embed → alpine 运行镜像）
- `.github/workflows/build.yml` — CI：推 tag（`v*`）或 main 分支时自动构建并推送多架构镜像到 Docker Hub

**构建镜像**

```bash
docker build -f docker/Dockerfile -t dockercopilot:dev .
```

前端产物会在构建时嵌入（embed）进 Go 二进制，最终镜像为单一 alpine 容器。

---

## 许可证

本项目遵循 [AGPLv3](https://www.gnu.org/licenses/agpl-3.0.en.html) 协议开源，详见 [LICENSE](./LICENSE)。

---

## 致谢

本项目基于 [onlyLTY/dockerCopilot](https://github.com/onlyLTY/dockerCopilot) 二次开发，感谢原作者的开源工作。原项目同样采用 AGPLv3 许可证。
