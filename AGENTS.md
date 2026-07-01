# Design Philosophy: Simplicity

*   **Focus**: Do one thing well.
*   **Clarity**: Code should explain itself.
*   **Restraint**: Less is more.

## Guidelines

1.  **Readability over cleverness**: Write code for humans first.
2.  **Minimal dependencies**: Rely on the standard library where possible.
3.  **Elegant error handling**: Fail gracefully and explicitly.

> "Simplicity is the ultimate sophistication."

## README.md 规范 (Documentation Standards)

遵循“极简与快速上手”原则：
1. **开门见山**: 一句话精简概括项目。
2. **核心聚焦**: 仅列最核心特性，拒绝长篇大论。
3. **三步启动**: 提供直接可复制的代码（克隆、配置、运行），确保 1 分钟内跑通。
4. **明确贡献**: 给出具体待开发的模块方向。

## 爬虫防风控架构规范 (Anti-bot Architecture Rules)

在对核心爬虫逻辑（如 `engine.go`, `client.go` 等）进行迭代修改时，必须**严格遵守**以下既定的防风控安全底线，任何改动不得破坏现有平衡：

1. **绝对速率限制 (Global Rate Limiting)**：所有对风鸟网发出的 HTTP 抓取请求，必须统一经过 `internal/fetcher/client.go` 中的全局无锁限流器 (`randomSleep()`) 控制。**严禁**任何绕过限流器发起的裸并发请求，否则会瞬间触发 IP 封禁。
2. **人工干预优先 (Human-in-the-loop fallback)**：当页面正则匹配不到 `orderNo` 等核心加密参数时，**禁止直接抛错崩溃**。必须通过循环重试（预留约 60 秒宽限期），并引导用户在 `Headless: false` 的浏览器窗口中手动消除人机验证（滑块等），实现“遇阻悬停，人工排雷，自动续爬”。
3. **冗余请求最小化**：对于具有相同 `orderNo` 通讯密钥的子维度（如投资、商标、App 等），必须复用母页面的 `orderNo`。**严禁**为每个子维度单独去拉取一次主页来获取密钥。

## Artifacts 语言规范 (Artifacts Language Rule)

*   **强制中文**: 所有生成的 Artifacts（如 implementation_plan.md, task.md, walkthrough.md 以及任何图表、表格或文档）必须使用**纯中文**生成。严禁在输出给用户的说明性文本或方案中混杂非必要的英文（专有名词、代码字段除外）。
