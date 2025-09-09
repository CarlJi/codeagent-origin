# CodeAgent - Your AI Programming Assistant 🤖

[![Go Report Card](https://goreportcard.com/badge/github.com/qiniu/codeagent)](https://goreportcard.com/report/github.com/qiniu/codeagent)
[![Go Version](https://img.shields.io/github/go-mod/go-version/qiniu/codeagent)](https://go.dev/)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

**CodeAgent** is an AI-powered intelligent programming assistant that integrates directly into your GitHub workflow. Simply mention the bot account in Issues or PRs, or use simple commands to get code analysis, automated programming, and code review services.

> ⚠️ **Important Note**: `@niupilot` in this documentation is an example. Replace it with your configured assistant account name in actual use.

## ✨ What You Can Do With It

CodeAgent supports two interaction modes to flexibly meet different use cases:

### 🎯 Mode 1: @ Mention Interaction (General Mode)

Use `@niupilot` + natural language, suitable for complex requirements and flexible expression

**In Issues:**

- `@niupilot Help me analyze the root cause of this issue` - Deep problem analysis
- `@niupilot Help me implement this feature` - Complete feature implementation and PR creation
- `@niupilot Design a solution` - Architecture design suggestions

**In PRs:**

- `@niupilot Help me analyze this implementation` - Code implementation analysis
- `@niupilot Optimize the performance` - Performance optimization improvements
- `@niupilot Add error handling logic` - Code improvement and commit submission
- `@niupilot Refactor this function` - Code refactoring

### ⚡ Mode 2: Slash Commands (Quick Mode)

Use predefined commands for concise and efficient completion of common tasks

| Command                   | Use Case            | Function Description                                            |
| ------------------------- | ------------------- | --------------------------------------------------------------- |
| `/code`                   | Issue comments      | Quick requirement analysis, code implementation and PR creation |
| `/continue [instruction]` | PR comments/Reviews | Continue development based on existing code, submit commits     |
| `/review`                 | PR comments         | Perform complete code review again                              |

### 🤖 Automated Services

- **Automatic Code Review**: Every PR receives intelligent code review
- **Fork Repository Support**: Support for PR interactions from fork repositories (comment mode only)
- **Smart Filtering**: Automatically exclude PRs from specific assistant accounts
- **Multi-model Support**: Specify different AI models via parameters (e.g., `-claude`, `-gemini`)

## 🚀 Deployment & Development

CodeAgent is a backend service that receives GitHub Webhook events and responds intelligently.

### Quick Deployment

**Step 1: Environment Setup**

```bash
# Clone the code
git clone https://github.com/qiniu/codeagent.git
cd codeagent

# Install dependencies
go mod download
```

**Step 2: Configure Environment Variables**

```bash
# Set required environment variables
export GITHUB_TOKEN="your-GitHub-Token"
export CLAUDE_API_KEY="your-Claude-API-Key"  # or other AI provider key
export WEBHOOK_SECRET="your-Webhook-Secret"
```

**Step 3: Start Service**

```bash
# Run directly
go run ./cmd/server --port 8888

# Check service status
curl http://localhost:8888/health
```

**Step 4: Configure GitHub Webhook**

Add a Webhook in your repository settings:

- URL: `https://your-domain.com/hook`
- Content type: `application/json`
- Secret: Same as `WEBHOOK_SECRET`
- Events: Check `Issue comments`, `Pull request reviews`, `Pull requests`

🎉 **Done!** Now you can use `@niupilot` in Issues or PRs

### Docker Deployment

```bash
# Build image
docker build -t codeagent .

# Run container
docker run -d \
  -p 8888:8888 \
  -e GITHUB_TOKEN="your-token" \
  -e CLAUDE_API_KEY="your-key" \
  -e WEBHOOK_SECRET="your-secret" \
  codeagent
```

### Local Development & Testing

```bash
# Build
make build

# Run tests
make test

# Test webhook events
curl -X POST http://localhost:8888/hook \
  -H "Content-Type: application/json" \
  -H "X-GitHub-Event: issue_comment" \
  -d @test-data/issue-comment.json
```

## 🤝 Contributing

Welcome to contribute! Please read [CONTRIBUTING.md](CONTRIBUTING.md) for detailed development process.

## 📄 License

This project is licensed under the [Apache License 2.0](LICENSE).

---

**Need help?** Check [documentation](docs/) or [open an issue](https://github.com/qiniu/codeagent/issues/new)
