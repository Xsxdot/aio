# AIO 单仓合并实施计划（admin 并入 aio 仓库）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 `aio/xiaozhizhang/` 的内容上移一层到 `aio/`，并将 `aio/admin/` 纳入同一个 git 仓库，使 `/Users/xushixin/workspace/aio` 同时成为 git 仓库根、Go module 根与日常开发主目录，且对外 module 路径 `github.com/xsxdot/aio` 保持不变。

**Architecture:** Go module 根必须等于仓库根——12 个下游项目 import `github.com/xsxdot/aio`，其中 3 个走公开 `go get`（v1.0.0），一旦 Go 代码落入子目录，module 路径会变成 `github.com/xsxdot/aio/xiaozhizhang`，所有下游 import 全部失效。因此唯一可行形态是「Go 上移到根 + admin 作为子目录」，而不是「aio/ 作为壳目录包住两个子项目」。关键性质：`.git` 随内容一起上移后，每个 Go 文件**相对仓库根的路径不变**（`pkg/foo.go` 仍是 `pkg/foo.go`），git 不会产生任何改名 diff，后端历史零扰动。

**Tech Stack:** git、Go 1.26.1（module `github.com/xsxdot/aio`）、Vue3 + Arco Design + Vite（admin）、SuperDev（项目配置与部署流水线）。

**前置决策（已确认）：**
- admin 源码随 `Xsxdot/aio` **公开**（后端已全公开，前端新增暴露面有限；已扫描 `src/` 无硬编码密钥）
- admin 原有 git 历史（单个 `arco-cli: initialize project` 模板 commit）**直接丢弃**，无保留价值

**注：** 本计划全部为目录搬迁与配置路径修改，不新增业务代码，故按 `instrumenting-code` 判定不含「加日志」步骤；但配置改动处必须留「为什么」注释（见 Task 6）。

---

## File Structure（目标形态）

```
/Users/xushixin/workspace/aio/          ← git 根 = Go module 根 = 开发主目录
├── go.mod                              module github.com/xsxdot/aio   ← 不变
├── go.sum  main.go  CLAUDE.md  ali.md
├── app/  base/  http/  pkg/  resources/  router/  system/  utils/
├── docs/                               后端文档 17 项 + superpowers/plans/
├── admin/                              前端（新纳入跟踪）
├── .cursorrules                        后端 DDD 规范 + 前端规范（合并后）
├── .gitignore                          新增 admin 相关忽略项
├── .claude/  .superdev/                工具链配置（新纳入跟踪）
└── .idea/  .vscode/
```

**冲突处理表**（`aio/` 与 `xiaozhizhang/` 同名项）：

| 项 | aio/ | xiaozhizhang/ | 处置 |
|---|---|---|---|
| `docs/` | 空目录（仅新建的 superpowers/） | 17 个文件 | 合并，无同名文件 |
| `.cursor/` | 空目录 | 空目录 | 任取其一 |
| `.cursorrules` | 4K 前端规范 | 28K 后端规范 | **拼接成一份** |
| `.DS_Store` | 8K | 12K | 删除并加入 gitignore |

---

## Task 1: 建立安全网（全量备份）

**Files:**
- Create: `~/aio-backup-20260820.tar.gz`

- [ ] **Step 1: 确认后端工作区干净、master 已推送**

```bash
git -C /Users/xushixin/workspace/aio/xiaozhizhang status --short
git -C /Users/xushixin/workspace/aio/xiaozhizhang rev-list --left-right --count master...origin/master
```

Expected: 第一条无输出（工作区干净）；第二条输出 `0	0`（与远端同步）。
若不同步，先 `git push origin master` 再继续。

- [ ] **Step 2: 打包备份整个 aio 目录**

```bash
tar --exclude='aio/admin/node_modules' --exclude='aio/admin/dist' \
    -czf ~/aio-backup-20260820.tar.gz \
    -C /Users/xushixin/workspace aio
```

- [ ] **Step 3: 验证备份可用**

```bash
ls -lh ~/aio-backup-20260820.tar.gz
tar -tzf ~/aio-backup-20260820.tar.gz | grep -c '^aio/'
tar -tzf ~/aio-backup-20260820.tar.gz | grep -E '^aio/(admin/src/api/executor.ts|xiaozhizhang/go.mod)$'
```

Expected: 文件存在且大于 20M；条目数 > 3000；两个关键文件路径都能列出。

**回滚方式（本计划任何阶段出错）：**
```bash
rm -rf /Users/xushixin/workspace/aio
tar -xzf ~/aio-backup-20260820.tar.gz -C /Users/xushixin/workspace
```

---

## Task 2: 目录上移（xiaozhizhang/ → aio/）

**Files:**
- Move: `aio/xiaozhizhang/*` 与 `aio/xiaozhizhang/.*` → `aio/`
- Delete: `aio/xiaozhizhang/`（空壳）

- [ ] **Step 1: 记录搬迁前的后端跟踪文件数（用作事后校验基线）**

```bash
git -C /Users/xushixin/workspace/aio/xiaozhizhang ls-files | wc -l
git -C /Users/xushixin/workspace/aio/xiaozhizhang rev-parse HEAD
```

Expected: `337`；记下 HEAD 的 commit id（应为 `506c4cb...`），Step 6 要比对。

- [ ] **Step 2: 先处理 docs 合并（aio/docs 已含本计划文件，不能被覆盖）**

```bash
cd /Users/xushixin/workspace/aio
mv xiaozhizhang/docs/* docs/
rmdir xiaozhizhang/docs
ls docs/ | wc -l
```

Expected: `docs/` 下 18 项（17 个后端文档 + `superpowers`）。

- [ ] **Step 3: 移动 .git 与所有普通文件/目录**

```bash
cd /Users/xushixin/workspace/aio
mv xiaozhizhang/.git .
mv xiaozhizhang/.gitignore .
mv xiaozhizhang/CLAUDE.md xiaozhizhang/ali.md xiaozhizhang/go.mod xiaozhizhang/go.sum xiaozhizhang/main.go .
mv xiaozhizhang/app xiaozhizhang/base xiaozhizhang/http xiaozhizhang/pkg .
mv xiaozhizhang/resources xiaozhizhang/router xiaozhizhang/system xiaozhizhang/utils .
mv xiaozhizhang/.idea xiaozhizhang/.vscode .
```

- [ ] **Step 3.5: 验证未跟踪的部署配置已随迁（关键，git 救不回来）**

`resources/dev.yaml`、`resources/prod.yaml`、`resources/test.yaml` 被 `.gitignore` 屏蔽，**不在 git 中**，只存在于本机磁盘。它们是 dev 启动与 prod 部署的真实配置（含数据库连接等），一旦在搬迁中丢失，只能从 Task 1 的备份恢复。

```bash
cd /Users/xushixin/workspace/aio
ls -l resources/dev.yaml resources/prod.yaml resources/test.yaml
```

Expected: 三个文件都存在且大小非 0。若缺失，立即从备份恢复：
```bash
tar -xzf ~/aio-backup-20260820.tar.gz -C /tmp aio/xiaozhizhang/resources
cp /tmp/aio/xiaozhizhang/resources/*.yaml /Users/xushixin/workspace/aio/resources/
```

- [ ] **Step 4: 处理剩余冲突项并删除空壳**

```bash
cd /Users/xushixin/workspace/aio
# .cursorrules 暂存到临时名，Task 3 再合并；.cursor 两边都是空目录，.DS_Store 直接丢弃
mv xiaozhizhang/.cursorrules .cursorrules.backend
rm -f xiaozhizhang/.DS_Store
rm -rf xiaozhizhang/.cursor
ls -a xiaozhizhang
```

Expected: 只剩 `.` 和 `..`。

```bash
rmdir /Users/xushixin/workspace/aio/xiaozhizhang
```

- [ ] **Step 5: 关键验证——Go 文件路径未变，git 应看不到任何删除/改名**

```bash
cd /Users/xushixin/workspace/aio
git status --short | grep -E '^( D|R )' | head
```

Expected: **无输出**。这是本计划最重要的一条验证——若出现大量 `D`（deleted），说明 `.git` 与源码的相对关系被破坏，立即执行 Task 1 的回滚。

- [ ] **Step 6: 验证仓库身份完好**

```bash
cd /Users/xushixin/workspace/aio
git rev-parse HEAD
git rev-parse --show-toplevel
git ls-files | wc -l
```

Expected: HEAD 与 Step 1 记录的 commit id 一致；toplevel 为 `/Users/xushixin/workspace/aio`；跟踪文件数仍为 `337`。

- [ ] **Step 7: 验证 Go 构建（module 根已就位）**

```bash
cd /Users/xushixin/workspace/aio && go build ./... && echo BUILD_OK
```

Expected: 输出 `BUILD_OK`，无编译错误。

- [ ] **Step 8: 提交搬迁（此时应只有 .cursorrules 一处变动）**

```bash
cd /Users/xushixin/workspace/aio
git status --short
```

Expected: 显示 ` D .cursorrules`（已被移走）加若干未跟踪项（`admin/`、`.claude/`、`.superdev/`、`.cursorrules.backend`、`docs/superpowers/`）。此步先不提交，等 Task 3 合并完 `.cursorrules` 一并提交。

---

## Task 3: 合并 .cursorrules 与加固 .gitignore

**Files:**
- Create: `aio/.cursorrules`（后端规范 + 前端规范）
- Delete: `aio/.cursorrules.backend`
- Modify: `aio/.gitignore`
- Modify: `aio/admin/.gitignore`（去重）

- [ ] **Step 1: 拼接 .cursorrules（后端在前、前端在后）**

```bash
cd /Users/xushixin/workspace/aio
{
  cat .cursorrules.backend
  printf '\n\n---\n\n'
  cat .cursorrules
} > .cursorrules.merged
mv .cursorrules.merged .cursorrules
rm .cursorrules.backend
wc -c .cursorrules
```

Expected: 约 32K（28K 后端 + 4K 前端）。

- [ ] **Step 2: 验证两部分内容都在**

```bash
cd /Users/xushixin/workspace/aio
grep -c '整体架构与目录结构' .cursorrules
grep -c '前端架构及代码规范' .cursorrules
```

Expected: 两条都输出 `1`。

- [ ] **Step 3: 加固根 .gitignore**

现有 `.gitignore` 已覆盖 `node_modules/`、`.DS_Store`、`*.local`、`.cursor/`，**不要重复添加**。真正缺失的只有下面三条——注意 `/dist/` 是根锚定规则，**不会**忽略 `admin/dist`，这是最容易漏的一条。

在 `/Users/xushixin/workspace/aio/.gitignore` 末尾追加：

```gitignore

# ---- 单仓合并后新增（2026-08-20）----
# 上方的 /dist/ 是根锚定规则，只忽略仓库根的 dist，不覆盖 admin/dist。
# 本仓库被 12 个项目当 Go 依赖 import，任何进入跟踪的文件都会被打进
# module zip 分发到每个下游的 module cache，构建产物必须挡在外面。
admin/dist/
admin/dist-ssr/

# 生产环境变量：当前为空文件，一旦填入真实域名或密钥会随公开仓库泄露，
# 因此不纳入跟踪，改由部署流水线注入。
admin/.env.production
```

- [ ] **Step 4: 确认 admin 环境变量文件的内容**

`admin/.env.production` 的「移出跟踪」在 Task 4 删除 `admin/.git` 后自然生效（届时它对新仓库只是一个未跟踪文件，且已被上面的规则忽略）。此处仅确认内容，实际的 ignore 生效验证放在 Task 4 Step 4：

```bash
cd /Users/xushixin/workspace/aio
cat admin/.env.development
wc -c admin/.env.production
```

Expected: `.env.development` 仅一行 `VITE_API_BASE_URL= 'http://localhost:9000'`；`.env.production` 为 0 字节。

- [ ] **Step 5: 去掉 admin/.gitignore 的重复行**

当前 `admin/.gitignore` 内容重复了两遍（`node_modules`、`.DS_Store`、`dist`、`dist-ssr`、`*.local` 各出现两次）。改写为：

```gitignore
node_modules
.DS_Store
dist
dist-ssr
*.local
```

---

## Task 4: 将 admin 纳入仓库并提交

**Files:**
- Delete: `aio/admin/.git/`
- Add: `aio/admin/**`、`aio/.claude/`、`aio/.superdev/`、`aio/docs/superpowers/`

- [ ] **Step 1: 最后一次确认 admin 无敏感信息**

```bash
cd /Users/xushixin/workspace/aio
grep -rEni '(secret|apikey|api_key|access_key|password|token)\s*[:=]\s*["'"'"'][A-Za-z0-9_/+-]{12,}' admin/src | head
cat admin/.env.development admin/.env.production
```

Expected: grep 无输出；`.env.development` 仅 `VITE_API_BASE_URL= 'http://localhost:9000'`；`.env.production` 为空。
**若 grep 有输出，停止执行并报告**——公开仓库不可逆。

- [ ] **Step 2: 删除 admin 的独立仓库**

```bash
cd /Users/xushixin/workspace/aio
git -C admin log --oneline | cat
rm -rf admin/.git
```

Expected: 删除前确认只有一条 `608447f arco-cli: initialize project`。若不止一条，停止并重新评估是否需要保留历史。

- [ ] **Step 3: 暂存全部新增内容**

```bash
cd /Users/xushixin/workspace/aio
git add -A
git status --short | head -20
git diff --cached --stat | tail -3
```

Expected: 新增文件集中在 `admin/`、`.claude/`、`.superdev/`、`docs/superpowers/`，`.cursorrules` 为修改。

- [ ] **Step 4: 确认没有误纳入 node_modules 或 dist**

```bash
cd /Users/xushixin/workspace/aio
git diff --cached --name-only | grep -E 'node_modules|admin/dist/|\.env\.production' | head
git diff --cached --name-only | wc -l
git check-ignore -v admin/dist admin/.env.production
```

Expected: 第一条**无输出**；第二条约 200 上下（admin 源码 129 跟踪文件 + 未跟踪业务文件 + 工具配置）；第三条应输出两行，显示两者分别命中 `.gitignore` 中新增的规则。若数字上千或 `check-ignore` 无命中，说明 gitignore 未生效，回到 Task 3。

- [ ] **Step 5: 提交**

```bash
cd /Users/xushixin/workspace/aio
git commit -m "$(cat <<'EOF'
chore: 合并 admin 前端进主仓库，目录上移至仓库根

将原 xiaozhizhang/ 的内容上移一层作为仓库根，admin 前端以子目录
形式纳入同一仓库。Go module 路径 github.com/xsxdot/aio 保持不变，
下游 12 个项目的 import 不受影响。

动机：SuperDev 部署流水线本就用 vue-go-combined-build 把前后端打进
同一个 artifact、部署为同一个 systemd 服务，.cursorrules 中前端规范
写的也是 admin/src/ 路径——只有 git 仓库还停留在分家状态。

同时将 admin/.env.production 移出跟踪，避免将来填入真实配置后
随公开仓库泄露。
EOF
)"
```

- [ ] **Step 6: 验证提交结果**

```bash
cd /Users/xushixin/workspace/aio
git log --oneline -2 | cat
git ls-files | wc -l
git ls-files admin | wc -l
```

Expected: 新 commit 在 `506c4cb` 之上；总跟踪文件数约 540；`admin/` 下约 200。

---

## Task 5: 更新 6 个下游项目的 replace 路径

**Files:**
- Modify: `/Users/xushixin/workspace/author/go/go.mod:82`
- Modify: `/Users/xushixin/workspace/one/go/go.mod:72`
- Modify: `/Users/xushixin/workspace/xianyu-new/go/go.mod:81`
- Modify: `/Users/xushixin/workspace/tk/nova-audio-go/go.mod:5`
- Modify: `/Users/xushixin/workspace/tk/server/go.mod:100`
- Modify: `/Users/xushixin/workspace/ai-hub/server/go.mod:79`

- [ ] **Step 1: 确认改前的全部命中位置**

```bash
grep -rn "aio/xiaozhizhang" /Users/xushixin/workspace --include=go.mod
```

Expected: 恰好 6 行（3 行相对路径 `../../aio/xiaozhizhang`，3 行绝对路径 `/Users/xushixin/workspace/aio/xiaozhizhang`）。

- [ ] **Step 2: 批量替换**

两种形态去掉尾部 `/xiaozhizhang` 后都恰好指向新的仓库根，因此一次替换即可覆盖：

```bash
grep -rl "aio/xiaozhizhang" /Users/xushixin/workspace --include=go.mod \
  | xargs sed -i '' 's|aio/xiaozhizhang|aio|g'
```

- [ ] **Step 3: 验证替换结果**

```bash
grep -rn "aio/xiaozhizhang" /Users/xushixin/workspace --include=go.mod
grep -rn "replace github.com/xsxdot/aio" /Users/xushixin/workspace --include=go.mod
```

Expected: 第一条**无输出**；第二条 6 行，路径分别为 `../../aio` 与 `/Users/xushixin/workspace/aio`。

- [ ] **Step 4: 抽查两个重点下游能否构建**

```bash
cd /Users/xushixin/workspace/tk/server && go build ./... && echo TK_OK
cd /Users/xushixin/workspace/ai-hub/server && go build ./... && echo AIHUB_OK
```

Expected: 输出 `TK_OK` 和 `AIHUB_OK`。若报 `module github.com/xsxdot/aio: replacement directory does not exist`，说明路径写错，回到 Step 2。

- [ ] **Step 5: 抽查一个相对路径下游**

```bash
cd /Users/xushixin/workspace/one/go && go build ./... && echo ONE_OK
```

Expected: 输出 `ONE_OK`。

---

## Task 6: 更新 SuperDev 项目配置

**Files:**
- Modify: `aio/.superdev/config.yaml`（4 处路径，行 44、49、87、91）

**纪律：** 本项目已被 SuperDev 接管，配置修改必须走 `preview_config_change` → `apply_config_change` 工具链，**不要直接编辑 yaml 文件**——直接改文件会绕过校验与审批门禁，且与 SuperDev 内部状态不一致。

- [ ] **Step 1: 读取当前配置确认待改点**

调用 `mcp__superdev__get_project_config`，确认以下 4 处仍为旧值：

| 行 | 字段 | 旧值 | 新值 |
|---|---|---|---|
| 44 | `pipelines[0].pipeline.build[0].with.vars.backend_dir` | `${workspace}/xiaozhizhang` | `${workspace}` |
| 49 | `...vars.files[1].from` | `${workspace}/xiaozhizhang/resources/prod.yaml` | `${workspace}/resources/prod.yaml` |
| 87 | `services[0].deployments[0].working_dir` | `xiaozhizhang` | `.` |
| 91 | `services[0].deployments[0].runtime.cwd` | `xiaozhizhang` | `.` |

**不需要改动**的项（合并后依然正确，误改会破坏构建）：
- `frontend_dir: ${workspace}/admin`
- `files[2].from: ${workspace}/admin/dist`
- `services[1]`（Admin: Dev Server）的 `working_dir: admin` 与 `cwd: admin`

- [ ] **Step 2: 预览变更**

调用 `mcp__superdev__preview_config_change`，提交上表 4 处修改，检查 diff 只包含这 4 行。

- [ ] **Step 3: 应用变更**

调用 `mcp__superdev__apply_config_change`。

- [ ] **Step 4: 校验流水线定义仍然合法**

调用 `mcp__superdev__validate_project_pipeline`，目标 pipeline `deploy-server-admin-prod`。

Expected: 校验通过，无路径类错误。

- [ ] **Step 5: 验证 dev 服务能正常启动（后端 working_dir 已变）**

调用 `mcp__superdev__restart_service` 启动 `server`（dev 环境），随后 `mcp__superdev__tail_logs` 查看启动日志。

Expected: 服务进入 running，日志中无 `no such file or directory` 或找不到配置文件的错误。
若启动失败，检查 `working_dir: .` 是否应改为空值——不同 SuperDev 版本对「仓库根」的表示可能不同，以 `probe_project_config` 的建议为准。

- [ ] **Step 6: 提交配置变更**

```bash
cd /Users/xushixin/workspace/aio
git add .superdev/config.yaml
git commit -m "chore(superdev): 后端路径由 xiaozhizhang 改为仓库根

目录上移后 Go 源码位于仓库根，backend_dir / working_dir / cwd
三处不再需要子目录前缀。前端 admin 路径不变。"
```

---

## Task 7: 端到端验证并推送

- [ ] **Step 1: 后端构建与测试**

```bash
cd /Users/xushixin/workspace/aio
go build ./... && echo BUILD_OK
go test ./... 2>&1 | tail -20
```

Expected: `BUILD_OK`；测试结果与合并前一致（合并未触碰任何 Go 源码，若出现新的失败必须查明原因，不得直接推送）。

- [ ] **Step 2: 前端构建**

```bash
cd /Users/xushixin/workspace/aio/admin
pnpm install --frozen-lockfile && pnpm build
ls dist | head
```

Expected: 构建成功，`dist/` 产出 `index.html` 等文件。

- [ ] **Step 3: 确认 dist 未被 git 纳入**

```bash
cd /Users/xushixin/workspace/aio
git status --short | grep -E 'admin/dist|node_modules' | head
```

Expected: **无输出**。

- [ ] **Step 4: 确认对外 module 契约未变**

```bash
cd /Users/xushixin/workspace/aio
head -1 go.mod
git ls-files | grep -c '^go.mod$'
```

Expected: `module github.com/xsxdot/aio`；`go.mod` 位于跟踪文件根层（输出 `1`）。

- [ ] **Step 5: 推送**

```bash
cd /Users/xushixin/workspace/aio
git log --oneline origin/master..master | cat
git push origin master
```

Expected: 推送成功。**注意：此步起 admin 源码即公开可见，不可逆。**

- [ ] **Step 6: 验证远端**

```bash
cd /Users/xushixin/workspace/aio
git fetch origin && git rev-list --left-right --count master...origin/master
```

Expected: `0	0`。

- [ ] **Step 7: 清理备份（确认一切正常后再执行）**

建议保留备份至少一周，确认 dev/prod 部署均正常后再删除：

```bash
rm ~/aio-backup-20260820.tar.gz
```

---

## 遗留事项（不在本计划范围）

1. **失效的 replace 路径**：`shanzilai/temp-factory/go.mod:133` 指向已不存在的 `/Users/xushixin/workspace/aio/go`；`workflow/core` 与 `workflow/workflow` 指向 `../../../css-workspace/aio/aio`（解析到 `/Users/xushixin/css-workspace/aio/aio`，该目录存在但与本仓库无关）。这三处在合并前就已错误，与本次改动无关，需单独确认它们依赖的是哪个版本。
2. **前端 tag 规约**：合并后若要为前端单独发版，Go module 只认纯 `vX.Y.Z` 形式的 tag。建议约定前端不打 tag，或使用 `admin/vX.Y.Z` 前缀，避免前端改动导致 Go 依赖版本号无谓跳动。
3. **prod.yaml 已确认安全**：`resources/dev.yaml`、`prod.yaml`、`test.yaml` 均被 `.gitignore` 屏蔽，不在公开仓库中，只有 `*.example` 被跟踪——现状正确，无需改动。但要意识到它们**没有任何版本备份**，只存在于本机；本计划的 Task 2 Step 3.5 专门验证它们随迁，日常也建议单独备份。
4. **`.gitignore` 中的 `xiaozhizhang` 一行**（Go build artifacts 段）原本用于忽略同名编译产物，目录改名后已无意义，可在后续清理中删除——本计划不动它，避免与搬迁验证混在一起。
