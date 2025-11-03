# 🔐 配置模板

克隆仓库后，需要配置您的API密钥和交易所信息。

## 快速开始

### 方式1: 使用Web界面配置（推荐）

```bash
# 1. 启动系统
docker-compose up -d

# 2. 访问Web界面
打开浏览器: http://localhost:3000

# 3. 配置AI模型
- Provider: DeepSeek
- API Key: 您的DeepSeek API密钥
- Model: deepseek-chat

# 4. 配置交易所
- Exchange: Hyperliquid
- Private Key: 您的Hyperliquid私钥
- Wallet Address: 您的钱包地址
- Testnet: false (主网) 或 true (测试网)

# 5. 创建交易员
- 填写交易员信息
- 选择AI模型和交易所
- 设置杠杆倍数（建议: BTC/ETH=5x, 山寨币=5x）
- 设置扫描间隔（建议: 3分钟）

# 6. 启动交易员
点击"启动"按钮开始自动交易
```

### 方式2: 使用config.json（旧版本兼容）

创建 `config.json` 文件：

```json
{
  "traders": [
    {
      "id": "my_trader",
      "name": "My AI Trader",
      "enabled": true,
      "ai_model": "deepseek",
      "exchange": "hyperliquid",
      "hyperliquid_private_key": "YOUR_PRIVATE_KEY",
      "hyperliquid_wallet_addr": "YOUR_WALLET_ADDRESS",
      "hyperliquid_testnet": false,
      "deepseek_key": "YOUR_DEEPSEEK_API_KEY",
      "initial_balance": 100,
      "scan_interval_minutes": 3
    }
  ],
  "leverage": {
    "btc_eth_leverage": 5,
    "altcoin_leverage": 5
  },
  "use_default_coins": true,
  "default_coins": [
    "BTCUSDT",
    "ETHUSDT",
    "SOLUSDT",
    "BNBUSDT",
    "XRPUSDT",
    "DOGEUSDT",
    "ADAUSDT",
    "HYPEUSDT"
  ]
}
```

## 🔑 获取API密钥

### DeepSeek API Key
1. 访问 https://platform.deepseek.com/
2. 注册账号并登录
3. 在 API Keys 页面创建新密钥
4. 复制密钥（格式：`sk-xxxxxxxxxxxxxxxx`）

### Hyperliquid 配置

**⚠️ 警告**: 永远不要分享您的私钥！

#### 主网（真实交易）
1. 创建或导入钱包
2. 获取私钥和钱包地址
3. 配置时设置 `testnet: false`

#### 测试网（推荐先测试）
1. 访问 https://app.hyperliquid-testnet.xyz/
2. 创建测试钱包
3. 获取测试网私钥
4. 配置时设置 `testnet: true`

## 🛡️ 安全建议

1. **不要提交密钥到GitHub**
   - `config.json` 已在 `.gitignore` 中
   - 数据库文件 `*.db` 已被忽略

2. **使用环境变量**（可选）
   ```bash
   export DEEPSEEK_API_KEY="sk-xxxxx"
   export HYPERLIQUID_PRIVATE_KEY="0x..."
   ```

3. **小额测试**
   - 先在测试网测试
   - 主网从小额资金开始（建议<$100）

4. **备份密钥**
   - 安全保存私钥备份
   - 使用密码管理器

## 📚 相关文档

- [完整部署文档](./DOCKER_DEPLOY.md)
- [中文说明](./README.zh-CN.md)
- [重启方案](./重新启动方案.md)

---

**重要**: 本系统为实验性质，AI自动交易存在风险，请谨慎使用！
