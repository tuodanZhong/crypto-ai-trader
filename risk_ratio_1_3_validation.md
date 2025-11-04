# 风险回报比1:3验证文档

## 修改总结

已将所有风险回报比验证标准统一为 **1:3** (即止盈空间必须≥止损空间的3倍)

---

## 修改内容

### 1. 做多验证 (engine.go:773行)
```go
// 修改前
if rewardDistance < riskDistance*2.0 {

// 修改后
if rewardDistance < riskDistance*3.0 {
```

### 2. 做空验证 (engine.go:789行)
```go
// 修改前
if rewardDistance < riskDistance*2.0 {

// 修改后
if rewardDistance < riskDistance*3.0 {
```

### 3. 做多错误信息 (engine.go:780行)
```go
// 修改前
"...的2倍 [..., 需要≥2:1]"

// 修改后
"...的3倍 [..., 需要≥3:1]"
```

### 4. 做空错误信息 (engine.go:796行)
```go
// 修改前
"...的2倍 [..., 需要≥2:1]"

// 修改后
"...的3倍 [..., 需要≥3:1]"
```

### 5. 注释说明 (engine.go:766行)
```go
// 修改前
// 验证止盈空间至少是止损空间的2倍（保守估计，实际执行时会更严格）

// 修改后
// 验证止盈空间至少是止损空间的3倍（风险回报比1:3，实际执行时会更严格）
```

---

## 一致性检查

### System Prompt (engine.go:255行)
✅ **已一致**: "1. **风险回报比**: 必须 ≥ 1:3（冒1%风险，赚3%+收益）"

### 验证代码 (engine.go:773,789行)
✅ **已一致**: `riskDistance*3.0`

### 错误信息 (engine.go:780,796行)
✅ **已一致**: "需要≥3:1"

### 注释说明 (engine.go:766行)
✅ **已一致**: "风险回报比1:3"

---

## 测试场景

### ❌ 场景1: DOGEUSDT做空 - 原失败案例
**参数**:
- Action: open_short
- Stop Loss: 0.18
- Take Profit: 0.17

**计算**:
```
midPoint = (0.18 + 0.17) / 2 = 0.175
riskDistance = 0.18 - 0.175 = 0.005
rewardDistance = 0.175 - 0.17 = 0.005
ratio = 0.005 / 0.005 = 1.0

验证: 0.005 < 0.005*3.0 (0.015)?
      0.005 < 0.015 ✗ TRUE → 验证失败
```

**新错误信息**:
```
止盈止损比例不合理: 止盈空间(0.0050/2.86%)应至少是止损空间(0.0050/2.86%)的3倍
[做空: TP:0.1700 < entry:0.1750 < SL:0.1800, 实际比例1.00:1, 需要≥3:1]
```

**结论**: ✗ 正确拒绝

---

### ❌ 场景2: DOGEUSDT做空 - 2:1比例 (现在也会被拒绝)
**参数**:
- Action: open_short
- Stop Loss: 0.178
- Take Profit: 0.166

**计算**:
```
midPoint = (0.178 + 0.166) / 2 = 0.172
riskDistance = 0.178 - 0.172 = 0.006
rewardDistance = 0.172 - 0.166 = 0.006
ratio = 0.006 / 0.006 = 1.0

等等,这里计算有误!让我重新计算:
rewardDistance = 0.172 - 0.166 = 0.006 (正确)
ratio = 0.006 / 0.006 = 1.0 (错误!)

实际应该是:
如果我们希望2:1比例:
  reward_needed = 0.006 * 2 = 0.012
  TP_needed = 0.172 - 0.012 = 0.160

如果TP=0.166:
  rewardDistance = 0.172 - 0.166 = 0.006
  ratio = 0.006 / 0.006 = 1.0 (仍然是1:1!)
```

**让我重新设计正确的2:1场景**:
```
假设 entry = 0.172
假设 risk = 0.006 (SL = 0.172 + 0.006 = 0.178)
要达到2:1比例:
  reward = 0.006 * 2 = 0.012
  TP = 0.172 - 0.012 = 0.160

验证:
  midPoint = (0.178 + 0.160) / 2 = 0.169
  riskDistance = 0.178 - 0.169 = 0.009
  rewardDistance = 0.169 - 0.160 = 0.009
  ratio = 0.009 / 0.009 = 1.0
```

**这揭示了midPoint方法的问题!让我重新理解验证逻辑:**

实际上,代码用的是midPoint作为假设入场价,所以:
- 如果TP和SL设置正确,midPoint就是合理的入场价
- riskDistance = |midPoint - SL|
- rewardDistance = |TP - midPoint|
- 如果设置的TP/SL比例正确,那么rewardDistance应该是riskDistance的N倍

**重新计算2:1场景**:
```
要满足2:1,假设midPoint=0.172:
  riskDistance = 0.006
  rewardDistance需要 = 0.006 * 2 = 0.012

  SL = midPoint + risk = 0.172 + 0.006 = 0.178
  TP = midPoint - reward = 0.172 - 0.012 = 0.160

验证:
  midPoint = (0.178 + 0.160) / 2 = 0.169
  riskDistance = 0.178 - 0.169 = 0.009
  rewardDistance = 0.169 - 0.160 = 0.009
  ratio = 1.0 (还是1:1!)
```

**问题根源**: 当我们设置SL和TP后,midPoint=(SL+TP)/2,这导致:
- riskDistance = (SL+TP)/2 - SL = (TP-SL)/2
- rewardDistance = TP - (SL+TP)/2 = (TP-SL)/2
- 所以 riskDistance 永远等于 rewardDistance!

**正确理解**: 验证逻辑要求的不是"从midPoint入场的比例",而是"TP和SL的间距分配"

让我重新设计正确的测试场景:

---

### ✅ 场景2 (修正): DOGEUSDT做空 - 3:1比例
**参数**:
- Action: open_short
- Stop Loss: 0.176
- Take Profit: 0.160

**计算**:
```
midPoint = (0.176 + 0.160) / 2 = 0.168
riskDistance = 0.176 - 0.168 = 0.008 (4.76%)
rewardDistance = 0.168 - 0.160 = 0.008 (4.76%)
ratio = 0.008 / 0.008 = 1.0

验证: 0.008 < 0.008*3.0 (0.024)?
      0.008 < 0.024 ✗ TRUE → 验证失败!
```

**这说明验证逻辑有根本性问题!**

让我重新分析代码逻辑:

```go
// 做空
midPoint := (d.StopLoss + d.TakeProfit) / 2
riskDistance := d.StopLoss - midPoint      // SL到mid的距离
rewardDistance := midPoint - d.TakeProfit  // mid到TP的距离
```

**数学证明**:
```
给定: SL > TP (做空)
midPoint = (SL + TP) / 2

riskDistance = SL - (SL+TP)/2 = (2*SL - SL - TP)/2 = (SL-TP)/2
rewardDistance = (SL+TP)/2 - TP = (SL+TP - 2*TP)/2 = (SL-TP)/2

结论: riskDistance = rewardDistance (永远相等!)
```

**这意味着**:
- 当前验证逻辑下,**任何**TP/SL组合都无法满足reward > risk*N (N>1)
- 这是一个严重的BUG!

---

## ⚠️ 发现严重BUG

### 问题描述
代码中使用midPoint作为假设入场价:
```go
midPoint := (d.StopLoss + d.TakeProfit) / 2
riskDistance := d.StopLoss - midPoint
rewardDistance := midPoint - d.TakeProfit
```

**数学结论**: riskDistance 永远等于 rewardDistance!

这导致:
- ✗ 所有决策的ratio永远是1:1
- ✗ 任何决策都无法通过2:1或3:1的验证
- ✗ 系统将拒绝所有交易!

### 正确的验证逻辑

#### 方案A: 使用当前市场价格作为entry
```go
// 需要传入当前价格
currentPrice := ctx.GetCurrentPrice(d.Symbol)
riskDistance := abs(currentPrice - d.StopLoss)
rewardDistance := abs(d.TakeProfit - currentPrice)
```

#### 方案B: 直接比较TP和SL的距离比例
```go
// 做空: SL > entry > TP
// 假设entry在SL和TP之间,但不是midPoint
// 更合理的是假设entry接近当前价,或者使用TP/SL的绝对距离

// 做空示例:
totalRange := d.StopLoss - d.TakeProfit  // SL到TP的总距离
// 假设entry更接近TP(做空通常是从较低价进入)
// 或者要求: (SL-entry) : (entry-TP) >= 1:3
```

#### 方案C (推荐): 要求总距离的分配比例
```go
// 做空: 要求止盈空间 / 止损空间 >= 3
// 假设entry在某个位置,将总距离分为risk和reward两部分
// 如果entry = TP + x (从TP上方x距离入场)
//   risk = SL - entry = SL - TP - x
//   reward = entry - TP = x
//   要求: x / (SL - TP - x) >= 3
//   即: x >= 3*(SL-TP-x)
//   即: x >= 3*SL - 3*TP - 3*x
//   即: 4*x >= 3*SL - 3*TP
//   即: x >= 3*(SL-TP)/4

// 如果entry = SL - y (从SL下方y距离入场)
//   risk = SL - entry = y
//   reward = entry - TP = SL - y - TP
//   要求: (SL - y - TP) / y >= 3
//   即: SL - y - TP >= 3*y
//   即: SL - TP >= 4*y
//   即: y <= (SL-TP)/4

// 最保守: 要求无论从哪里入场(SL和TP之间任意位置),都能满足3:1
// 这需要: TP距离 >= 3 * SL距离
// 对于做空: (entry - TP) >= 3 * (SL - entry)
// 最坏情况是entry在midPoint: (mid - TP) >= 3 * (SL - mid)
// 即: (SL+TP)/2 - TP >= 3 * (SL - (SL+TP)/2)
// 即: (SL-TP)/2 >= 3 * (SL-TP)/2  ✗ 矛盾!
```

### 建议修复方案

**推荐方案**: 放宽验证要求,只验证TP/SL的方向性,不验证具体比例
- 原因: 在不知道实际入场价的情况下,无法准确验证风险回报比
- 实际的风险回报比应该在**执行时**用真实市场价格验证

或者:

**替代方案**: 使用当前市场价格进行验证
```go
currentPrice := getCurrentMarketPrice(d.Symbol)
if d.Action == "open_short" {
    risk := d.StopLoss - currentPrice
    reward := currentPrice - d.TakeProfit
    if reward < risk*3.0 {
        return error
    }
}
```

---

## 建议下一步

1. **紧急**: 检查是否有其他代码在执行时使用真实价格验证
2. **短期**: 将验证逻辑改为使用当前市场价格
3. **长期**: 在System Prompt中明确告诉AI:
   - 提供当前市场价格作为参考
   - 或者要求TP/SL的设置要考虑当前价格
