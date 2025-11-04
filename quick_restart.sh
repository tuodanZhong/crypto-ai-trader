#!/bin/bash

# 🔄 快速热更新脚本
# 适用场景: Prompt优化、验证逻辑调整、非核心代码修改

set -e  # 遇到错误立即退出

echo "🚀 开始热更新..."
echo ""

# 检查是否在正确的目录
if [ ! -f "docker-compose.yml" ]; then
    echo "❌ 错误: 请在nofx目录下运行此脚本"
    exit 1
fi

# 1. 重新构建镜像
echo "📦 步骤1/3: 重新构建镜像..."
docker-compose build nofx
echo "✅ 镜像构建完成"
echo ""

# 2. 滚动更新
echo "🔄 步骤2/3: 滚动更新容器（零停机）..."
docker-compose up -d --no-deps nofx
echo "✅ 容器更新完成"
echo ""

# 等待容器启动
echo "⏳ 等待容器启动..."
sleep 3

# 3. 验证更新
echo "✅ 步骤3/3: 验证更新状态..."
echo ""

# 检查容器状态
echo "📊 容器状态:"
docker ps | grep -E "CONTAINER|nofx" || echo "❌ 未找到nofx容器"
echo ""

# 显示最新日志
echo "📝 最新日志 (最后20行):"
docker logs nofx-trading --tail 20
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✅ 热更新完成!"
echo ""
echo "🌐 请访问Web界面确认系统正常:"
echo "   http://localhost:3000"
echo ""
echo "📊 查看完整日志:"
echo "   docker logs nofx-trading -f"
echo ""
echo "⚠️  注意: 持仓时间已重置,30-60分钟后恢复正常"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
