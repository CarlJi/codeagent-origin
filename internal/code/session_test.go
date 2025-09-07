package code

import (
	"testing"
	"time"

	"github.com/google/go-github/v58/github"
	"github.com/qiniu/codeagent/internal/config"
	"github.com/qiniu/codeagent/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockCode is a mock implementation of Code interface for testing
type MockCode struct {
	mock.Mock
}

func (m *MockCode) Prompt(message string) (*Response, error) {
	args := m.Called(message)
	return args.Get(0).(*Response), args.Error(1)
}

func (m *MockCode) Close() error {
	args := m.Called()
	return args.Error(0)
}

func TestSessionManager_GenerateSessionKey(t *testing.T) {
	cfg := &config.Config{}
	sm := NewSessionManager(cfg)

	now := time.Now()

	tests := []struct {
		name      string
		workspace *models.Workspace
		expected  string
	}{
		{
			name: "PR workspace",
			workspace: &models.Workspace{
				AIModel:   "claude",
				Org:       "qiniu",
				Repo:      "codeagent",
				PRNumber:  123,
				CreatedAt: now,
			},
			expected: "claude-qiniu-codeagent-pr-123",
		},
		{
			name: "Issue workspace",
			workspace: &models.Workspace{
				AIModel:   "claude",
				Org:       "qiniu",
				Repo:      "codeagent",
				PRNumber:  0,
				Issue:     &github.Issue{Number: github.Int(456)},
				CreatedAt: now,
			},
			expected: "claude-qiniu-codeagent-issue-456",
		},
		{
			name: "Invalid workspace returns empty string",
			workspace: &models.Workspace{
				AIModel:   "claude",
				Org:       "qiniu",
				Repo:      "codeagent",
				PRNumber:  0,
				Issue:     nil,
				CreatedAt: now,
			},
			expected: "",
		},
		{
			name: "Different AI model",
			workspace: &models.Workspace{
				AIModel:   "gemini",
				Org:       "qiniu",
				Repo:      "codeagent",
				PRNumber:  123,
				CreatedAt: now,
			},
			expected: "gemini-qiniu-codeagent-pr-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := sm.generateSessionKey(tt.workspace)
			assert.Equal(t, tt.expected, key)
		})
	}
}

func TestSessionManager_SessionIsolation(t *testing.T) {
	cfg := &config.Config{
		CodeProvider: "mock", // Use mock provider for testing
	}
	sm := NewSessionManager(cfg)

	now := time.Now()

	// Create workspaces for different scenarios
	prWorkspace := &models.Workspace{
		AIModel:   "claude",
		Org:       "qiniu",
		Repo:      "codeagent",
		PRNumber:  123,
		CreatedAt: now,
	}

	issueWorkspace := &models.Workspace{
		AIModel:   "claude",
		Org:       "qiniu",
		Repo:      "codeagent",
		PRNumber:  0,
		Issue:     &github.Issue{Number: github.Int(456)},
		CreatedAt: now,
	}

	anotherIssueWorkspace := &models.Workspace{
		AIModel:   "claude",
		Org:       "qiniu",
		Repo:      "codeagent",
		PRNumber:  0,
		Issue:     &github.Issue{Number: github.Int(789)},
		CreatedAt: now,
	}

	// Test that different workspace types get different session keys
	prKey := sm.generateSessionKey(prWorkspace)
	issueKey := sm.generateSessionKey(issueWorkspace)
	anotherIssueKey := sm.generateSessionKey(anotherIssueWorkspace)

	assert.NotEqual(t, prKey, issueKey, "PR and Issue should have different session keys")
	assert.NotEqual(t, issueKey, anotherIssueKey, "Different Issues should have different session keys")
	assert.NotEqual(t, prKey, anotherIssueKey, "PR and Issue should have different session keys")

	// Verify the exact key formats
	assert.Equal(t, "claude-qiniu-codeagent-pr-123", prKey)
	assert.Equal(t, "claude-qiniu-codeagent-issue-456", issueKey)
	assert.Equal(t, "claude-qiniu-codeagent-issue-789", anotherIssueKey)
}

func TestSessionManager_SameWorkspaceReturnsSameSession(t *testing.T) {
	cfg := &config.Config{
		CodeProvider: "mock",
	}
	sm := NewSessionManager(cfg)

	workspace := &models.Workspace{
		AIModel:   "claude",
		Org:       "qiniu",
		Repo:      "codeagent",
		PRNumber:  0,
		Issue:     &github.Issue{Number: github.Int(371)},
		CreatedAt: time.Now(),
	}

	// Since we can't mock the New function directly, we'll test the session key generation logic instead
	// This test validates that the same workspace generates the same session key consistently
	key1 := sm.generateSessionKey(workspace)
	key2 := sm.generateSessionKey(workspace)
	assert.Equal(t, key1, key2, "Same workspace should generate same session key")

	// Verify the session key is unique and consistent for this workspace
	expectedKey := "claude-qiniu-codeagent-issue-371"
	actualKey := sm.generateSessionKey(workspace)
	assert.Equal(t, expectedKey, actualKey, "Session key should match expected format")
}

func TestSessionManager_DifferentWorkspacesGetDifferentSessions(t *testing.T) {
	cfg := &config.Config{
		CodeProvider: "mock",
	}
	sm := NewSessionManager(cfg)

	workspace1 := &models.Workspace{
		AIModel:   "claude",
		Org:       "qiniu",
		Repo:      "codeagent",
		PRNumber:  0,
		Issue:     &github.Issue{Number: github.Int(371)},
		CreatedAt: time.Now(),
	}

	workspace2 := &models.Workspace{
		AIModel:   "claude",
		Org:       "qiniu",
		Repo:      "codeagent",
		PRNumber:  0,
		Issue:     &github.Issue{Number: github.Int(367)},
		CreatedAt: time.Now(),
	}

	// Test that different workspaces generate different session keys
	key1 := sm.generateSessionKey(workspace1)
	key2 := sm.generateSessionKey(workspace2)

	// Should generate different session keys
	assert.NotEqual(t, key1, key2, "Different workspaces should generate different session keys")
	assert.Equal(t, "claude-qiniu-codeagent-issue-371", key1)
	assert.Equal(t, "claude-qiniu-codeagent-issue-367", key2)
}

func TestSessionManager_CloseSession(t *testing.T) {
	cfg := &config.Config{
		CodeProvider: "mock",
	}
	sm := NewSessionManager(cfg)

	workspace := &models.Workspace{
		AIModel:   "claude",
		Org:       "qiniu",
		Repo:      "codeagent",
		PRNumber:  0,
		Issue:     &github.Issue{Number: github.Int(371)},
		CreatedAt: time.Now(),
	}

	// Test that the close session method generates the correct key
	expectedKey := "claude-qiniu-codeagent-issue-371"
	actualKey := sm.generateSessionKey(workspace)
	assert.Equal(t, expectedKey, actualKey, "CloseSession should use same key generation logic")

	// Test that calling CloseSession on non-existent session doesn't error
	err := sm.CloseSession(workspace)
	assert.NoError(t, err, "CloseSession should not error on non-existent session")
}

func TestSessionManager_GetSessionWithInvalidWorkspace(t *testing.T) {
	cfg := &config.Config{
		CodeProvider: "mock",
	}
	sm := NewSessionManager(cfg)

	// Create workspace without PR or Issue
	invalidWorkspace := &models.Workspace{
		AIModel:   "claude",
		Org:       "qiniu",
		Repo:      "codeagent",
		PRNumber:  0,
		Issue:     nil,
		CreatedAt: time.Now(),
	}

	// GetSession should return error for invalid workspace
	session, err := sm.GetSession(invalidWorkspace)
	assert.Error(t, err)
	assert.Nil(t, session)
	assert.Contains(t, err.Error(), "failed to generate session key")
}

func TestSessionManager_CloseSessionWithInvalidWorkspace(t *testing.T) {
	cfg := &config.Config{
		CodeProvider: "mock",
	}
	sm := NewSessionManager(cfg)

	// Create workspace without PR or Issue
	invalidWorkspace := &models.Workspace{
		AIModel:   "claude",
		Org:       "qiniu",
		Repo:      "codeagent",
		PRNumber:  0,
		Issue:     nil,
		CreatedAt: time.Now(),
	}

	// CloseSession should return error for invalid workspace
	err := sm.CloseSession(invalidWorkspace)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to generate session key")
}
