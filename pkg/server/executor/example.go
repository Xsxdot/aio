package executor

import (
	"context"
	"log"
	"time"

	"github.com/xsxdot/aio/pkg/server"
	"github.com/xsxdot/aio/pkg/server/credential"
)

// GitCloneExample Git克隆使用示例
type GitCloneExample struct {
	executor          server.Executor
	credentialService credential.Service
}

// ExampleSSHKeyClone 使用SSH密钥克隆示例
func (example *GitCloneExample) ExampleSSHKeyClone() {
	ctx := context.Background()

	// 1. 创建SSH密钥认证信息
	sshPrivateKey := `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAACmFlczI1Ni1jdHIAAAAGYmNyeXB0AAAAGAAAABDQd+XnqNhWW
...你的SSH私钥内容...
-----END OPENSSH PRIVATE KEY-----`

	sshCredReq := &credential.CredentialCreateRequest{
		Name:        "GitHub SSH密钥",
		Description: "用于克隆GitHub私有仓库的SSH密钥",
		Type:        credential.CredentialTypeSSHKey,
		Content:     sshPrivateKey,
	}

	sshCred, err := example.credentialService.CreateCredential(ctx, sshCredReq)
	if err != nil {
		log.Fatalf("创建SSH密钥失败: %v", err)
	}

	// 2. 使用SSH密钥克隆私有仓库
	gitReq := &server.GitCloneRequest{
		ServerID:        "server-001",
		RepoURL:         "git@github.com:yourusername/your-private-repo.git",
		TargetDir:       "/opt/projects/your-private-repo",
		Branch:          "main",
		GitCredentialID: sshCred.ID,
		Timeout:         10 * time.Minute,
		SaveLog:         true,
	}

	result, err := example.executor.CloneGitRepository(ctx, gitReq)
	if err != nil {
		log.Fatalf("SSH密钥克隆失败: %v", err)
	}

	if result.Status == server.CommandStatusSuccess {
		log.Printf("SSH密钥克隆成功到: %s", result.TargetDir)
		log.Printf("执行时间: %v", result.Duration)
	} else {
		log.Printf("SSH密钥克隆失败: %s", result.Error)
	}
}

// ExamplePasswordClone 使用用户名密码克隆示例
func (example *GitCloneExample) ExamplePasswordClone() {
	ctx := context.Background()

	// 1. 创建密码认证信息（通常是访问令牌）
	passwordCredReq := &credential.CredentialCreateRequest{
		Name:        "GitHub访问令牌",
		Description: "用于克隆GitHub私有仓库的访问令牌",
		Type:        credential.CredentialTypePassword,
		Content:     "ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", // GitHub Personal Access Token
	}

	passwordCred, err := example.credentialService.CreateCredential(ctx, passwordCredReq)
	if err != nil {
		log.Fatalf("创建密码认证失败: %v", err)
	}

	// 2. 使用用户名密码克隆私有仓库
	gitReq := &server.GitCloneRequest{
		ServerID:        "server-001",
		RepoURL:         "https://github.com/yourusername/your-private-repo.git",
		TargetDir:       "/opt/projects/your-private-repo-pwd",
		Branch:          "main",
		GitCredentialID: passwordCred.ID,
		Username:        "yourusername", // 指定用户名
		Timeout:         10 * time.Minute,
		SaveLog:         true,
	}

	result, err := example.executor.CloneGitRepository(ctx, gitReq)
	if err != nil {
		log.Fatalf("密码认证克隆失败: %v", err)
	}

	if result.Status == server.CommandStatusSuccess {
		log.Printf("密码认证克隆成功到: %s", result.TargetDir)
		log.Printf("执行时间: %v", result.Duration)
	} else {
		log.Printf("密码认证克隆失败: %s", result.Error)
	}
}

// ExamplePublicRepoClone 克隆公开仓库示例
func (example *GitCloneExample) ExamplePublicRepoClone() {
	ctx := context.Background()

	// 克隆公开仓库（无需认证）
	gitReq := &server.GitCloneRequest{
		ServerID:  "server-001",
		RepoURL:   "https://github.com/golang/go.git",
		TargetDir: "/opt/projects/golang",
		Branch:    "master",
		Depth:     1, // 浅克隆，只克隆最新提交
		Timeout:   5 * time.Minute,
		SaveLog:   true,
	}

	result, err := example.executor.CloneGitRepository(ctx, gitReq)
	if err != nil {
		log.Fatalf("公开仓库克隆失败: %v", err)
	}

	if result.Status == server.CommandStatusSuccess {
		log.Printf("公开仓库克隆成功到: %s", result.TargetDir)
		log.Printf("执行时间: %v", result.Duration)
	} else {
		log.Printf("公开仓库克隆失败: %s", result.Error)
	}
}

// ExampleGitLabClone GitLab仓库克隆示例
func (example *GitCloneExample) ExampleGitLabClone() {
	ctx := context.Background()

	// 创建GitLab访问令牌
	gitlabCredReq := &credential.CredentialCreateRequest{
		Name:        "GitLab访问令牌",
		Description: "用于克隆GitLab私有仓库的访问令牌",
		Type:        credential.CredentialTypePassword,
		Content:     "glpat-xxxxxxxxxxxxxxxxxxxx", // GitLab Personal Access Token
	}

	gitlabCred, err := example.credentialService.CreateCredential(ctx, gitlabCredReq)
	if err != nil {
		log.Fatalf("创建GitLab认证失败: %v", err)
	}

	// 克隆GitLab私有仓库
	gitReq := &server.GitCloneRequest{
		ServerID:        "server-001",
		RepoURL:         "https://gitlab.com/yourusername/your-project.git",
		TargetDir:       "/opt/projects/gitlab-project",
		Branch:          "develop",
		GitCredentialID: gitlabCred.ID,
		Username:        "oauth2", // GitLab使用oauth2作为用户名
		Timeout:         10 * time.Minute,
		SaveLog:         true,
	}

	result, err := example.executor.CloneGitRepository(ctx, gitReq)
	if err != nil {
		log.Fatalf("GitLab克隆失败: %v", err)
	}

	if result.Status == server.CommandStatusSuccess {
		log.Printf("GitLab克隆成功到: %s", result.TargetDir)
		log.Printf("执行时间: %v", result.Duration)
	} else {
		log.Printf("GitLab克隆失败: %s", result.Error)
	}
}

// 常见Git服务商的用户名建议
/*
Git服务商认证信息建议：

1. GitHub:
   - SSH: git@github.com:user/repo.git
   - HTTPS + Token: https://github.com/user/repo.git, username=实际用户名, password=Personal Access Token

2. GitLab:
   - SSH: git@gitlab.com:user/repo.git
   - HTTPS + Token: https://gitlab.com/user/repo.git, username=oauth2, password=Personal Access Token

3. Bitbucket:
   - SSH: git@bitbucket.org:user/repo.git
   - HTTPS + Token: https://bitbucket.org/user/repo.git, username=实际用户名, password=App Password

4. Azure DevOps:
   - SSH: git@ssh.dev.azure.com:v3/org/project/repo
   - HTTPS + Token: https://dev.azure.com/org/project/_git/repo, username=任意, password=Personal Access Token

5. 私有Git服务器:
   - 根据具体配置而定，通常支持用户名密码或SSH密钥
*/
