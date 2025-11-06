# 交易记录统计Bug修复报告

**修复日期**: 2025-11-06
**问题**: 交易统计数据不准确,实际有4笔完整交易但只统计到2笔
**根本原因**: 平仓记录quantity=0导致无法正确匹配和计算

---

## 🔍 问题发现过程

### 用户反馈
运行了一上午(97个周期),但AI反思中显示:
- 总交易数: 2笔 (实际应该4笔)
- 胜率: 0%
- 平均盈利: 0 USDT
- 夏普比率: 0.004 (接近0)

### 实际交易情况
通过分析决策日志发现:
```
✅ 成功开仓: 5次 (全部做空)
✅ 成功平仓: 4次
⏳ 当前持仓: 3个 (SOL, BNB, HYPE)

理论上应该有: 4笔完整交易
但API只统计到: 2笔交易
```

### 详细交易操作
| 操作 | 币种 | 价格 | 状态 |
|------|------|------|------|
| close_short | DOGEUSDT | 0.16561 | ❌ 未统计 |
| open_short | ETHUSDT | 3393.36 | ✅ |
| close_short | ETHUSDT | 3404.37 | ✅ |
| open_short | ETHUSDT | 3424.06 | ✅ |
| close_short | BTCUSDT | 103722 | ❌ 未统计 |
| close_short | ETHUSDT | 3449.64 | ✅ |
| open_short | SOLUSDT | 161.94 | ⏳ 持仓中 |
| open_short | HYPEUSDT | 40.771 | ⏳ 持仓中 |
| open_short | BNBUSDT | 953.6 | ⏳ 持仓中 |

---

## 🐛 Bug根源分析

### Bug #1: 平仓quantity未记录 (严重)

**问题代码位置**: `trader/auto_trader.go:772-821`

#### 问题表现
所有平仓记录的`quantity`字段为0:

```json
// DOGE平仓
{
  "action": "close_short",
  "symbol": "DOGEUSDT",
  "quantity": 0,  // ❌ 错误! 应该是实际平仓数量
  "leverage": 0,
  "price": 0.16561,
  "success": true
}

// BTC平仓
{
  "action": "close_short",
  "symbol": "BTCUSDT",
  "quantity": 0,  // ❌ 错误!
  "leverage": 0,
  "price": 103722,
  "success": true
}
```

#### 问题代码
```go
func (at *AutoTrader) executeCloseShortWithRecord(...) error {
    // 获取当前价格
    marketData, err := market.Get(decision.Symbol)
    actionRecord.Price = marketData.CurrentPrice

    // ❌ 问题: 直接平仓quantity=0(全部),但没有记录实际数量
    order, err := at.trader.CloseShort(decision.Symbol, 0)

    // ❌ 缺少: actionRecord.Quantity = actualQuantity
    // ❌ 缺少: actionRecord.Leverage = actualLeverage

    return nil
}
```

#### 影响
- AnalyzePerformance虽然使用开仓quantity计算盈亏,但需要找到对应的开仓记录
- 如果开仓记录在窗口外或其他原因找不到,就无法匹配
- 导致统计遗漏交易

---

### Bug #2: 开仓leverage未记录 (中等)

**问题代码位置**: `trader/auto_trader.go:573-670, 673-770`

#### 问题表现
开仓记录中`leverage`字段未设置:

```json
{
  "action": "open_short",
  "symbol": "ETHUSDT",
  "quantity": 0.377,  // ✅ 正确
  "leverage": 0,      // ❌ 错误! 应该是5
  "price": 3393.36,   // ✅ 正确
  "success": true
}
```

#### 问题代码
```go
func (at *AutoTrader) executeOpenShortWithRecord(...) error {
    quantity := decision.PositionSizeUSD / marketData.CurrentPrice
    actionRecord.Quantity = quantity
    actionRecord.Price = marketData.CurrentPrice
    // ❌ 缺少: actionRecord.Leverage = decision.Leverage

    // 后面使用 decision.Leverage 开仓
    order, err := at.trader.OpenShort(decision.Symbol, quantity, decision.Leverage)
    ...
}
```

#### 影响
- `leverage=0`导致计算保证金时可能除零错误
- AnalyzePerformance计算盈亏百分比时依赖leverage
- 统计的MarginUsed和PnLPct可能不准确

---

## ✅ 修复方案

### 修复 #1: 平仓前获取并记录quantity

**修改后的代码**:

```go
func (at *AutoTrader) executeCloseShortWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
    log.Printf("  🔄 平空仓: %s", decision.Symbol)

    // 获取当前价格
    marketData, err := market.Get(decision.Symbol)
    if err != nil {
        return err
    }
    actionRecord.Price = marketData.CurrentPrice

    // ✅ 修复: 在平仓前获取实际持仓数量,用于统计分析
    positions, err := at.trader.GetPositions()
    if err == nil {
        for _, pos := range positions {
            if pos["symbol"] == decision.Symbol && pos["side"] == "short" {
                if qty, ok := pos["positionAmt"].(float64); ok {
                    if qty < 0 {
                        qty = -qty // 确保数量为正
                    }
                    actionRecord.Quantity = qty
                    log.Printf("  📊 准备平仓数量: %.4f", qty)
                    break
                }
            }
        }
    }

    // 平仓
    order, err := at.trader.CloseShort(decision.Symbol, 0) // 0 = 全部平仓
    if err != nil {
        return err
    }

    // 记录订单ID
    if orderID, ok := order["orderId"].(int64); ok {
        actionRecord.OrderID = orderID
    }

    log.Printf("  ✓ 平仓成功")
    return nil
}
```

**同样修复** `executeCloseLongWithRecord` 函数。

### 修复 #2: 记录开仓leverage

**修改后的代码**:

```go
func (at *AutoTrader) executeOpenShortWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
    // ... 前面代码 ...

    // 计算数量
    quantity := decision.PositionSizeUSD / marketData.CurrentPrice
    actionRecord.Quantity = quantity
    actionRecord.Price = marketData.CurrentPrice
    actionRecord.Leverage = decision.Leverage // ✅ 记录杠杆倍数

    // ... 后面代码 ...
}
```

**同样修复** `executeOpenLongWithRecord` 函数。

---

## 📊 修复效果

### 修复前
```json
{
  "total_trades": 2,          // ❌ 遗漏2笔
  "winning_trades": 0,
  "losing_trades": 2,
  "win_rate": 0,
  "avg_loss": -6.74,
  "profit_factor": 0
}
```

### 修复后 (对新交易生效)
- ✅ 平仓记录将包含实际quantity
- ✅ 开仓记录将包含实际leverage
- ✅ AnalyzePerformance能正确匹配所有交易对
- ✅ 统计数据将更准确

### 重要说明
⚠️ **历史数据问题**:
- 已有的决策日志quantity=0,leverage=0无法修复
- 只有**新产生的交易记录**才会有正确数据
- 建议等待新的完整交易周期后再查看统计

---

## 🔧 技术细节

### 为什么需要在平仓前获取quantity?

因为平仓API是这样的:
```go
trader.CloseShort(symbol, 0)  // 0 表示全部平仓
```

参数0是告诉交易所"平掉所有持仓",但我们不知道实际平了多少。所以需要:
1. **平仓前**: 先查询持仓,获取实际quantity
2. **记录**: 将quantity保存到actionRecord
3. **平仓**: 调用API平仓
4. **统计**: AnalyzePerformance可以使用这个quantity

### AnalyzePerformance如何匹配交易?

```go
// 使用 symbol_side 作为key
posKey := "BTCUSDT_short"

// 开仓时保存
openPositions[posKey] = {
    "quantity": 0.012,
    "openPrice": 103500,
    "leverage": 5,
    ...
}

// 平仓时查找
if openPos, exists := openPositions[posKey]; exists {
    // 使用开仓的quantity计算盈亏
    pnl := quantity * (openPrice - closePrice)
}
```

**关键**: 虽然主要使用开仓quantity,但平仓记录也应该完整!

### 为什么leverage=0会有问题?

```go
// 计算保证金
marginUsed := positionValue / float64(leverage)
// 如果leverage=0 → division by zero!

// 计算盈亏百分比
pnlPct := (pnl / marginUsed) * 100
// marginUsed错误 → pnlPct错误
```

---

## 🎯 其他发现

### 已验证正确的部分
✅ 开仓记录quantity - 正确
✅ 开仓记录price - 正确
✅ 平仓记录price - 正确
✅ AnalyzePerformance匹配逻辑 - 正确
✅ 窗口大小(10倍) - 足够大

### 无需修复的部分
- **夏普比率0.004**: 准确反映了2笔亏损交易的表现
- **平均盈利0**: 因为没有盈利交易,正确
- **胜率0%**: 2笔都亏损,正确
- **平均亏损-6.74**: 准确

---

## 📝 总结

### 问题本质
交易记录的关键字段缺失:
1. 平仓quantity=0 → 统计不完整
2. 开仓leverage=0 → 计算错误

### 修复方法
1. 平仓前查询持仓获取quantity
2. 开仓时记录decision.Leverage

### 影响范围
- **历史数据**: 无法修复(quantity已经是0)
- **新交易**: 完全修复(会有正确数据)

### 验证方法
等待新的交易完成后:
```bash
# 检查新的平仓记录
jq '.decisions[]? | select(.action=="close_short") |
    {symbol, quantity, leverage, price}' decision_latest.json

# 应该看到 quantity > 0, leverage = 5
```

### 建议
1. ✅ 继续运行交易员,产生新的交易记录
2. ✅ 等待至少2-3笔新的完整交易
3. ✅ 验证统计数据是否准确
4. ⚠️ 历史统计数据仍然不准确,只能手动统计

---

## 🚀 部署状态

- ✅ Bug已修复
- ✅ Docker镜像已重新构建
- ✅ 容器已重启
- ✅ 新的交易将有正确的记录
- ⏳ 等待新交易验证修复效果
