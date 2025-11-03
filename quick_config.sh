#!/bin/bash
# 快速配置交易员脚本

echo "🔧 正在配置交易员..."

# 1. 先配置AI模型
curl -s -X PUT http://localhost:8080/api/models \
  -H "Content-Type: application/json" \
  -d '[{
    "id": "deepseek-model-1",
    "name": "DeepSeek",
    "provider": "deepseek",
    "api_key": "sk-52e08756910f45af84c063f4ba4d2aeb",
    "model_name": "deepseek-chat"
  }]' > /dev/null

echo "✓ AI模型配置完成"

# 2. 配置交易所
curl -s -X PUT http://localhost:8080/api/exchanges \
  -H "Content-Type: application/json" \
  -d '[{
    "id": "hyperliquid-main-1",
    "name": "Hyperliquid Mainnet",
    "exchange_type": "hyperliquid",
    "api_key": "0e858df9012f67f768875a6e5c4ba25b37afc6d94fb3246d79344363ccb85aa9",
    "api_secret": "0x0a6598dccfd7fd59a1b281efeee8dc2add26f075",
    "testnet": false
  }]' > /dev/null

echo "✓ 交易所配置完成"

# 3. 创建交易员
curl -s -X POST http://localhost:8080/api/traders \
  -H "Content-Type: application/json" \
  -d '{
    "id": "hyperliquid_admin_deepseek_1762062114",
    "name": "Hyperliquid Main Account",
    "enabled": false,
    "ai_model_id": "deepseek-model-1",
    "exchange_id": "hyperliquid-main-1",
    "initial_balance": 100,
    "scan_interval_minutes": 3,
    "btc_eth_leverage": 5,
    "altcoin_leverage": 5
  }' > /dev/null

echo "✓ 交易员创建完成"

# 4. 启动交易员
curl -s -X POST http://localhost:8080/api/traders/hyperliquid_admin_deepseek_1762062114/start > /dev/null

echo "✓ 交易员已启动"
echo ""
echo "🎉 配置完成！系统正在运行中..."
echo "📊 查看状态: http://localhost:3000"
