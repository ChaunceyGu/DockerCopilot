# Docker Copilot 改动总结

## 本次会话完成的功能

### 1. 镜像加速器/代理支持（任务 20-21）
**问题**：国内网络拉取 Docker Hub 镜像经常超时失败。

**解决方案**：
- **后端**：在 `updatecontainer.go` 和 `restorecontainer.go` 中，拉镜像前先用加速器地址替换 `docker.io`，拉完后用 `ImageTag` 把镜像打回原始名字，避免创建容器时找不到镜像
- **设置 API**：新增 `GET/POST /api/settings` 接口（JWT 保护），支持镜像加速器地址的保存和读取，持久化到 `settings.json`
- **前端界面**：新增"设置"页面，提供 1Panel、DaoCloud、毫秒镜像、AtomHub 等常用加速器一键填入，支持自定义地址

**涉及文件**：
- `internal/utiles/settings.go` - 设置持久化逻辑
- `internal/utiles/updatecontainer.go` - 更新容器时应用加速器
- `internal/utiles/restorecontainer.go` - 恢复备份时应用加速器
- `internal/logic/settings/*` - 设置的 logic 层
- `internal/handler/settings/*` - 设置的 handler 层
- `internal/handler/routes.go` - 注册设置路由
- `internal/svc/servicecontext.go` - 加载设置到上下文
- `frontend/src/components/Settings.jsx` - 设置页面组件
- `frontend/src/App.jsx` - 路由集成
- `frontend/src/components/Header.jsx` - 侧边栏菜单项
- `frontend/src/api/client.js` - 设置 API 调用

---

### 2. 容器更新进度显示修复（任务 18-19）
**问题**：
- 前端进度轮询超时设置太短（5分钟），大镜像/慢网络下会提前放弃，进度条卡死
- 更新失败时只在 console 打日志，用户看不到错误原因
- 依赖关键词猜测成功/失败，不可靠

**解决方案**：
- **后端**：给 `TaskProgress` 加了 `IsError` 字段，在 `getprogresslogic.go` 里透出
- **前端**：
  - 重建损坏的 `useProgress.js`（之前被 fuse bug 截断）
  - 重写 `Containers.jsx` 的 `pollProgress` 函数：
    - 轮询窗口从 5 分钟拉长到 30 分钟（对齐后端拉取超时）
    - 完全依赖后端的 `isDone`/`isError` 字段判断成败
    - 失败时用弹窗展示错误原因和详细日志（`detailMsg`）
    - 成功时清理状态并刷新容器列表

**涉及文件**：
- `internal/types/taskprogress.go` - 新增 `IsError` 字段
- `internal/logic/progress/getprogresslogic.go` - 透出 `IsError`
- `frontend/src/hooks/useProgress.js` - 重建
- `frontend/src/components/Containers.jsx` - 重写 `pollProgress`
- `frontend/src/api/client.js` - 重建（之前损坏）

---

### 3. 运行时间显示修复（任务 24）
**问题**：部分容器显示"运行: 0分钟"，实际上在运行。

**原因**：后端直接把 Docker API 的原始 Status 字符串（如 `"Up About a minute"` 或 `"Up 45 seconds"`）传给前端，前端正则只能匹配带明确数字的格式，模糊表达解析失败。

**解决方案**：在后端 `containerslistlogic.go` 加了 `parseRunningTime` 函数，统一把 Status 解析成规范的中文格式：
- `"Up 5 days"` → `"5天"`
- `"Up 2 hours 30 minutes"` → `"2小时30分钟"`
- `"Up About a minute"` → `"1分钟"`
- `"Up 45 seconds"` → `"0分钟"`
- `"Exited (0) 3 hours ago"` → `"已停止"`

**涉及文件**：
- `internal/logic/container/containerslistlogic.go` - 新增 `parseRunningTime` 函数

---

### 4. 单个容器手动检查更新功能（任务 25）
**问题**：全局定时检查不够直观，用户想立即知道某个容器是否有更新。

**解决方案**：
- **后端**：新增 `POST /api/container/:id/check-update` 接口（JWT 保护），调用现有的 `checkSingleImage` 逻辑实时检查
- **前端**：
  - 在每个容器卡片的"更新"按钮旁边加了"检查"按钮（带 loading 动画）
  - 点击后调用后端接口，检查完成后弹窗显示结果："发现新版本"或"已是最新"
  - 实时更新容器列表中的 `haveUpdate` 状态

**涉及文件**：
- `internal/logic/container/checkupdatelogic.go` - 检查更新的 logic
- `internal/handler/container/checkupdatehandler.go` - 对应的 handler
- `internal/handler/routes.go` - 注册路由
- `frontend/src/api/client.js` - 新增 `checkUpdate` API
- `frontend/src/components/Containers.jsx` - 新增 `handleCheckUpdate` 和"检查"按钮

---

## 文件修复记录
以下文件在本次会话中因 fuse 写入 bug 被截断，已重建完整版本：
- `internal/utiles/updatecontainer.go`
- `internal/handler/routes.go`
- `frontend/src/App.jsx`
- `frontend/src/api/client.js`
- `frontend/src/hooks/useProgress.js`

所有文件已通过语法检查（Go: gofmt + 括号平衡；前端: babel parser），无 NUL 字节残留。

---

## 构建注意事项
1. 后端构建需要 Go 1.21+
2. 前端构建需要 Node.js 18+ 和 npm
3. 本地验证命令：
   ```bash
   # 后端
   go build ./...
   go vet ./...
   
   # 前端
   cd frontend
   npm install
   npm run build
   ```

---

## 已知问题
无

---

## 下一步建议
1. 考虑在设置页面加"测试连通性"按钮，验证加速器地址是否可用
2. 可以给"检查更新"功能加批量操作（一键检查所有容器）
3. 运行时间可以考虑加实时更新（每分钟刷新一次显示）
