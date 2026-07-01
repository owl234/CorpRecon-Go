# CorpRecon-Go

一个精简版命令行爬虫工具，专精于全自动化查询与挖掘“企业对外投资子公司”。

## 🌟 核心聚焦

- **极致专一**：剔除所有冗余功能，当前版本**仅保留对外投资子公司查询**。此举是为了应对**账号单日查询上限问题**，将宝贵的查询额度全部用于核心的子公司挖掘，运行极速轻量。
- **配置驱动**：无需修改代码，通过 `config.json` 轻松设置抓取账号与目标企业名单。
- **纯粹输出**：支持导出详尽的 CSV 关联数据报表，以及便于二次利用的纯文本公司名单（TXT）。
- **联合生态**：强烈建议将本项目导出的子公司名单与 **[ICP 查询库](https://github.com/HG-ha/ICP_Query)** 联合使用，进行更全面的资产梳理，效果更佳！
- **防风控机制**：保留核心限流器与自动化人机验证拦截机制，遇阻悬停人工排雷，确保抓取不中断。

## 🚀 三步快速启动

确保环境已安装 Go 1.20+，并在 1 分钟内跑通：

```bash
# 1. 克隆代码并进入目录
git clone https://github.com/owl234/CorpRecon-Go.git
cd CorpRecon-Go

# 2. 编译并初始化配置文件 (首次运行会自动生成 config.json)
go build -o corprecon_cli ./corprecon/main.go
./corprecon_cli

# 3. 编辑 config.json 填入真实账号和名单，再次运行即可自动抓取
./corprecon_cli
```

> **注意**: 如果抓取途中遇到风控拦截，程序会自动唤起浏览器供您手动滑块验证，验证完成后会自动恢复抓取。

## 🤝 待开发模块 (贡献指南)

欢迎提交 Pull Request，目前主要急需以下方向的建设：

1. **图数据库直连**：增加对 Neo4j 等图数据库的支持，实现投资层级实时可视化。
2. **多账号动态轮询**：增加多账号与代理池，突破单日查询上限。
3. **风控策略自适应**：优化全局限流器的随机休眠曲线，降低滑块触发率。

## 📈 Star History

[![Star History Chart](https://api.star-history.com/svg?repos=owl234/CorpRecon-Go&type=Date)](https://star-history.com/#owl234/CorpRecon-Go&Date)
