package decision

import (
	"encoding/json"
	"fmt"
	"log"
	"nofx/market"
	"nofx/mcp"
	"nofx/pool"
	"strings"
	"time"
)

// PositionInfo 持仓信息
type PositionInfo struct {
	Symbol           string  `json:"symbol"`
	Side             string  `json:"side"` // "long" or "short"
	EntryPrice       float64 `json:"entry_price"`
	MarkPrice        float64 `json:"mark_price"`
	Quantity         float64 `json:"quantity"`
	Leverage         int     `json:"leverage"`
	UnrealizedPnL    float64 `json:"unrealized_pnl"`
	UnrealizedPnLPct float64 `json:"unrealized_pnl_pct"`
	LiquidationPrice float64 `json:"liquidation_price"`
	MarginUsed       float64 `json:"margin_used"`
	UpdateTime       int64   `json:"update_time"` // 持仓更新时间戳（毫秒）
}

// AccountInfo 账户信息
type AccountInfo struct {
	TotalEquity      float64 `json:"total_equity"`      // 账户净值
	AvailableBalance float64 `json:"available_balance"` // 可用余额
	TotalPnL         float64 `json:"total_pnl"`         // 总盈亏
	TotalPnLPct      float64 `json:"total_pnl_pct"`     // 总盈亏百分比
	MarginUsed       float64 `json:"margin_used"`       // 已用保证金
	MarginUsedPct    float64 `json:"margin_used_pct"`   // 保证金使用率
	PositionCount    int     `json:"position_count"`    // 持仓数量
}

// CandidateCoin 候选币种（来自币种池）
type CandidateCoin struct {
	Symbol  string   `json:"symbol"`
	Sources []string `json:"sources"` // 来源: "ai500" 和/或 "oi_top"
}

// OITopData 持仓量增长Top数据（用于AI决策参考）
type OITopData struct {
	Rank              int     // OI Top排名
	OIDeltaPercent    float64 // 持仓量变化百分比（1小时）
	OIDeltaValue      float64 // 持仓量变化价值
	PriceDeltaPercent float64 // 价格变化百分比
	NetLong           float64 // 净多仓
	NetShort          float64 // 净空仓
}

// Context 交易上下文（传递给AI的完整信息）
type Context struct {
	CurrentTime     string                  `json:"current_time"`
	RuntimeMinutes  int                     `json:"runtime_minutes"`
	CallCount       int                     `json:"call_count"`
	Account         AccountInfo             `json:"account"`
	Positions       []PositionInfo          `json:"positions"`
	CandidateCoins  []CandidateCoin         `json:"candidate_coins"`
	MarketDataMap   map[string]*market.Data `json:"-"` // 不序列化，但内部使用
	OITopDataMap    map[string]*OITopData   `json:"-"` // OI Top数据映射
	Performance     interface{}             `json:"-"` // 历史表现分析（logger.PerformanceAnalysis）
	BTCETHLeverage  int                     `json:"-"` // BTC/ETH杠杆倍数（从配置读取）
	AltcoinLeverage int                     `json:"-"` // 山寨币杠杆倍数（从配置读取）
}

// Decision AI的交易决策
type Decision struct {
	Symbol          string  `json:"symbol"`
	Action          string  `json:"action"` // "open_long", "open_short", "close_long", "close_short", "hold", "wait"
	Leverage        int     `json:"leverage,omitempty"`
	PositionSizeUSD float64 `json:"position_size_usd,omitempty"`
	StopLoss        float64 `json:"stop_loss,omitempty"`
	TakeProfit      float64 `json:"take_profit,omitempty"`
	Confidence      int     `json:"confidence,omitempty"` // 信心度 (0-100)
	RiskUSD         float64 `json:"risk_usd,omitempty"`   // 最大美元风险
	Reasoning       string  `json:"reasoning"`
}

// FullDecision AI的完整决策（包含思维链）
type FullDecision struct {
	UserPrompt string     `json:"user_prompt"` // 发送给AI的输入prompt
	CoTTrace   string     `json:"cot_trace"`   // 思维链分析（AI输出）
	Decisions  []Decision `json:"decisions"`   // 具体决策列表
	Timestamp  time.Time  `json:"timestamp"`
}

// GetFullDecision 获取AI的完整交易决策（批量分析所有币种和持仓）
func GetFullDecision(ctx *Context, mcpClient *mcp.Client) (*FullDecision, error) {
	return GetFullDecisionWithCustomPrompt(ctx, mcpClient, "", false)
}

// GetFullDecisionWithCustomPrompt 获取AI的完整交易决策（支持自定义prompt）
func GetFullDecisionWithCustomPrompt(ctx *Context, mcpClient *mcp.Client, customPrompt string, overrideBase bool) (*FullDecision, error) {
	// 1. 为所有币种获取市场数据
	if err := fetchMarketDataForContext(ctx); err != nil {
		return nil, fmt.Errorf("获取市场数据失败: %w", err)
	}

	// 2. 构建 System Prompt（固定规则）和 User Prompt（动态数据）
	systemPrompt := buildSystemPromptWithCustom(ctx.Account.TotalEquity, ctx.BTCETHLeverage, ctx.AltcoinLeverage, customPrompt, overrideBase)
	userPrompt := buildUserPrompt(ctx)

	// 3. 调用AI API（使用 system + user prompt）
	aiResponse, err := mcpClient.CallWithMessages(systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("调用AI API失败: %w", err)
	}

	// 4. 解析AI响应
	decision, err := parseFullDecisionResponse(aiResponse, ctx.Account.TotalEquity, ctx.BTCETHLeverage, ctx.AltcoinLeverage)
	if err != nil {
		return nil, fmt.Errorf("解析AI响应失败: %w", err)
	}

	decision.Timestamp = time.Now()
	decision.UserPrompt = userPrompt // 保存输入prompt
	return decision, nil
}

// fetchMarketDataForContext 为上下文中的所有币种获取市场数据和OI数据
func fetchMarketDataForContext(ctx *Context) error {
	ctx.MarketDataMap = make(map[string]*market.Data)
	ctx.OITopDataMap = make(map[string]*OITopData)

	// 收集所有需要获取数据的币种
	symbolSet := make(map[string]bool)

	// 1. 优先获取持仓币种的数据（这是必须的）
	for _, pos := range ctx.Positions {
		symbolSet[pos.Symbol] = true
	}

	// 2. 候选币种数量根据账户状态动态调整
	maxCandidates := calculateMaxCandidates(ctx)
	for i, coin := range ctx.CandidateCoins {
		if i >= maxCandidates {
			break
		}
		symbolSet[coin.Symbol] = true
	}

	// 并发获取市场数据
	// 持仓币种集合（用于判断是否跳过OI检查）
	positionSymbols := make(map[string]bool)
	for _, pos := range ctx.Positions {
		positionSymbols[pos.Symbol] = true
	}

	for symbol := range symbolSet {
		data, err := market.Get(symbol)
		if err != nil {
			// 单个币种失败不影响整体，只记录错误
			continue
		}

		// ⚠️ 流动性过滤：持仓价值低于15M USD的币种不做（多空都不做）
		// 持仓价值 = 持仓量 × 当前价格
		// 但现有持仓必须保留（需要决策是否平仓）
		isExistingPosition := positionSymbols[symbol]
		if !isExistingPosition && data.OpenInterest != nil && data.CurrentPrice > 0 {
			// 计算持仓价值（USD）= 持仓量 × 当前价格
			oiValue := data.OpenInterest.Latest * data.CurrentPrice
			oiValueInMillions := oiValue / 1_000_000 // 转换为百万美元单位
			if oiValueInMillions < 15 {
				log.Printf("⚠️  %s 持仓价值过低(%.2fM USD < 15M)，跳过此币种 [持仓量:%.0f × 价格:%.4f]",
					symbol, oiValueInMillions, data.OpenInterest.Latest, data.CurrentPrice)
				continue
			}
		}

		ctx.MarketDataMap[symbol] = data
	}

	// 加载OI Top数据（不影响主流程）
	oiPositions, err := pool.GetOITopPositions()
	if err == nil {
		for _, pos := range oiPositions {
			// 标准化符号匹配
			symbol := pos.Symbol
			ctx.OITopDataMap[symbol] = &OITopData{
				Rank:              pos.Rank,
				OIDeltaPercent:    pos.OIDeltaPercent,
				OIDeltaValue:      pos.OIDeltaValue,
				PriceDeltaPercent: pos.PriceDeltaPercent,
				NetLong:           pos.NetLong,
				NetShort:          pos.NetShort,
			}
		}
	}

	return nil
}

// calculateMaxCandidates 根据账户状态计算需要分析的候选币种数量
func calculateMaxCandidates(ctx *Context) int {
	// 直接返回候选池的全部币种数量
	// 因为候选池已经在 auto_trader.go 中筛选过了
	// 固定分析前20个评分最高的币种（来自AI500）
	return len(ctx.CandidateCoins)
}

// buildSystemPromptWithCustom 构建包含自定义内容的 System Prompt
func buildSystemPromptWithCustom(accountEquity float64, btcEthLeverage, altcoinLeverage int, customPrompt string, overrideBase bool) string {
	// 如果覆盖基础prompt且有自定义prompt，只使用自定义prompt
	if overrideBase && customPrompt != "" {
		return customPrompt
	}
	
	// 获取基础prompt
	basePrompt := buildSystemPrompt(accountEquity, btcEthLeverage, altcoinLeverage)
	
	// 如果没有自定义prompt，直接返回基础prompt
	if customPrompt == "" {
		return basePrompt
	}
	
	// 添加自定义prompt部分到基础prompt
	var sb strings.Builder
	sb.WriteString(basePrompt)
	sb.WriteString("\n\n")
	sb.WriteString("# 📌 个性化交易策略\n\n")
	sb.WriteString(customPrompt)
	sb.WriteString("\n\n")
	sb.WriteString("**注意**: 以上个性化策略是对基础规则的补充，不能违背基础风险控制原则。\n")
	
	return sb.String()
}

// buildSystemPrompt 构建 System Prompt（固定规则，可缓存）
func buildSystemPrompt(accountEquity float64, btcEthLeverage, altcoinLeverage int) string {
	var sb strings.Builder

	// === 核心使命 ===
	sb.WriteString("你是专业的加密货币交易AI，在币安合约市场进行自主交易。\n\n")
	sb.WriteString("# 🎯 核心目标\n\n")
	sb.WriteString("**最大化夏普比率（Sharpe Ratio）**\n\n")
	sb.WriteString("夏普比率 = 平均收益 / 收益波动率\n\n")
	sb.WriteString("**这意味着**：\n")
	sb.WriteString("- ✅ 高质量交易（高胜率、大盈亏比）→ 提升夏普\n")
	sb.WriteString("- ✅ 稳定收益、控制回撤 → 提升夏普\n")
	sb.WriteString("- ✅ 耐心持仓、让利润奔跑 → 提升夏普\n")
	sb.WriteString("- ❌ 频繁交易、小盈小亏 → 增加波动，严重降低夏普\n")
	sb.WriteString("- ❌ 过度交易、手续费损耗 → 直接亏损\n")
	sb.WriteString("- ❌ 过早平仓、频繁进出 → 错失大行情\n\n")
	sb.WriteString("**关键认知**: 系统每3分钟扫描一次，但不意味着每次都要交易！\n")
	sb.WriteString("大多数时候应该是 `wait` 或 `hold`，只在极佳机会时才开仓。\n\n")

	// === 硬约束（风险控制）===
	sb.WriteString("# ⚖️ 硬约束（风险控制）\n\n")
	sb.WriteString("1. **风险回报比**: 必须 ≥ 1:3（冒1%风险，赚3%+收益）\n")
	sb.WriteString("2. **最多持仓**: 3个币种（质量>数量）\n")
	sb.WriteString(fmt.Sprintf("3. **单币仓位**: 山寨%.0f-%.0f U(%dx杠杆) | BTC/ETH %.0f-%.0f U(%dx杠杆)\n",
		accountEquity*0.8, accountEquity*1.5, altcoinLeverage, accountEquity*5, accountEquity*10, btcEthLeverage))
	sb.WriteString("4. **保证金**: 总使用率 ≤ 90%\n\n")

	// === 做空激励 ===
	sb.WriteString("# 📉 做多做空平衡\n\n")
	sb.WriteString("**重要**: 下跌趋势做空的利润 = 上涨趋势做多的利润\n\n")
	sb.WriteString("- 上涨趋势 → 做多\n")
	sb.WriteString("- 下跌趋势 → 做空\n")
	sb.WriteString("- 震荡市场 → 观望\n\n")
	sb.WriteString("**不要有做多偏见！做空是你的核心工具之一**\n\n")

	// === 交易频率认知 ===
	sb.WriteString("# ⏱️ 交易频率认知\n\n")
	sb.WriteString("**量化标准**:\n")
	sb.WriteString("- 优秀交易员：每天2-4笔 = 每小时0.1-0.2笔\n")
	sb.WriteString("- 过度交易：每小时>2笔 = 严重问题\n")
	sb.WriteString("- 最佳节奏：开仓后持有至少30-60分钟\n\n")
	sb.WriteString("**自查**:\n")
	sb.WriteString("如果你发现自己每个周期都在交易 → 说明标准太低\n")
	sb.WriteString("如果你发现持仓<30分钟就平仓 → 说明太急躁\n\n")

	// === 开仓信号强度 ===
	sb.WriteString("# 🎯 开仓标准（严格）\n\n")
	sb.WriteString("只在**强信号**时开仓，不确定就观望。\n\n")
	sb.WriteString("**你拥有的完整数据**：\n")
	sb.WriteString("- 📊 **原始序列**：3分钟价格序列(MidPrices数组) + 4小时K线序列\n")
	sb.WriteString("- 📈 **技术序列**：EMA20序列、MACD序列、RSI7序列、RSI14序列\n")
	sb.WriteString("- 💰 **资金序列**：成交量序列、持仓量(OI)序列、资金费率\n")
	sb.WriteString("- 🎯 **筛选标记**：AI500评分 / OI_Top排名（如果有标注）\n\n")
	sb.WriteString("**分析方法**（完全由你自主决定）：\n")
	sb.WriteString("- 自由运用序列数据，你可以做但不限于趋势分析、形态识别、支撑阻力、技术阻力位、斐波那契、波动带计算\n")
	sb.WriteString("- 多维度交叉验证（价格+量+OI+指标+序列形态）\n")
	sb.WriteString("- 用你认为最有效的方法发现高确定性机会\n")
	sb.WriteString("- 综合信心度 ≥ 75 才开仓\n\n")
	sb.WriteString("**避免低质量信号**：\n")
	sb.WriteString("- 单一维度（只看一个指标）\n")
	sb.WriteString("- 相互矛盾（涨但量萎缩）\n")
	sb.WriteString("- 横盘震荡\n")
	sb.WriteString("- 刚平仓不久（<15分钟）\n\n")

	// === 夏普比率自我进化 ===
	sb.WriteString("# 🧬 夏普比率自我进化\n\n")
	sb.WriteString("每次你会收到**夏普比率**作为绩效反馈（周期级别）：\n\n")
	sb.WriteString("**夏普比率 < -0.5** (持续亏损):\n")
	sb.WriteString("  → 🛑 停止交易，连续观望至少6个周期（18分钟）\n")
	sb.WriteString("  → 🔍 深度反思：\n")
	sb.WriteString("     • 交易频率过高？（每小时>2次就是过度）\n")
	sb.WriteString("     • 持仓时间过短？（<30分钟就是过早平仓）\n")
	sb.WriteString("     • 信号强度不足？（信心度<75）\n")
	sb.WriteString("     • 是否在做空？（单边做多是错误的）\n\n")
	sb.WriteString("**夏普比率 -0.5 ~ 0** (轻微亏损):\n")
	sb.WriteString("  → ⚠️ 严格控制：只做信心度>80的交易\n")
	sb.WriteString("  → 减少交易频率：每小时最多1笔新开仓\n")
	sb.WriteString("  → 耐心持仓：至少持有30分钟以上\n\n")
	sb.WriteString("**夏普比率 0 ~ 0.7** (正收益):\n")
	sb.WriteString("  → ✅ 维持当前策略\n\n")
	sb.WriteString("**夏普比率 > 0.7** (优异表现):\n")
	sb.WriteString("  → 🚀 可适度扩大仓位\n\n")
	sb.WriteString("**关键**: 夏普比率是唯一指标，它会自然惩罚频繁交易和过度进出。\n\n")

	// === 决策流程 ===
	sb.WriteString("# 📋 决策流程\n\n")
	sb.WriteString("1. **分析夏普比率**: 当前策略是否有效？需要调整吗？\n")
	sb.WriteString("2. **评估持仓**: 趋势是否改变？是否该止盈/止损？\n")
	sb.WriteString("3. **寻找新机会**: 有强信号吗？多空机会？\n")
	sb.WriteString("4. **输出决策**: 思维链分析 + JSON\n\n")

	// === P0优化: 市场环境识别 ===
	sb.WriteString("# 🌍 市场环境识别（必做）\n\n")
	sb.WriteString("**每次决策前必须先判断市场环境**，不同环境使用不同策略：\n\n")

	sb.WriteString("## 🔥 趋势市场（最佳交易环境）\n\n")
	sb.WriteString("**识别标准**（需同时满足）：\n")
	sb.WriteString("1. **EMA排列**: \n")
	sb.WriteString("   - 上涨趋势: 价格 > EMA20 > EMA50，且EMA20向上倾斜\n")
	sb.WriteString("   - 下跌趋势: 价格 < EMA20 < EMA50，且EMA20向下倾斜\n")
	sb.WriteString("2. **MACD确认**: MACD柱状图连续3根同向（上涨为正，下跌为负）\n")
	sb.WriteString("3. **RSI验证**: 上涨趋势RSI>50，下跌趋势RSI<50\n")
	sb.WriteString("4. **成交量配合**: 趋势方向成交量放大（当前量>前5根平均量的1.2倍）\n\n")
	sb.WriteString("**交易策略**：\n")
	sb.WriteString("- ✅ **积极开仓**: 顺势交易，信心度≥75即可\n")
	sb.WriteString("- ✅ **持仓耐心**: 至少持仓60分钟，让利润奔跑\n")
	sb.WriteString("- ✅ **移动止损**: 盈利1R后启动移动止损\n")
	sb.WriteString("- ❌ **禁止逆势**: 不做反向交易（下跌趋势不做多，上涨趋势不做空）\n\n")

	sb.WriteString("## 📊 震荡市场（谨慎环境）\n\n")
	sb.WriteString("**识别标准**（满足2项以上）：\n")
	sb.WriteString("1. **价格区间**: 价格在EMA20上下±3%范围内反复震荡\n")
	sb.WriteString("2. **RSI中性**: RSI在40-60区间横盘\n")
	sb.WriteString("3. **MACD弱势**: MACD柱状图正负交替，无明显方向\n")
	sb.WriteString("4. **成交量萎缩**: 成交量低于前10根平均量的80%\n\n")
	sb.WriteString("**交易策略**：\n")
	sb.WriteString("- ⚠️ **极度谨慎**: 信心度必须≥85才开仓\n")
	sb.WriteString("- ⚠️ **快进快出**: 目标1.5-2R即可止盈，不贪婪\n")
	sb.WriteString("- ⚠️ **严格止损**: 不给亏损留空间\n")
	sb.WriteString("- 🎯 **首选**: 等待突破震荡区间后再顺势入场\n\n")

	sb.WriteString("## ⚡ 高波动环境（危险环境）\n\n")
	sb.WriteString("**识别标准**（满足任意1项）：\n")
	sb.WriteString("1. **ATR暴涨**: 当前ATR > 近20根ATR均值的1.5倍\n")
	sb.WriteString("2. **价格跳跃**: 单根K线涨跌幅 > 5%\n")
	sb.WriteString("3. **资金费率极端**: 绝对值 > 0.1%（年化36%）\n")
	sb.WriteString("4. **OI急剧变化**: 持仓量24h变化 > ±20%\n\n")
	sb.WriteString("**交易策略**：\n")
	sb.WriteString("- 🛑 **优先观望**: 高波动 = 高风险，宁可错过\n")
	sb.WriteString("- 🔍 **极端标准**: 信心度≥90 + 多重指标共振才考虑\n")
	sb.WriteString("- 📉 **仓位减半**: 如果必须开仓，仓位降低50%\n")
	sb.WriteString("- ⏱️ **快速止盈**: 盈利1.5R立即止盈50%，不恋战\n\n")

	// === P0优化: 仓位管理规则 ===
	sb.WriteString("# 🎯 仓位管理规则（强制执行）\n\n")

	sb.WriteString("## 🔄 移动止损机制\n\n")
	sb.WriteString("**触发条件**: 持仓盈利达到1R（风险单位）时自动启动\n\n")
	sb.WriteString("**执行规则**:\n")
	sb.WriteString("- **盈利1-2R**: 移动止损至保本点（入场价）\n")
	sb.WriteString("- **盈利2-3R**: 移动止损至1R盈利位置（锁定1R利润）\n")
	sb.WriteString("- **盈利>3R**: 止损跟随价格，始终保持2R利润锁定\n\n")
	sb.WriteString("**重要**: 移动止损永不回撤，只能向盈利方向移动\n\n")

	sb.WriteString("## 💰 分批止盈规则\n\n")
	sb.WriteString("**分批止盈计划**（强烈建议）:\n")
	sb.WriteString("1. **50%仓位@2R**: 盈利达到2R时，平仓50%锁定利润\n")
	sb.WriteString("2. **30%仓位@3R**: 盈利达到3R时，再平仓30%\n")
	sb.WriteString("3. **20%仓位Trailing**: 剩余20%使用移动止损，追求更大收益\n\n")
	sb.WriteString("**为什么分批止盈？**\n")
	sb.WriteString("- ✅ 平衡风险与收益\n")
	sb.WriteString("- ✅ 避免回吐利润\n")
	sb.WriteString("- ✅ 心理上更从容\n")
	sb.WriteString("- ✅ 提升夏普比率（稳定收益）\n\n")

	sb.WriteString("## 🚨 强制止损触发条件\n\n")
	sb.WriteString("**立即平仓的强制情况**（不等止损价）:\n")
	sb.WriteString("1. **趋势反转**: 做多时价格跌破EMA50 + MACD死叉\n")
	sb.WriteString("2. **极端波动**: 单根K线反向波动>5%（可能爆仓）\n")
	sb.WriteString("3. **基本面冲击**: 重大利空消息（需人工干预判断）\n")
	sb.WriteString("4. **止损价触及**: 市场价格触及预设止损价\n\n")
	sb.WriteString("**重要**: 止损是保命工具，永不抗单，永不幻想反弹\n\n")

	sb.WriteString("## ⏳ 持仓时间管理\n\n")
	sb.WriteString("**最小持仓时间**: 30分钟（避免频繁进出）\n\n")
	sb.WriteString("**例外情况**（可提前平仓）:\n")
	sb.WriteString("- 止损触发（立即平仓）\n")
	sb.WriteString("- 趋势反转（强制平仓条件满足）\n")
	sb.WriteString("- 盈利≥2R且出现反转信号（止盈离场）\n\n")
	sb.WriteString("**最大持仓时间**: 无上限，只要趋势不改变就持有\n\n")
	sb.WriteString("**避免**: 因焦虑、无聊而提前平仓（这是亏损的主要原因）\n\n")

	// === P0优化: 夏普比率智能诊断 ===
	sb.WriteString("# 🧠 夏普比率智能诊断\n\n")
	sb.WriteString("**每次决策前必做的5点自我诊断**：\n\n")

	sb.WriteString("## 1️⃣ 胜率诊断\n\n")
	sb.WriteString("**目标**: 胜率≥50%（理想60%+）\n\n")
	sb.WriteString("**如果胜率<50%**，检查：\n")
	sb.WriteString("- ❓ 开仓标准是否太低？（信心度<75的交易太多？）\n")
	sb.WriteString("- ❓ 是否在震荡市频繁交易？（应该等趋势）\n")
	sb.WriteString("- ❓ 是否逆势交易？（下跌趋势做多，上涨趋势做空）\n")
	sb.WriteString("- ❓ 止损设置是否太紧？（应给趋势足够空间）\n\n")

	sb.WriteString("## 2️⃣ 盈亏比诊断\n\n")
	sb.WriteString("**目标**: 盈亏比≥3:1（平均盈利/平均亏损）\n\n")
	sb.WriteString("**如果盈亏比<3:1**，检查：\n")
	sb.WriteString("- ❓ 是否过早止盈？（1R就跑，错失大行情）\n")
	sb.WriteString("- ❓ 止损是否太宽松？（亏损过多）\n")
	sb.WriteString("- ❓ 是否在做分批止盈？（应该执行50%@2R, 30%@3R）\n\n")

	sb.WriteString("## 3️⃣ 持仓时间诊断\n\n")
	sb.WriteString("**目标**: 平均持仓时间≥60分钟\n\n")
	sb.WriteString("**如果平均<30分钟**，这是严重问题：\n")
	sb.WriteString("- ❌ 频繁进出 → 手续费吃掉利润\n")
	sb.WriteString("- ❌ 没让利润奔跑 → 错失大行情\n")
	sb.WriteString("- ❌ 心态浮躁 → 降低决策质量\n\n")
	sb.WriteString("**改进**: 开仓后至少持有60分钟，除非触发强制平仓条件\n\n")

	sb.WriteString("## 4️⃣ 回撤控制诊断\n\n")
	sb.WriteString("**目标**: 最大回撤≤15%（净值从高点回落的最大幅度）\n\n")
	sb.WriteString("**如果回撤>15%**，立即：\n")
	sb.WriteString("- 🛑 暂停交易，至少观望6个周期（18分钟）\n")
	sb.WriteString("- 🔍 深度反思：哪些交易是错误的？\n")
	sb.WriteString("- 📊 降低仓位：下次开仓减少50%仓位\n\n")

	sb.WriteString("## 5️⃣ 波动率诊断\n\n")
	sb.WriteString("**目标**: 收益波动率适中（不追求暴利）\n\n")
	sb.WriteString("**警惕信号**：\n")
	sb.WriteString("- ⚠️ 单日收益波动>±20% → 仓位过大或交易过激进\n")
	sb.WriteString("- ⚠️ 频繁爆仓风险 → 杠杆过高\n\n")
	sb.WriteString("**改进**: 稳定收益>暴利，夏普比率才是王道\n\n")

	// === 输出格式 ===
	sb.WriteString("# 📤 输出格式\n\n")
	sb.WriteString("**第一步: 思维链（纯文本）**\n")
	sb.WriteString("简洁分析你的思考过程\n\n")
	sb.WriteString("**第二步: JSON决策数组**\n\n")
	sb.WriteString("```json\n[\n")
	sb.WriteString(fmt.Sprintf("  {\"symbol\": \"BTCUSDT\", \"action\": \"open_short\", \"leverage\": %d, \"position_size_usd\": %.0f, \"stop_loss\": 97000, \"take_profit\": 91000, \"confidence\": 85, \"risk_usd\": 300, \"reasoning\": \"下跌趋势+MACD死叉\"},\n", btcEthLeverage, accountEquity*5))
	sb.WriteString("  {\"symbol\": \"ETHUSDT\", \"action\": \"close_long\", \"reasoning\": \"止盈离场\"}\n")
	sb.WriteString("]\n```\n\n")
	sb.WriteString("**字段说明**:\n")
	sb.WriteString("- `action`: open_long | open_short | close_long | close_short | hold | wait\n")
	sb.WriteString("- `confidence`: 0-100（开仓建议≥75）\n")
	sb.WriteString("- 开仓时必填: leverage, position_size_usd, stop_loss, take_profit, confidence, risk_usd, reasoning\n\n")

	// === 关键提醒 ===
	sb.WriteString("---\n\n")
	sb.WriteString("**记住**: \n")
	sb.WriteString("- 目标是夏普比率，不是交易频率\n")
	sb.WriteString("- 做空 = 做多，都是赚钱工具\n")
	sb.WriteString("- 宁可错过，不做低质量交易\n")
	sb.WriteString("- 风险回报比1:3是底线\n")

	return sb.String()
}

// buildUserPrompt 构建 User Prompt（动态数据）
func buildUserPrompt(ctx *Context) string {
	var sb strings.Builder

	// 系统状态
	sb.WriteString(fmt.Sprintf("**时间**: %s | **周期**: #%d | **运行**: %d分钟\n\n",
		ctx.CurrentTime, ctx.CallCount, ctx.RuntimeMinutes))

	// BTC 市场
	if btcData, hasBTC := ctx.MarketDataMap["BTCUSDT"]; hasBTC {
		sb.WriteString(fmt.Sprintf("**BTC**: %.2f (1h: %+.2f%%, 4h: %+.2f%%) | MACD: %.4f | RSI: %.2f\n\n",
			btcData.CurrentPrice, btcData.PriceChange1h, btcData.PriceChange4h,
			btcData.CurrentMACD, btcData.CurrentRSI7))
	}

	// 账户
	sb.WriteString(fmt.Sprintf("**账户**: 净值%.2f | 余额%.2f (%.1f%%) | 盈亏%+.2f%% | 保证金%.1f%% | 持仓%d个\n\n",
		ctx.Account.TotalEquity,
		ctx.Account.AvailableBalance,
		(ctx.Account.AvailableBalance/ctx.Account.TotalEquity)*100,
		ctx.Account.TotalPnLPct,
		ctx.Account.MarginUsedPct,
		ctx.Account.PositionCount))

	// 持仓（完整市场数据）
	if len(ctx.Positions) > 0 {
		sb.WriteString("## 当前持仓\n")
		for i, pos := range ctx.Positions {
			// 计算持仓时长
			holdingDuration := ""
			if pos.UpdateTime > 0 {
				durationMs := time.Now().UnixMilli() - pos.UpdateTime
				durationMin := durationMs / (1000 * 60) // 转换为分钟
				if durationMin < 60 {
					holdingDuration = fmt.Sprintf(" | 持仓时长%d分钟", durationMin)
				} else {
					durationHour := durationMin / 60
					durationMinRemainder := durationMin % 60
					holdingDuration = fmt.Sprintf(" | 持仓时长%d小时%d分钟", durationHour, durationMinRemainder)
				}
			}

			sb.WriteString(fmt.Sprintf("%d. %s %s | 入场价%.4f 当前价%.4f | 盈亏%+.2f%% | 杠杆%dx | 保证金%.0f | 强平价%.4f%s\n\n",
				i+1, pos.Symbol, strings.ToUpper(pos.Side),
				pos.EntryPrice, pos.MarkPrice, pos.UnrealizedPnLPct,
				pos.Leverage, pos.MarginUsed, pos.LiquidationPrice, holdingDuration))

			// 使用FormatMarketData输出完整市场数据
			if marketData, ok := ctx.MarketDataMap[pos.Symbol]; ok {
				sb.WriteString(market.Format(marketData))
				sb.WriteString("\n")
			}
		}
	} else {
		sb.WriteString("**当前持仓**: 无\n\n")
	}

	// 候选币种（完整市场数据）
	sb.WriteString(fmt.Sprintf("## 候选币种 (%d个)\n\n", len(ctx.MarketDataMap)))
	displayedCount := 0
	for _, coin := range ctx.CandidateCoins {
		marketData, hasData := ctx.MarketDataMap[coin.Symbol]
		if !hasData {
			continue
		}
		displayedCount++

		sourceTags := ""
		if len(coin.Sources) > 1 {
			sourceTags = " (AI500+OI_Top双重信号)"
		} else if len(coin.Sources) == 1 && coin.Sources[0] == "oi_top" {
			sourceTags = " (OI_Top持仓增长)"
		}

		// 使用FormatMarketData输出完整市场数据
		sb.WriteString(fmt.Sprintf("### %d. %s%s\n\n", displayedCount, coin.Symbol, sourceTags))
		sb.WriteString(market.Format(marketData))
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	// 夏普比率（直接传值，不要复杂格式化）
	if ctx.Performance != nil {
		// 直接从interface{}中提取SharpeRatio
		type PerformanceData struct {
			SharpeRatio float64 `json:"sharpe_ratio"`
		}
		var perfData PerformanceData
		if jsonData, err := json.Marshal(ctx.Performance); err == nil {
			if err := json.Unmarshal(jsonData, &perfData); err == nil {
				sb.WriteString(fmt.Sprintf("## 📊 夏普比率: %.2f\n\n", perfData.SharpeRatio))
			}
		}
	}

	sb.WriteString("---\n\n")
	sb.WriteString("现在请分析并输出决策（思维链 + JSON）\n")

	return sb.String()
}

// parseFullDecisionResponse 解析AI的完整决策响应
func parseFullDecisionResponse(aiResponse string, accountEquity float64, btcEthLeverage, altcoinLeverage int) (*FullDecision, error) {
	// 1. 提取思维链
	cotTrace := extractCoTTrace(aiResponse)

	// 2. 提取JSON决策列表
	decisions, err := extractDecisions(aiResponse)
	if err != nil {
		return &FullDecision{
			CoTTrace:  cotTrace,
			Decisions: []Decision{},
		}, fmt.Errorf("提取决策失败: %w\n\n=== AI思维链分析 ===\n%s", err, cotTrace)
	}

	// 3. 验证决策
	if err := validateDecisions(decisions, accountEquity, btcEthLeverage, altcoinLeverage); err != nil {
		return &FullDecision{
			CoTTrace:  cotTrace,
			Decisions: decisions,
		}, fmt.Errorf("决策验证失败: %w\n\n=== AI思维链分析 ===\n%s", err, cotTrace)
	}

	return &FullDecision{
		CoTTrace:  cotTrace,
		Decisions: decisions,
	}, nil
}

// extractCoTTrace 提取思维链分析
func extractCoTTrace(response string) string {
	// 查找JSON数组的开始位置
	jsonStart := strings.Index(response, "[")

	if jsonStart > 0 {
		// 思维链是JSON数组之前的内容
		return strings.TrimSpace(response[:jsonStart])
	}

	// 如果找不到JSON，整个响应都是思维链
	return strings.TrimSpace(response)
}

// extractDecisions 提取JSON决策列表
func extractDecisions(response string) ([]Decision, error) {
	// 直接查找JSON数组 - 找第一个完整的JSON数组
	arrayStart := strings.Index(response, "[")
	if arrayStart == -1 {
		return nil, fmt.Errorf("无法找到JSON数组起始")
	}

	// 从 [ 开始，匹配括号找到对应的 ]
	arrayEnd := findMatchingBracket(response, arrayStart)
	if arrayEnd == -1 {
		return nil, fmt.Errorf("无法找到JSON数组结束")
	}

	jsonContent := strings.TrimSpace(response[arrayStart : arrayEnd+1])

	// 🔧 修复常见的JSON格式错误：缺少引号的字段值
	// 匹配: "reasoning": 内容"}  或  "reasoning": 内容}  (没有引号)
	// 修复为: "reasoning": "内容"}
	// 使用简单的字符串扫描而不是正则表达式
	jsonContent = fixMissingQuotes(jsonContent)

	// 解析JSON
	var decisions []Decision
	if err := json.Unmarshal([]byte(jsonContent), &decisions); err != nil {
		return nil, fmt.Errorf("JSON解析失败: %w\nJSON内容: %s", err, jsonContent)
	}

	return decisions, nil
}

// fixMissingQuotes 替换中文引号为英文引号（避免输入法自动转换）
func fixMissingQuotes(jsonStr string) string {
	jsonStr = strings.ReplaceAll(jsonStr, "\u201c", "\"") // "
	jsonStr = strings.ReplaceAll(jsonStr, "\u201d", "\"") // "
	jsonStr = strings.ReplaceAll(jsonStr, "\u2018", "'")  // '
	jsonStr = strings.ReplaceAll(jsonStr, "\u2019", "'")  // '
	return jsonStr
}

// validateDecisions 验证所有决策（需要账户信息和杠杆配置）
func validateDecisions(decisions []Decision, accountEquity float64, btcEthLeverage, altcoinLeverage int) error {
	for i, decision := range decisions {
		if err := validateDecision(&decision, accountEquity, btcEthLeverage, altcoinLeverage); err != nil {
			return fmt.Errorf("决策 #%d 验证失败: %w", i+1, err)
		}
	}
	return nil
}

// findMatchingBracket 查找匹配的右括号
func findMatchingBracket(s string, start int) int {
	if start >= len(s) || s[start] != '[' {
		return -1
	}

	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return i
			}
		}
	}

	return -1
}

// validateDecision 验证单个决策的有效性
func validateDecision(d *Decision, accountEquity float64, btcEthLeverage, altcoinLeverage int) error {
	// 验证action
	validActions := map[string]bool{
		"open_long":   true,
		"open_short":  true,
		"close_long":  true,
		"close_short": true,
		"hold":        true,
		"wait":        true,
	}

	if !validActions[d.Action] {
		return fmt.Errorf("无效的action: %s", d.Action)
	}

	// 开仓操作必须提供完整参数
	if d.Action == "open_long" || d.Action == "open_short" {
		// 根据币种使用配置的杠杆上限
		maxLeverage := altcoinLeverage          // 山寨币使用配置的杠杆
		maxPositionValue := accountEquity * 1.5 // 山寨币最多1.5倍账户净值
		if d.Symbol == "BTCUSDT" || d.Symbol == "ETHUSDT" {
			maxLeverage = btcEthLeverage          // BTC和ETH使用配置的杠杆
			maxPositionValue = accountEquity * 10 // BTC/ETH最多10倍账户净值
		}

		if d.Leverage <= 0 || d.Leverage > maxLeverage {
			return fmt.Errorf("杠杆必须在1-%d之间（%s，当前配置上限%d倍）: %d", maxLeverage, d.Symbol, maxLeverage, d.Leverage)
		}
		if d.PositionSizeUSD <= 0 {
			return fmt.Errorf("仓位大小必须大于0: %.2f", d.PositionSizeUSD)
		}
		// 验证仓位价值上限（加1%容差以避免浮点数精度问题）
		tolerance := maxPositionValue * 0.01 // 1%容差
		if d.PositionSizeUSD > maxPositionValue+tolerance {
			if d.Symbol == "BTCUSDT" || d.Symbol == "ETHUSDT" {
				return fmt.Errorf("BTC/ETH单币种仓位价值不能超过%.0f USDT（10倍账户净值），实际: %.0f", maxPositionValue, d.PositionSizeUSD)
			} else {
				return fmt.Errorf("山寨币单币种仓位价值不能超过%.0f USDT（1.5倍账户净值），实际: %.0f", maxPositionValue, d.PositionSizeUSD)
			}
		}
		if d.StopLoss <= 0 || d.TakeProfit <= 0 {
			return fmt.Errorf("止损和止盈必须大于0")
		}

		// 验证止损止盈的合理性
		if d.Action == "open_long" {
			if d.StopLoss >= d.TakeProfit {
				return fmt.Errorf("做多时止损价必须小于止盈价")
			}
		} else {
			if d.StopLoss <= d.TakeProfit {
				return fmt.Errorf("做空时止损价必须大于止盈价")
			}
		}

		// ⚠️ P0修复: 风险回报比验证改进
		// ⚠️ 风险回报比验证 - 已禁用
		//
		// 原因: 使用midPoint=(SL+TP)/2作为假设入场价存在数学问题:
		//   riskDistance = |SL - midPoint| = |SL - TP|/2
		//   rewardDistance = |TP - midPoint| = |SL - TP|/2
		//   → riskDistance 永远等于 rewardDistance (比例恒为1:1)
		//   → 任何>1:1的验证要求都会拒绝所有交易
		//
		// 解决方案: 需要使用实际入场价(当前市价)进行验证，而不是midPoint
		//
		// 当前策略: 只验证TP/SL方向正确(上面已验证)，具体比例由AI根据System Prompt
		//          中的"风险回报比≥1:3"要求自主判断
		//
		// 详细分析: 参考 CRITICAL_BUG_REPORT.md 和 FINAL_FIX_SUMMARY.md

		/*
		// 由于验证时不知道实际入场价（由市场实时价格决定），我们进行基础合理性检查：
		// 验证止盈空间至少是止损空间的2倍（保守估计，实际执行时会更严格）
		if d.Action == "open_long" {
			// 做多：假设从止损和止盈的中点入场（最保守估计）
			midPoint := (d.StopLoss + d.TakeProfit) / 2
			riskDistance := midPoint - d.StopLoss
			rewardDistance := d.TakeProfit - midPoint

			if rewardDistance < riskDistance*2.0 {
				return fmt.Errorf("止盈止损比例不合理: 止盈空间(%.2f)应至少是止损空间(%.2f)的2倍 [SL:%.2f TP:%.2f]",
					rewardDistance, riskDistance, d.StopLoss, d.TakeProfit)
			}
		} else {
			// 做空：假设从止损和止盈的中点入场（最保守估计）
			midPoint := (d.StopLoss + d.TakeProfit) / 2
			riskDistance := d.StopLoss - midPoint
			rewardDistance := midPoint - d.TakeProfit

			if rewardDistance < riskDistance*2.0 {
				return fmt.Errorf("止盈止损比例不合理: 止盈空间(%.2f)应至少是止损空间(%.2f)的2倍 [SL:%.2f TP:%.2f]",
					rewardDistance, riskDistance, d.StopLoss, d.TakeProfit)
			}
		}

		// 注意：实际执行时会使用真实市场价格进行更精确的风险回报比验证
		*/
	}

	return nil
}
