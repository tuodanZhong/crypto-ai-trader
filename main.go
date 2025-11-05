package main

import (
	"fmt"
	"log"
	"nofx/api"
	"nofx/auth"
	"nofx/config"
	"nofx/manager"
	"nofx/pool"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
)

func main() {
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║    🤖 AI多模型交易系统 - 支持 DeepSeek & Qwen            ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// 初始化数据库配置
	dbPath := "config.db"
	if len(os.Args) > 1 {
		dbPath = os.Args[1]
	}

	log.Printf("📋 初始化配置数据库: %s", dbPath)
	database, err := config.NewDatabase(dbPath)
	if err != nil {
		log.Fatalf("❌ 初始化数据库失败: %v", err)
	}
	defer database.Close()

	// 获取系统配置
	useDefaultCoinsStr, _ := database.GetSystemConfig("use_default_coins")
	useDefaultCoins := useDefaultCoinsStr == "true"
	apiPortStr, _ := database.GetSystemConfig("api_server_port")
	
	// 获取管理员模式配置
	adminModeStr, _ := database.GetSystemConfig("admin_mode")
	adminMode := adminModeStr != "false" // 默认为true
	
	// 设置JWT密钥
	jwtSecret, _ := database.GetSystemConfig("jwt_secret")
	if jwtSecret == "" {
		jwtSecret = "your-jwt-secret-key-change-in-production-make-it-long-and-random"
		log.Printf("⚠️  使用默认JWT密钥，建议在生产环境中配置")
	}
	auth.SetJWTSecret(jwtSecret)
	
	// 在管理员模式下，确保admin用户存在
	if adminMode {
		err := database.EnsureAdminUser()
		if err != nil {
			log.Printf("⚠️  创建admin用户失败: %v", err)
		} else {
			log.Printf("✓ 管理员模式已启用，无需登录")
		}
		auth.SetAdminMode(true)
	}
	
	log.Printf("✓ 配置数据库初始化成功")
	fmt.Println()

	// 设置默认主流币种列表
	defaultCoins := []string{"BTC", "ETH", "SOL", "BNB", "XRP", "DOGE", "ADA", "HYPE"}
	pool.SetDefaultCoins(defaultCoins)

	// 设置是否使用默认主流币种
	pool.SetUseDefaultCoins(useDefaultCoins)
	if useDefaultCoins {
		log.Printf("✓ 已启用默认主流币种列表（BTC、ETH、SOL、BNB、XRP、DOGE、ADA、HYPE）")
	}

	// 设置币种池API URL
	coinPoolAPIURL, _ := database.GetSystemConfig("coin_pool_api_url")
	if coinPoolAPIURL != "" {
		pool.SetCoinPoolAPI(coinPoolAPIURL)
		log.Printf("✓ 已配置AI500币种池API")
	}
	
	oiTopAPIURL, _ := database.GetSystemConfig("oi_top_api_url")
	if oiTopAPIURL != "" {
		pool.SetOITopAPI(oiTopAPIURL)
		log.Printf("✓ 已配置OI Top API")
	}

	// 创建TraderManager
	traderManager := manager.NewTraderManager()

	// 从数据库加载所有交易员到内存
	err = traderManager.LoadTradersFromDatabase(database)
	if err != nil {
		log.Fatalf("❌ 加载交易员失败: %v", err)
	}

	// 获取数据库中的所有交易员配置（用于显示，使用default用户）
	traders, err := database.GetTraders("default")
	if err != nil {
		log.Fatalf("❌ 获取交易员列表失败: %v", err)
	}

	// 显示加载的交易员信息
	fmt.Println()
	fmt.Println("🤖 数据库中的AI交易员配置:")
	if len(traders) == 0 {
		fmt.Println("  • 暂无配置的交易员，请通过Web界面创建")
	} else {
		for _, trader := range traders {
			status := "停止"
			if trader.IsRunning {
				status = "运行中"
			}
			fmt.Printf("  • %s - 初始资金: %.0f USDT [%s]\n",
				trader.Name, trader.InitialBalance, status)
		}
	}

	fmt.Println()
	fmt.Println("🤖 AI全权决策模式:")
	fmt.Printf("  • AI将自主决定每笔交易的杠杆倍数（山寨币最高5倍，BTC/ETH最高5倍）\n")
	fmt.Println("  • AI将自主决定每笔交易的仓位大小")
	fmt.Println("  • AI将自主设置止损和止盈价格")
	fmt.Println("  • AI将基于市场数据、技术指标、账户状态做出全面分析")
	fmt.Println()
	fmt.Println("⚠️  风险提示: AI自动交易有风险，建议小额资金测试！")
	fmt.Println()
	fmt.Println("按 Ctrl+C 停止运行")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println()

	// 获取API服务器端口
    apiPort := 8080 // 默认端口
	if apiPortStr != "" {
		if port, err := strconv.Atoi(apiPortStr); err == nil {
			apiPort = port
		}
	}

	// 创建并启动API服务器
	apiServer := api.NewServer(traderManager, database, apiPort)
	go func() {
		if err := apiServer.Start(); err != nil {
			log.Printf("❌ API服务器错误: %v", err)
		}
	}()

	// 设置优雅退出
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// TODO: 启动数据库中配置为运行状态的交易员
	// traderManager.StartAll()

	// 等待退出信号
	<-sigChan
	fmt.Println()
	fmt.Println()
	log.Println("📛 收到退出信号，正在停止所有trader...")
	traderManager.StopAll()

	fmt.Println()
	fmt.Println("👋 感谢使用AI交易系统！")
}
