package github

import (
	"context"
	"fmt"
	"sync"

	"github.com/qiniu/codeagent/internal/config"
	"github.com/qiniu/codeagent/internal/github/auth"
	"github.com/qiniu/codeagent/pkg/models"

	"github.com/google/go-github/v58/github"
	"github.com/qiniu/x/log"
)

// ClientManagerInterface 管理GitHub客户端，支持根据仓库动态获取合适的客户端
type ClientManagerInterface interface {
	// GetClient 根据仓库信息获取GitHub客户端
	GetClient(ctx context.Context, repo *models.Repository) (*Client, error)

	// GetAccessTokenForOrg 获取指定组织的访问令牌
	GetAccessTokenForOrg(ctx context.Context, org string) (string, error)

	// Close 释放资源
	Close() error
}

// ClientManager 客户端管理器实现
type ClientManager struct {
	authenticator auth.Authenticator // 认证器
	config        *config.Config     // 配置
	clientCache   map[string]*Client // 客户端缓存，key为"owner"
	cacheMutex    sync.RWMutex       // 缓存读写锁
}

// NewClientManager 创建客户端管理器
func NewClientManager(cfg *config.Config) (ClientManagerInterface, error) {
	// 创建默认认证器
	builder := auth.NewAuthenticatorBuilder(cfg)
	authenticator, err := builder.BuildAuthenticator()
	if err != nil {
		return nil, fmt.Errorf("failed to create authenticator: %w", err)
	}

	// 打印当前使用的认证方式信息
	authInfo := authenticator.GetAuthInfo()
	switch authInfo.Type {
	case auth.AuthTypeApp:
		log.Infof("🔐 GitHub Client Manager initialized with GitHub App authentication")
		log.Infof("   └── App ID: %d", cfg.GitHub.App.AppID)
		if cfg.GitHub.App.PrivateKeyPath != "" {
			log.Infof("   └── Private Key: from file (%s)", cfg.GitHub.App.PrivateKeyPath)
		} else if cfg.GitHub.App.PrivateKeyEnv != "" {
			log.Infof("   └── Private Key: from environment variable (%s)", cfg.GitHub.App.PrivateKeyEnv)
		} else {
			log.Infof("   └── Private Key: from direct configuration")
		}
	case auth.AuthTypePAT:
		log.Infof("🔐 GitHub Client Manager initialized with Personal Access Token (PAT)")
		tokenPreview := cfg.GitHub.Token
		if len(tokenPreview) > 10 {
			tokenPreview = tokenPreview[:7] + "***" + tokenPreview[len(tokenPreview)-4:]
		}
		log.Infof("   └── Token: %s", tokenPreview)
	default:
		log.Infof("🔐 GitHub Client Manager initialized with unknown authentication type")
	}

	return &ClientManager{
		authenticator: authenticator,
		config:        cfg,
		clientCache:   make(map[string]*Client),
		cacheMutex:    sync.RWMutex{},
	}, nil
}

// GetClient 根据仓库信息获取GitHub客户端
func (m *ClientManager) GetClient(ctx context.Context, repo *models.Repository) (*Client, error) {
	// 仓库信息是必需的
	if repo == nil {
		return nil, fmt.Errorf("repository information is required")
	}

	// 构建缓存键：组织
	cacheKey := repo.Owner

	// 检查缓存
	m.cacheMutex.RLock()
	if cachedClient, exists := m.clientCache[cacheKey]; exists {
		m.cacheMutex.RUnlock()
		return cachedClient, nil
	}
	m.cacheMutex.RUnlock()

	// 获取写锁，双重检查
	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()

	if cachedClient, exists := m.clientCache[cacheKey]; exists {
		return cachedClient, nil
	}

	// 尝试为特定组织创建客户端
	client, err := m.createClientForRepo(ctx, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to create client for organization '%s': %w", repo.Owner, err)
	}

	// 缓存新创建的客户端
	log.Infof("✅ Created and cached GitHub client for organization: %s", repo.Owner)
	m.clientCache[cacheKey] = client
	return client, nil
}

// createClientForRepo 为特定组织创建客户端
func (m *ClientManager) createClientForRepo(ctx context.Context, repo *models.Repository) (*Client, error) {
	authInfo := m.authenticator.GetAuthInfo()

	if authInfo.Type == auth.AuthTypeApp {
		// GitHub App模式：查找组织的安装
		log.Infof("Looking for GitHub App installation for organization: %s", repo.Owner)
		installationID, err := m.findInstallationForOrg(ctx, repo.Owner)
		if err != nil {
			return nil, fmt.Errorf("failed to find installation for org %s: %w", repo.Owner, err)
		}
		log.Infof("Found GitHub App installation ID %d for organization: %s", installationID, repo.Owner)

		// 获取安装客户端
		githubClient, err := m.authenticator.GetInstallationClient(ctx, installationID)
		if err != nil {
			return nil, fmt.Errorf("failed to get installation client for %s: %w", repo.Owner, err)
		}

		log.Infof("✅ Created GitHub App installation client for organization: %s (Installation ID: %d)", repo.Owner, installationID)
		return &Client{
			client: githubClient,
		}, nil
	}

	// PAT模式：创建通用客户端
	githubClient, err := m.authenticator.GetClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create PAT client: %w", err)
	}

	return &Client{
		client: githubClient,
	}, nil
}

// GetAccessTokenForOrg 获取指定组织的访问令牌
func (m *ClientManager) GetAccessTokenForOrg(ctx context.Context, org string) (string, error) {
	if m.authenticator == nil {
		return "", fmt.Errorf("authenticator is not configured")
	}

	return m.authenticator.GetAccessTokenForOrg(ctx, org)
}

// findInstallationForOrg 查找组织对应的GitHub App安装ID
func (m *ClientManager) findInstallationForOrg(ctx context.Context, owner string) (int64, error) {
	// 获取App客户端
	appClient, err := m.authenticator.GetClient(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get app client: %w", err)
	}

	// 列出所有安装
	installations, _, err := appClient.Apps.ListInstallations(ctx, &github.ListOptions{})
	if err != nil {
		return 0, fmt.Errorf("failed to list installations: %w", err)
	}

	// 查找匹配的组织安装
	for _, installation := range installations {
		if installation.Account != nil && installation.Account.GetLogin() == owner {
			return installation.GetID(), nil
		}
	}

	return 0, fmt.Errorf("no installation found for organization: %s", owner)
}

// Close 释放资源
func (m *ClientManager) Close() error {
	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()

	// 清空缓存
	m.clientCache = make(map[string]*Client)
	return nil
}
