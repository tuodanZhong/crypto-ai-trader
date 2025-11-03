# 📤 上传项目到GitHub指南

## ✅ 准备工作检查

- [x] 所有修改已提交
- [x] 敏感信息已添加到 `.gitignore`
- [x] 配置模板文档已创建

## 🚀 上传步骤

### 第1步: 在GitHub创建新仓库

1. **访问GitHub创建仓库页面**
   ```
   https://github.com/new
   ```

2. **填写仓库信息**
   - Repository name: `my-ai-trading-system` (或您喜欢的名字)
   - Description: `AI-powered cryptocurrency trading system with DeepSeek`
   - Visibility:
     - ✅ Private (推荐 - 保护您的交易策略)
     - ⚠️ Public (如果想开源)

3. **重要: 不要勾选以下选项**
   - [ ] Add a README file
   - [ ] Add .gitignore
   - [ ] Choose a license

   (因为我们已经有这些文件了)

4. **点击 "Create repository"**

5. **复制仓库URL**
   - 会看到类似: `https://github.com/YOUR_USERNAME/my-ai-trading-system.git`
   - 复制这个URL

### 第2步: 运行上传脚本

```bash
# 在nofx目录下执行
./upload_to_github.sh
```

脚本会提示您：
1. 输入GitHub仓库URL
2. 确认操作
3. 自动推送代码

### 第3步: 验证上传成功

1. **访问您的GitHub仓库**
   ```
   https://github.com/YOUR_USERNAME/my-ai-trading-system
   ```

2. **检查以下内容**
   - [ ] 代码文件已上传
   - [ ] README.md 可见
   - [ ] `config.json` **不**可见（已被忽略）
   - [ ] `decision_logs/` **不**可见（已被忽略）
   - [ ] `*.db` 文件**不**可见（已被忽略）

3. **确认分支**
   - 当前在 `pr-92` 分支
   - 可选: 创建 `main` 分支作为主分支

## 🔐 安全检查清单

上传前请确认：

- [ ] `config.json` 在 `.gitignore` 中
- [ ] 所有 `*.db` 文件被忽略
- [ ] `decision_logs/` 被忽略
- [ ] API密钥未硬编码在代码中
- [ ] 私钥未提交到仓库

## 📋 手动上传方式（备选）

如果脚本失败，可以手动执行：

```bash
# 1. 移除原始remote
git remote remove origin

# 2. 添加您的GitHub仓库
git remote add origin https://github.com/YOUR_USERNAME/my-ai-trading-system.git

# 3. 推送代码
git push -u origin pr-92

# 4. (可选) 创建main分支
git checkout -b main
git push -u origin main
```

## 🔄 克隆到其他机器

上传成功后，可以在其他机器克隆：

```bash
# 克隆仓库
git clone https://github.com/YOUR_USERNAME/my-ai-trading-system.git
cd my-ai-trading-system

# 配置API密钥（参考 CONFIG_TEMPLATE.md）
# 方式1: Web界面配置
docker-compose up -d
# 访问 http://localhost:3000

# 方式2: 创建 config.json
cp CONFIG_TEMPLATE.md config.json
vim config.json  # 填入您的API密钥
```

## 🛠️ 常见问题

### Q1: 推送时要求输入用户名密码？

**A:** GitHub已停止支持密码认证，需要使用Personal Access Token (PAT)：

1. 访问 https://github.com/settings/tokens
2. 点击 "Generate new token (classic)"
3. 勾选 `repo` 权限
4. 生成token并复制
5. 推送时输入token作为密码

### Q2: 如何切换到main分支？

```bash
git checkout -b main
git push -u origin main

# 在GitHub设置main为默认分支
# Settings → Branches → Default branch → 选择main
```

### Q3: 不小心提交了敏感信息怎么办？

```bash
# 1. 立即修改 .gitignore
echo "config.json" >> .gitignore

# 2. 从历史中删除敏感文件
git rm --cached config.json
git commit -m "Remove sensitive config"

# 3. 如果已推送到GitHub，强制推送
git push -f origin pr-92

# 4. 立即更换API密钥！
```

### Q4: 如何保持与上游同步（原始nofx项目）？

```bash
# 添加上游remote
git remote add upstream https://github.com/tinkle-community/nofx.git

# 拉取上游更新
git fetch upstream
git merge upstream/main

# 推送到您的仓库
git push origin pr-92
```

## 📚 下一步

- [ ] 阅读 `CONFIG_TEMPLATE.md` 配置API密钥
- [ ] 阅读 `重新启动方案.md` 了解重启策略
- [ ] 在测试网测试系统
- [ ] 小额资金开始实盘交易

## 🎉 完成！

您的AI交易系统现在已经安全地上传到GitHub！

---

**创建日期**: 2025-11-03
**最后更新**: 2025-11-03
