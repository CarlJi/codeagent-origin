# CodeAgent - 你的智能编程助手 🤖

[![Go Report Card](https://goreportcard.com/badge/github.com/qiniu/codeagent)](https://goreportcard.com/report/github.com/qiniu/codeagent)
[![Go Version](https://img.shields.io/github/go-mod/go-version/qiniu/codeagent)](https://go.dev/)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

**CodeAgent** 是一个基于 AI 的智能编程助手，直接集成到你的 GitHub 工作流中。只需要在 Issue 或 PR 中 @ 助手账号或使用简单命令，就能获得代码分析、自动编程、代码审查等服务。

> 📝 **说明**：文档中的 `@bot-name` 是示例，实际使用时需要替换为你配置的助手账号名称

## ✨ 你能用它做什么

CodeAgent 支持两种交互模式，灵活满足不同使用场景：

### 🎯 模式一：@ 提及交互（通用模式）

使用 `@bot-name` + 自然语言，适合复杂需求和灵活表达

**在 Issue 中：**

- `@bot-name 帮我分析一下这个问题的根因` - 深度问题分析
- `@bot-name 帮我实现这个功能` - 完整功能实现并创建 PR
- `@bot-name 设计一个解决方案` - 架构设计建议

**在 PR 中：**

- `@bot-name 帮我分析一下这个实现` - 代码实现分析
- `@bot-name 优化一下性能` - 性能优化改进
- `@bot-name 添加错误处理逻辑` - 代码改进并提交 commit
- `@bot-name 重构这个函数` - 代码重构

### ⚡ 模式二：Slash Commands（快捷模式）

使用预定义命令，简洁高效完成常见任务

| 命令               | 使用场景       | 功能说明                          |
| ------------------ | -------------- | --------------------------------- |
| `/code`            | Issue 评论     | 快速分析需求，实现代码并创建 PR   |
| `/continue [指令]` | PR 评论/Review | 基于现有代码继续开发，提交 commit |
| `/review`          | PR 评论        | 重新进行完整的代码审查            |

### 🤖 自动化服务

- **自动 Code Review**：每个 PR 都会收到智能的代码审查
- **Fork 仓库支持**：支持来自 Fork 仓库的 PR 交互（仅评论模式）
- **智能过滤**：自动排除特定助手账号的 PR
- **多模型支持**：可通过参数指定不同 AI 模型（如 `-claude`, `-gemini`）

## 🚀 部署与开发

CodeAgent 是一个接受 GitHub Webhook 事件并智能响应的后台服务。

### 快速部署

**第 1 步：环境准备**

```bash
# 克隆代码
git clone https://github.com/qiniu/codeagent.git
cd codeagent

# 安装依赖
go mod download
```

**第 2 步：配置环境变量**

```bash
# 设置必需的环境变量
export GITHUB_TOKEN="你的GitHub Token"
export CLAUDE_API_KEY="你的Claude API Key"  # 或其他AI提供商的Key
export WEBHOOK_SECRET="你的Webhook密钥"
```

**第 3 步：启动服务**

```bash
# 直接运行
go run ./cmd/server --port 8888

# 检查服务状态
curl http://localhost:8888/health
```

**第 4 步：配置 GitHub Webhook**

在你的仓库设置中添加 Webhook：

- URL: `https://你的域名.com/hook`
- Content type: `application/json`
- Secret: 与 `WEBHOOK_SECRET` 相同
- 事件: 勾选 `Issue comments`, `Pull request reviews`, `Pull requests`

🎉 **完成！** 现在就可以在 Issue 或 PR 中使用 `@bot-name` 了

### Docker 部署

```bash
# 构建镜像
docker build -t codeagent .

# 运行容器
docker run -d \
  -p 8888:8888 \
  -e GITHUB_TOKEN="your-token" \
  -e CLAUDE_API_KEY="your-key" \
  -e WEBHOOK_SECRET="your-secret" \
  codeagent
```

### 本地开发与测试

```bash
# 构建
make build

# 运行测试
make test

# 测试webhook事件
curl -X POST http://localhost:8888/hook \
  -H "Content-Type: application/json" \
  -H "X-GitHub-Event: issue_comment" \
  -d @test-data/issue-comment.json
```

## 🤝 贡献

欢迎贡献代码！请阅读 [CONTRIBUTING.md](CONTRIBUTING.md) 了解详细的开发流程。

## 📄 许可证

本项目采用 [Apache License 2.0](LICENSE) 许可证。

---

**需要帮助？** 查看[文档](docs/)或[提出问题](https://github.com/qiniu/codeagent/issues/new)
