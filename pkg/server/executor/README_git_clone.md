# Git仓库克隆功能

## 功能说明

`CloneGitRepository` 方法支持将Git仓库直接克隆到指定目录中，克隆完成后，直接 `cd` 到目标目录即可进入项目根目录。

## 使用示例

```go
req := &server.GitCloneRequest{
    ServerID:  "server-123",
    RepoURL:   "https://github.com/user/repo.git", 
    TargetDir: "/tmp/git-repo",
    Branch:    "main",
    Depth:     1,
}

result, err := executor.CloneGitRepository(ctx, req)
```

克隆完成后，执行 `cd /tmp/git-repo` 即可直接进入项目根目录。

## 优化内容

- **之前**: git clone 会在目标目录下创建仓库名的子目录
  - 结构: `/tmp/git-repo/repo-name/...`
- **优化后**: 仓库内容直接克隆到目标目录
  - 结构: `/tmp/git-repo/...`

## 认证方式

1. SSH密钥认证 - 提供 `GitCredentialID`
2. 用户名密码认证 - 提供 `GitCredentialID` 和 `Username`  
3. 公开仓库 - 无需认证信息 