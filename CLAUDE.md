# AI Development Harness — Claude Code 指令

你正在使用 **AI 开发 Harness** 系统进行持续开发。本文件定义了无限运行开发循环的核心规则。

## 会话启动仪式 (每次对话必须执行)

1. `pwd` — 确认工作目录
2. 读取 `claude-progress.txt` — 了解最近的进展
3. 读取 `feature_list.json` — 了解功能完成状态
4. `git log --oneline -20` — 查看最近提交
5. 读取 `init.sh` — 了解如何启动开发服务器
6. 启动开发服务器并运行冒烟测试，确认应用处于健康状态
7. 从 `feature_list.json` 中选择优先级最高且 `passes: false` 的功能

## 核心规则

### 一次只做一个功能
- 每个会话只实现一个功能。不要尝试一次性完成所有功能
- 将功能分解为可在单个会话中完成的最小可测试单元

### 实现 → 测试 → 提交 → 记录
1. 编写代码实现功能
2. 使用浏览器自动化或 API 测试进行端到端验证
3. 如果测试通过，将 `passes` 设为 `true`
4. 写描述性的 git commit 信息
5. 追加摘要到 `claude-progress.txt`

### 保持环境干净
- 每次会话结束时，代码应处于可合并到主分支的状态
- 不应有重大 bug、未完成的功能或混乱的代码
- 不要删除或修改 `feature_list.json` 中的测试步骤——只能修改 `passes` 字段

### feature_list.json 不可变规则
- **只能修改 `passes` 字段** （从 `false` 改为 `true`）
- **禁止**删除、重命名或修改任何功能的 `description` 或 `steps`
- 功能列表是你与初始需求之间的契约

## 会话结束仪式

在每次会话结束前（上下文接近限制时或被要求停止时）：
1. 确保所有改动已提交（git commit）
2. 更新 `claude-progress.txt`，记录本次会话完成的工作
3. 确保开发服务器处于可运行状态
4. 如果当前功能未完成，不要标记为 `passes: true`

## 可用工具

你拥有完整的工具集：Bash、Read、Write、Edit、浏览器自动化（Playwright MCP）等。
使用浏览器自动化以真实用户的方式测试功能。
