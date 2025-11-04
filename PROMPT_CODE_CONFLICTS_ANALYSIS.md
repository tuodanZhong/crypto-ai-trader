# AI提示词 vs 系统代码 - 完整冲突分析报告

## 📋 审查信息
- **审查时间**: 2025-11-03
- **审查范围**:
  - System Prompt (engine.go 200-491行)
  - 决策验证 (engine.go 708-820行)
  - 执行逻辑 (auto_trader.go executeOpen*)
  - 数据提供 (market/data.go Format函数)
- **方法**: 逐条对比Prompt承诺 vs 代码实现

---

# 🚨 发现的冲突汇总

| # | 冲突类型 | 严重程度 | 位置 | 状态 |
|---|---------|---------|------|------|
| 1 | 风险回报比验证失效 | 🔴 P0 | engine.go:255,764 | ⚠️ 需修复 |
| 2 | 单币仓位下限缺失 | 🟡 P2 | engine.go:260,728 | ⚠️ 表述调整 |
| 3 | 杠杆限制 | ✅ P4 | engine.go:260,727 | ✅ 一致 |
| 4 | 持仓数量限制缺失 | 🟠 P1 | engine.go:259,708 | ⚠️ 需添加验证 |
| 5 | 保证金使用率验证 | ✅ P4 | engine.go:262,auto_trader | ✅ 已实现 |
| 6 | 同币种重复开仓限制 | 🟡 P2 | auto_trader.go:571 | ⚠️ Prompt缺失 |
| 7 | 移动止损/分批止盈 | 🔴 P0 | engine.go:373-390 | ⚠️ 功能缺失 |
| 8 | 强制止损触发 | 🟡 P2 | engine.go:392-398 | ⚠️ 表述不准确 |
| 9 | 市场环境识别数据 | ✅ P4 | engine.go:329,market/data.go | ✅ 数据完整 |

---

# 详细分析

## 🔴 P0级冲突 (阻断性问题)

### 冲突 #1: 风险回报比验证完全失效

**Prompt承诺** (engine.go:255,488行):
```
1. **风险回报比**: 必须 ≥ 1:3（冒1%风险，赚3%+收益）
...
- 风险回报比1:3是底线
```

**Prompt检查清单** (engine.go:468-480行):
```markdown
**做空验证**:
2. 假设入场价 `entry = (stop_loss + take_profit) / 2`
3. 风险空间 `risk = stop_loss - entry`
4. 收益空间 `reward = entry - take_profit`
5. 验证 `reward / risk >= 3.0`

**示例**:
- ✓ 做空: SL=0.178, TP=0.166, entry=0.172 → risk=0.006(3.5%), reward=0.012(7.0%), 比例1:2
- ✗ 做空: SL=0.18, TP=0.17, entry=0.175 → risk=0.005(2.9%), reward=0.005(2.9%), 比例1:1
```

**代码实现** (engine.go:764-816行):
```go
// ⚠️ 风险回报比验证 - 暂时禁用
//
// 原因: 使用midPoint=(SL+TP)/2作为假设入场价存在数学问题
//   → riskDistance 永远等于 rewardDistance (比例恒为1:1)
//   → 任何>1:1的验证要求都会拒绝所有交易
//
// 当前策略: 只验证TP/SL方向正确，具体比例由AI自主判断
/* 验证代码已全部注释 */
```

**问题分析**:

1. **Prompt中的数学错误**:
```
示例说: SL=0.178, TP=0.166, entry=0.172 → reward=0.012 (7.0%)

实际计算:
  entry = (0.178 + 0.166) / 2 = 0.172 ✓
  risk = 0.178 - 0.172 = 0.006 ✓
  reward = 0.172 - 0.166 = 0.006 (不是0.012!)
  比例 = 0.006 / 0.006 = 1:1 (不是1:2!)
```

2. **Prompt vs 代码不一致**:
   - Prompt说: "必须≥1:3, 这是底线"
   - Prompt给: 详细的计算清单
   - 代码实际: 完全不验证比例

3. **AI行为预测**:
   - AI会认为系统严格验证1:3
   - AI可能过度谨慎,拒绝一些1:2的机会
   - 或AI可能困惑为什么1:1的决策也能通过

**修复方案**:

#### 方案A (推荐): 修正Prompt,移除错误的检查清单

```markdown
## 删除或修改以下内容 (engine.go:465-480行):

### 原文:
## 止盈止损计算检查清单（生成决策前必须验证）
...

### 修改为:
## 止盈止损设置建议

**做多建议**:
- 止盈价应高于止损价
- 建议盈利空间≥3倍风险空间 (如入场100, SL=98, 建议TP≥106)

**做空建议**:
- 止损价应高于止盈价
- 建议盈利空间≥3倍风险空间 (如入场100, SL=102, 建议TP≤94)

**注意**: 系统只验证方向正确性,具体比例由你根据市场情况判断
```

#### 方案B: 实现真实的比例验证 (需要entry_price字段)

详见之前的 CRITICAL_BUG_REPORT.md 中的方案B

---

### 冲突 #7: 移动止损和分批止盈 - 功能完全缺失

**Prompt详细描述** (engine.go:373-390行):

```markdown
## 🔄 移动止损机制

**触发条件**: 持仓盈利达到1R时自动启动

**执行规则**:
- **盈利1-2R**: 移动止损至保本点
- **盈利2-3R**: 移动止损至1R盈利位置
- **盈利>3R**: 止损跟随价格，始终保持2R利润锁定

## 💰 分批止盈规则

**分批止盈计划**（强烈建议）:
1. **50%仓位@2R**: 盈利达到2R时，平仓50%锁定利润
2. **30%仓位@3R**: 盈利达到3R时，再平仓30%
3. **20%仓位Trailing**: 剩余20%使用移动止损
```

**代码实现**: ❌ 完全不存在

```bash
# 搜索相关代码
grep -rn "移动止损\|trailing\|分批止盈\|partial.*profit" nofx/
# 结果: 只在Prompt中,代码中无任何实现
```

**为什么AI无法执行**:

1. **决策语法限制**: AI只能生成一次性决策
```json
{
  "action": "open_long",
  "stop_loss": 100,
  "take_profit": 130
}
```

2. **无法表达条件逻辑**: 没有"when 盈利>2R then 平仓50%"的语法

3. **系统无监控模块**: 没有代码持续监控盈利并自动执行

**实际行为**:
```
时刻T0: AI开仓 SL=100, TP=130
时刻T1: 价格涨到120 (盈利2R)
  - Prompt期望: 自动平仓50%
  - 实际: 什么都不发生

时刻T2: 价格回落到105
  - Prompt期望: 移动止损已保本,无损失
  - 实际: 继续持有,可能触及SL=100造成亏损
```

**修复方案**:

#### 方案A (推荐): 从Prompt删除移动止损和分批止盈

**删除** engine.go 第373-398行的全部内容

**原因**:
- 系统不支持这些功能
- AI无法执行
- 造成误导和混淆

#### 方案B: 实现Position Manager模块 (工程量大)

```go
// 新增模块: position_manager.go
type PositionManager struct {
    // 每分钟检查持仓盈利
    // 根据盈利自动调整止损
    // 执行分批平仓
}

// 需要:
// 1. 后台常驻进程
// 2. 持仓盈利监控
// 3. 自动下单逻辑
// 4. 与AI决策协调
```

**不推荐**: 工程量巨大,与"AI自主决策"理念冲突

#### 方案C: 让AI手动执行分批平仓 (低效)

每个3分钟周期,AI检查持仓:
```json
// 发现BTCUSDT盈利达到2R
{"action": "close_long", "symbol": "BTCUSDT", "close_percentage": 50}
```

**缺点**:
- 依赖AI记忆和计算
- 不"自动",需AI主动判断
- 3分钟延迟,可能错过最佳时机

---

## 🟠 P1级冲突 (功能缺失)

### 冲突 #4: 持仓数量限制 - 缺少验证

**Prompt要求** (engine.go:259行):
```
2. **最多持仓**: 3个币种（质量>数量）
```

**代码验证**: ❌ 完全没有检查

```go
// engine.go validateDecision 函数中
// 只验证了: action, 杠杆, 仓位大小, TP/SL方向
// 没有验证: 持仓数量限制
```

**实际行为**:
```
当前持仓: BTC, ETH, SOL (已3个)
AI决策: open_long ADA
验证结果: ✅ 通过 (无持仓数检查)
执行结果: ✅ 成功开仓 (现在持有4个币种)
```

**风险**:
- 违反"质量>数量"原则
- 分散注意力和风险管理
- AI可能开10个小仓位

**修复方案**:

```go
// engine.go validateDecision 函数中添加

if d.Action == "open_long" || d.Action == "open_short" {
    // ... 现有验证 ...

    // 新增: 持仓数量检查 (需要传入当前持仓信息)
    // 注意: 需要修改函数签名,传入positions参数
}
```

或者在 auto_trader.go 执行层添加:

```go
// executeOpenLongWithRecord / executeOpenShortWithRecord 中

positions, err := at.trader.GetPositions()
if err == nil {
    // 检查是否已有该币种持仓
    hasPosition := false
    for _, pos := range positions {
        if pos["symbol"] == decision.Symbol {
            hasPosition = true
            break
        }
    }

    // 如果是新币种,检查总持仓数
    if !hasPosition && len(positions) >= 3 {
        return fmt.Errorf("❌ 已持有3个币种,不能再开新仓 (质量>数量原则)")
    }
}
```

---

## 🟡 P2级冲突 (表述问题)

### 冲突 #2: 单币仓位限制 - 只有上限,无下限

**Prompt表述** (engine.go:260-261行):
```go
fmt.Sprintf("3. **单币仓位**: 山寨%.0f-%.0f U(%dx杠杆) | BTC/ETH %.0f-%.0f U(%dx杠杆)\n",
    accountEquity*0.8, accountEquity*1.5, altcoinLeverage,  // 示例: 800-1500 U
    accountEquity*5, accountEquity*10, btcEthLeverage)      // 示例: 5000-10000 U
```

**代码验证** (engine.go:728-747行):
```go
maxPositionValue := accountEquity * 1.5  // 只验证上限
if d.PositionSizeUSD > maxPositionValue+tolerance {
    return error  // 超过上限→拒绝
}
// 没有最小值验证!
```

**实际行为**:
- AI可以开仓100 U的山寨币 (远低于800 U下限)
- 代码不会拒绝
- Prompt"800-1500"暗示有下限要求

**修复方案**:

修改Prompt,去掉下限:
```go
sb.WriteString(fmt.Sprintf("3. **单币仓位**: 山寨最多%.0f U(%dx杠杆) | BTC/ETH最多%.0f U(%dx杠杆)\n",
    accountEquity*1.5, altcoinLeverage, accountEquity*10, btcEthLeverage))
```

---

### 冲突 #6: 同币种重复开仓限制 - Prompt未说明

**Prompt**: ❌ 完全没有提到

**代码实现** (auto_trader.go:571-578,667-673行):
```go
// executeOpenLongWithRecord
for _, pos := range positions {
    if pos["symbol"] == decision.Symbol && pos["side"] == "long" {
        return fmt.Errorf("❌ %s 已有多仓，拒绝开仓以防止仓位叠加超限", decision.Symbol)
    }
}
```

**AI行为**:
```
场景: 已持有BTCUSDT多仓
AI决策: 再次 open_long BTCUSDT (不知道不能重复)
执行: ❌ 拒绝 "已有多仓"
AI困惑: 为什么被拒绝? Prompt没说不能重复开仓
```

**修复方案**:

在Prompt"硬约束"部分添加:
```markdown
5. **防止叠加**: 同一币种同一方向不能重复开仓
   - 如需加仓: 先close_long再open_long (用更大的position_size)
   - 换方向: 先close_long再open_short
```

---

### 冲突 #8: 强制止损触发 - 表述误导

**Prompt表述** (engine.go:392-398行):
```markdown
**立即平仓的强制情况**（不等止损价）:
1. **趋势反转**: 做多时价格跌破EMA50 + MACD死叉
2. **极端波动**: 单根K线反向波动>5%（可能爆仓）
3. **基本面冲击**: 重大利空消息
4. **止损价触及**: 市场价格触及预设止损价
```

**实际情况**:

1. **"立即平仓"误导**: AI决策周期是3分钟,不是实时
2. **极端波动检测**: AI看不到实时价格,无法检测5%跳跃
3. **止损价触及**: 由交易所自动执行,不是AI判断

**实际行为时间线**:
```
00:00 - AI开仓 (SL=95)
00:01 - 价格突然跌到96 (极端波动5%)
       Prompt期望: 立即平仓
       实际: AI在睡觉,下次决策是00:03
00:02 - 价格跌破95
       交易所自动止损平仓
00:03 - AI醒来,发现已被止损
```

**修复方案**:

修改Prompt表述:
```markdown
## 止损机制

**自动止损** (交易所执行):
- 价格触及止损价 → 交易所立即平仓
- 无需AI判断,24小时保护

**AI主动平仓** (每3分钟周期检查):
1. **趋势反转**: 做多时价格跌破EMA50 + MACD死叉 → 生成close决策
2. **信号消失**: 开仓理由不再成立 → 主动离场
3. **接近止损**: 价格接近止损价 → 可选择提前止损

**重要**: AI每3分钟决策一次,不是实时监控。极端情况依赖交易所止损保护。
```

---

## ✅ P4级 (无冲突)

### 冲突 #3: 杠杆限制 - 完全一致 ✅

**Prompt** (engine.go:260行): 使用`altcoinLeverage`和`btcEthLeverage`变量
**代码** (engine.go:727-735行): 使用同样的变量验证
**结论**: ✅ 动态一致,无问题

---

### 冲突 #5: 保证金使用率 - 已在执行层验证 ✅

**Prompt** (engine.go:262行): "保证金总使用率 ≤ 90%"
**代码** (auto_trader.go:605-614行):
```go
marginUtilization := (futureUsedMargin / totalEquity) * 100
if marginUtilization > 90.0 {
    return fmt.Errorf("⚠️ 保证金使用率将超过90%%, 拒绝开仓")
}
```
**结论**: ✅ 已实现,只是在执行层而非验证层

**潜在改进**: 在User Prompt中告诉AI当前保证金使用率

---

### 冲突 #9: 市场环境识别数据 - 数据完整 ✅

**Prompt要求** (engine.go:329-368行):
- EMA20, EMA50
- MACD
- RSI
- 成交量
- ATR
- 资金费率
- 持仓量(OI)

**数据提供** (market/data.go:456-506行):
```go
// 3分钟序列
MidPrices, EMA20Values, MACDValues, RSI7Values, RSI14Values

// 4小时长期数据
20‑Period EMA: xxx vs. 50‑Period EMA: xxx  ✅
3‑Period ATR: xxx vs. 14‑Period ATR: xxx   ✅
Current Volume vs. Average Volume         ✅

// 合约数据
Open Interest, Funding Rate                ✅
```

**结论**: ✅ 所有Prompt要求的数据都有提供

---

# 📊 修复优先级总结

## 🔴 立即修复 (P0)

### 1. 风险回报比验证冲突
- **修复方法**: 修改Prompt,删除/修正检查清单
- **涉及文件**: engine.go (465-480行)
- **工作量**: 10分钟
- **影响**: 消除AI困惑,明确系统行为

### 2. 移动止损/分批止盈功能缺失
- **修复方法**: 从Prompt删除相关描述
- **涉及文件**: engine.go (373-398行)
- **工作量**: 5分钟
- **影响**: 避免AI期望不存在的功能

## 🟠 近期修复 (P1)

### 3. 持仓数量限制缺失
- **修复方法**: 添加3个币种上限验证
- **涉及文件**: auto_trader.go或engine.go
- **工作量**: 30分钟
- **影响**: 确保"质量>数量"原则

## 🟡 优化改进 (P2)

### 4. 单币仓位下限表述
- **修复方法**: Prompt去掉下限描述
- **涉及文件**: engine.go (260行)
- **工作量**: 2分钟

### 5. 同币种重复开仓说明
- **修复方法**: Prompt添加限制说明
- **涉及文件**: engine.go (硬约束部分)
- **工作量**: 5分钟

### 6. 强制止损表述优化
- **修复方法**: 修改Prompt,区分自动止损vs AI决策
- **涉及文件**: engine.go (392-398行)
- **工作量**: 10分钟

---

# 📝 修复建议代码

## 修复 #1: 风险回报比Prompt修正

```go
// engine.go 第465-480行
// 删除原有的"止盈止损计算检查清单"

// 替换为:
sb.WriteString("## 止盈止损设置建议\n\n")
sb.WriteString("**做多建议**:\n")
sb.WriteString("- 止盈价必须高于止损价\n")
sb.WriteString("- 建议盈利空间≥3倍风险空间\n")
sb.WriteString("- 示例: 入场100, SL=98(-2%), 建议TP≥106(+6%), 比例1:3 ✓\n\n")
sb.WriteString("**做空建议**:\n")
sb.WriteString("- 止损价必须高于止盈价\n")
sb.WriteString("- 建议盈利空间≥3倍风险空间\n")
sb.WriteString("- 示例: 入场100, SL=102(+2%), 建议TP≤94(-6%), 比例1:3 ✓\n\n")
sb.WriteString("**注意**: 系统只验证TP/SL方向正确性,具体比例由你根据市场情况自主判断。\n\n")
```

## 修复 #2: 删除移动止损和分批止盈

```go
// engine.go 删除373-398行的全部内容
// 包括:
// - ## 🔄 移动止损机制
// - ## 💰 分批止盈规则
// - ## 🚨 强制止损触发条件
```

## 修复 #3: 添加持仓数量限制验证

```go
// auto_trader.go executeOpenLongWithRecord 和 executeOpenShortWithRecord 中
// 在现有的"同币种同方向"检查后添加:

positions, err := at.trader.GetPositions()
if err == nil {
    // 现有检查: 同币种同方向
    for _, pos := range positions {
        if pos["symbol"] == decision.Symbol && pos["side"] == "long" {
            return fmt.Errorf("❌ %s 已有多仓，拒绝开仓", decision.Symbol)
        }
    }

    // 新增检查: 最多3个币种
    uniqueSymbols := make(map[string]bool)
    for _, pos := range positions {
        uniqueSymbols[pos["symbol"].(string)] = true
    }

    // 如果是新币种,检查总数
    if !uniqueSymbols[decision.Symbol] && len(uniqueSymbols) >= 3 {
        return fmt.Errorf("❌ 已持有3个币种(%v),不能再开新仓 (质量>数量原则)",
            getSymbolList(uniqueSymbols))
    }
}
```

## 修复 #5: Prompt添加同币种限制说明

```go
// engine.go 第262行后添加:

sb.WriteString("5. **防止叠加**: 同一币种同一方向不能重复开仓\n")
sb.WriteString("   - 如需加仓: 先close当前仓位,再用更大的position_size开仓\n")
sb.WriteString("   - 换方向: 先close_long再open_short (或反之)\n\n")
```

---

# 🎯 总结

## 发现的主要问题

1. **承诺与实现脱节**: Prompt描述了很多系统不支持的功能
2. **数学错误**: 风险回报比计算示例有误
3. **验证缺失**: 一些硬约束没有代码强制执行
4. **表述误导**: 部分描述让AI误解系统能力

## 建议的修复策略

**第一阶段** (立即执行):
- ✅ 修正或删除错误的Prompt内容
- ✅ 对齐Prompt承诺与代码实现
- ✅ 工作量: 1小时内完成

**第二阶段** (本周内):
- ✅ 添加缺失的验证逻辑
- ✅ 完善Prompt表述
- ✅ 工作量: 2-3小时

**第三阶段** (可选):
- 🔄 实现Position Manager (如果需要移动止损功能)
- 🔄 添加entry_price字段 (如果需要准确的比例验证)
- 🔄 工作量: 数天

## 预期效果

修复后:
- ✅ Prompt与代码完全一致
- ✅ AI不会被误导
- ✅ 决策更准确,执行更可预测
- ✅ 风险控制更可靠

---

**报告生成时间**: 2025-11-03
**审查人员**: Claude Code
**状态**: ✅ 完成
