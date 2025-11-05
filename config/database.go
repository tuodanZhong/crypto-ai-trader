package config

import (
	"crypto/rand"
	"database/sql"
	"encoding/base32"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Database 配置数据库
type Database struct {
	db *sql.DB
}

// NewDatabase 创建配置数据库
func NewDatabase(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	database := &Database{db: db}
	if err := database.createTables(); err != nil {
		return nil, fmt.Errorf("创建表失败: %w", err)
	}

	if err := database.initDefaultData(); err != nil {
		return nil, fmt.Errorf("初始化默认数据失败: %w", err)
	}

	return database, nil
}

// createTables 创建数据库表
func (d *Database) createTables() error {
	queries := []string{
		// AI模型配置表
		`CREATE TABLE IF NOT EXISTS ai_models (
			unique_id TEXT PRIMARY KEY,
			id TEXT NOT NULL,
			user_id TEXT NOT NULL DEFAULT 'default',
			name TEXT NOT NULL,
			provider TEXT NOT NULL,
			config_alias TEXT DEFAULT '',
			enabled BOOLEAN DEFAULT 0,
			api_key TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,

		// 交易所配置表
		`CREATE TABLE IF NOT EXISTS exchanges (
			unique_id TEXT PRIMARY KEY,
			id TEXT NOT NULL,
			user_id TEXT NOT NULL DEFAULT 'default',
			name TEXT NOT NULL,
			type TEXT NOT NULL, -- 'cex' or 'dex'
			config_alias TEXT DEFAULT '',
			enabled BOOLEAN DEFAULT 0,
			api_key TEXT DEFAULT '',
			secret_key TEXT DEFAULT '',
			testnet BOOLEAN DEFAULT 0,
			-- Hyperliquid 特定字段
			hyperliquid_wallet_addr TEXT DEFAULT '',
			-- Aster 特定字段
			aster_user TEXT DEFAULT '',
			aster_signer TEXT DEFAULT '',
			aster_private_key TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,

		// 交易员配置表
		`CREATE TABLE IF NOT EXISTS traders (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL DEFAULT 'default',
			name TEXT NOT NULL,
			ai_model_unique_id TEXT NOT NULL,
			exchange_unique_id TEXT NOT NULL,
			initial_balance REAL NOT NULL,
			scan_interval_minutes INTEGER DEFAULT 3,
			is_running BOOLEAN DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY (ai_model_unique_id) REFERENCES ai_models(unique_id),
			FOREIGN KEY (exchange_unique_id) REFERENCES exchanges(unique_id)
		)`,

		// 用户表
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			otp_secret TEXT,
			otp_verified BOOLEAN DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// 系统配置表
		`CREATE TABLE IF NOT EXISTS system_config (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// 触发器：自动更新 updated_at
		`CREATE TRIGGER IF NOT EXISTS update_users_updated_at
			AFTER UPDATE ON users
			BEGIN
				UPDATE users SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
			END`,

		`CREATE TRIGGER IF NOT EXISTS update_ai_models_updated_at
			AFTER UPDATE ON ai_models
			BEGIN
				UPDATE ai_models SET updated_at = CURRENT_TIMESTAMP WHERE unique_id = NEW.unique_id;
			END`,

		`CREATE TRIGGER IF NOT EXISTS update_exchanges_updated_at
			AFTER UPDATE ON exchanges
			BEGIN
				UPDATE exchanges SET updated_at = CURRENT_TIMESTAMP WHERE unique_id = NEW.unique_id;
			END`,

		`CREATE TRIGGER IF NOT EXISTS update_traders_updated_at
			AFTER UPDATE ON traders
			BEGIN
				UPDATE traders SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
			END`,

		`CREATE TRIGGER IF NOT EXISTS update_system_config_updated_at
			AFTER UPDATE ON system_config
			BEGIN
				UPDATE system_config SET updated_at = CURRENT_TIMESTAMP WHERE key = NEW.key;
			END`,
	}

	for _, query := range queries {
		if _, err := d.db.Exec(query); err != nil {
			return fmt.Errorf("执行SQL失败 [%s]: %w", query, err)
		}
	}

	// 为现有数据库添加新字段（向后兼容）
	alterQueries := []string{
		`ALTER TABLE exchanges ADD COLUMN hyperliquid_wallet_addr TEXT DEFAULT ''`,
		`ALTER TABLE exchanges ADD COLUMN aster_user TEXT DEFAULT ''`,
		`ALTER TABLE exchanges ADD COLUMN aster_signer TEXT DEFAULT ''`,
		`ALTER TABLE exchanges ADD COLUMN aster_private_key TEXT DEFAULT ''`,
		`ALTER TABLE traders ADD COLUMN custom_prompt TEXT DEFAULT ''`,
		`ALTER TABLE traders ADD COLUMN override_base_prompt BOOLEAN DEFAULT 0`,
		`ALTER TABLE ai_models ADD COLUMN unique_id TEXT DEFAULT ''`,
		`ALTER TABLE ai_models ADD COLUMN config_alias TEXT DEFAULT ''`,
		`ALTER TABLE exchanges ADD COLUMN unique_id TEXT DEFAULT ''`,
		`ALTER TABLE exchanges ADD COLUMN config_alias TEXT DEFAULT ''`,
		`ALTER TABLE traders ADD COLUMN ai_model_unique_id TEXT DEFAULT ''`,
		`ALTER TABLE traders ADD COLUMN exchange_unique_id TEXT DEFAULT ''`,
		`ALTER TABLE traders ADD COLUMN ai500_coin_limit INTEGER DEFAULT 20`,
	}

	for _, query := range alterQueries {
		// 忽略已存在字段的错误
		d.db.Exec(query)
	}

	// 数据迁移：从旧结构迁移到新结构
	err := d.migrateToMultiConfig()
	if err != nil {
		log.Printf("⚠️ 迁移到多配置结构失败: %v", err)
		return err
	}

	return nil
}

// initDefaultData 初始化默认数据
func (d *Database) initDefaultData() error {
	// 初始化AI模型（使用default用户）
	aiModels := []struct {
		id, name, provider string
	}{
		{"deepseek", "DeepSeek", "deepseek"},
		{"qwen", "Qwen", "qwen"},
	}

	for _, model := range aiModels {
		uniqueID := fmt.Sprintf("default_%s_%d", model.id, time.Now().UnixNano())
		_, err := d.db.Exec(`
			INSERT OR IGNORE INTO ai_models (unique_id, id, user_id, name, provider, enabled)
			VALUES (?, ?, 'default', ?, ?, 0)
		`, uniqueID, model.id, model.name, model.provider)
		if err != nil {
			return fmt.Errorf("初始化AI模型失败: %w", err)
		}
	}

	// 初始化交易所（使用default用户）
	exchanges := []struct {
		id, name, typ string
	}{
		{"binance", "Binance Futures", "binance"},
		{"hyperliquid", "Hyperliquid", "hyperliquid"},
		{"aster", "Aster DEX", "aster"},
	}

	for _, exchange := range exchanges {
		uniqueID := fmt.Sprintf("default_%s_%d", exchange.id, time.Now().UnixNano())
		_, err := d.db.Exec(`
			INSERT OR IGNORE INTO exchanges (unique_id, id, user_id, name, type, enabled)
			VALUES (?, ?, 'default', ?, ?, 0)
		`, uniqueID, exchange.id, exchange.name, exchange.typ)
		if err != nil {
			return fmt.Errorf("初始化交易所失败: %w", err)
		}
	}

	// 初始化系统配置
	systemConfigs := map[string]string{
		"api_server_port":       "8080",
		"use_default_coins":     "true",
		"coin_pool_api_url":     "",
		"oi_top_api_url":        "",
		"max_daily_loss":        "10.0",
		"max_drawdown":          "20.0",
		"stop_trading_minutes":  "60",
	}

	for key, value := range systemConfigs {
		_, err := d.db.Exec(`
			INSERT OR IGNORE INTO system_config (key, value) 
			VALUES (?, ?)
		`, key, value)
		if err != nil {
			return fmt.Errorf("初始化系统配置失败: %w", err)
		}
	}

	return nil
}

// migrateToMultiConfig 迁移数据库到支持多配置的新结构
func (d *Database) migrateToMultiConfig() error {
	// 检查是否已经迁移过（通过检查是否有数据已有unique_id）
	var count int
	err := d.db.QueryRow(`
		SELECT COUNT(*) FROM ai_models WHERE unique_id IS NOT NULL AND unique_id != ''
	`).Scan(&count)
	if err != nil {
		return err
	}

	// 如果已经有数据有unique_id，说明已经迁移过
	if count > 0 {
		return nil
	}

	log.Printf("🔄 开始迁移数据库到多配置结构...")

	// 1. 为ai_models生成unique_id
	rows, err := d.db.Query(`SELECT id, user_id FROM ai_models WHERE unique_id IS NULL OR unique_id = ''`)
	if err != nil {
		return fmt.Errorf("查询ai_models失败: %w", err)
	}

	type modelRow struct {
		id     string
		userID string
	}
	var models []modelRow
	for rows.Next() {
		var m modelRow
		if err := rows.Scan(&m.id, &m.userID); err != nil {
			rows.Close()
			return err
		}
		models = append(models, m)
	}
	rows.Close()

	for _, m := range models {
		uniqueID := fmt.Sprintf("%s_%s_%d", m.userID, m.id, time.Now().UnixNano())
		_, err = d.db.Exec(`UPDATE ai_models SET unique_id = ? WHERE id = ? AND user_id = ?`, uniqueID, m.id, m.userID)
		if err != nil {
			return fmt.Errorf("更新ai_models unique_id失败: %w", err)
		}
	}

	// 2. 为exchanges生成unique_id
	rows, err = d.db.Query(`SELECT id, user_id FROM exchanges WHERE unique_id IS NULL OR unique_id = ''`)
	if err != nil {
		return fmt.Errorf("查询exchanges失败: %w", err)
	}

	type exchangeRow struct {
		id     string
		userID string
	}
	var exchanges []exchangeRow
	for rows.Next() {
		var e exchangeRow
		if err := rows.Scan(&e.id, &e.userID); err != nil {
			rows.Close()
			return err
		}
		exchanges = append(exchanges, e)
	}
	rows.Close()

	for _, e := range exchanges {
		uniqueID := fmt.Sprintf("%s_%s_%d", e.userID, e.id, time.Now().UnixNano())
		_, err = d.db.Exec(`UPDATE exchanges SET unique_id = ? WHERE id = ? AND user_id = ?`, uniqueID, e.id, e.userID)
		if err != nil {
			return fmt.Errorf("更新exchanges unique_id失败: %w", err)
		}
	}

	// 3. 更新traders表的外键引用
	// 首先检查旧列是否存在
	var hasOldColumns bool
	err = d.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('traders') WHERE name IN ('ai_model_id', 'exchange_id')`).Scan(&hasOldColumns)
	if err != nil || !hasOldColumns {
		// 旧列不存在，跳过迁移
		log.Printf("✅ 数据库已经是新结构，跳过traders迁移")
		return nil
	}

	rows, err = d.db.Query(`
		SELECT t.id, t.ai_model_id, t.exchange_id, t.user_id
		FROM traders t
		WHERE (t.ai_model_unique_id IS NULL OR t.ai_model_unique_id = '')
		   OR (t.exchange_unique_id IS NULL OR t.exchange_unique_id = '')
	`)
	if err != nil {
		return fmt.Errorf("查询traders失败: %w", err)
	}

	type traderRow struct {
		id         string
		modelID    string
		exchangeID string
		userID     string
	}
	var traders []traderRow
	for rows.Next() {
		var t traderRow
		if err := rows.Scan(&t.id, &t.modelID, &t.exchangeID, &t.userID); err != nil {
			rows.Close()
			return err
		}
		traders = append(traders, t)
	}
	rows.Close()

	for _, t := range traders {
		// 查找对应的model unique_id
		var modelUniqueID string
		err = d.db.QueryRow(`SELECT unique_id FROM ai_models WHERE id = ? AND user_id = ?`, t.modelID, t.userID).Scan(&modelUniqueID)
		if err != nil {
			log.Printf("⚠️ 找不到模型 id=%s, user_id=%s: %v", t.modelID, t.userID, err)
			continue
		}

		// 查找对应的exchange unique_id
		var exchangeUniqueID string
		err = d.db.QueryRow(`SELECT unique_id FROM exchanges WHERE id = ? AND user_id = ?`, t.exchangeID, t.userID).Scan(&exchangeUniqueID)
		if err != nil {
			log.Printf("⚠️ 找不到交易所 id=%s, user_id=%s: %v", t.exchangeID, t.userID, err)
			continue
		}

		// 更新trader
		_, err = d.db.Exec(`
			UPDATE traders
			SET ai_model_unique_id = ?, exchange_unique_id = ?
			WHERE id = ?
		`, modelUniqueID, exchangeUniqueID, t.id)
		if err != nil {
			return fmt.Errorf("更新trader外键失败: %w", err)
		}
	}

	log.Printf("✅ 数据库迁移到多配置结构完成")
	return nil
}

// User 用户配置
type User struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	PasswordHash string   `json:"-"` // 不返回到前端
	OTPSecret   string    `json:"-"` // 不返回到前端
	OTPVerified bool      `json:"otp_verified"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// AIModelConfig AI模型配置
type AIModelConfig struct {
	UniqueID    string    `json:"unique_id"`
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Name        string    `json:"name"`
	Provider    string    `json:"provider"`
	ConfigAlias string    `json:"config_alias"`
	Enabled     bool      `json:"enabled"`
	APIKey      string    `json:"apiKey"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ExchangeConfig 交易所配置
type ExchangeConfig struct {
	UniqueID    string    `json:"unique_id"`
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	ConfigAlias string    `json:"config_alias"`
	Enabled     bool      `json:"enabled"`
	APIKey      string    `json:"apiKey"`
	SecretKey   string    `json:"secretKey"`
	Testnet     bool      `json:"testnet"`
	// Hyperliquid 特定字段
	HyperliquidWalletAddr string `json:"hyperliquidWalletAddr"`
	// Aster 特定字段
	AsterUser       string    `json:"asterUser"`
	AsterSigner     string    `json:"asterSigner"`
	AsterPrivateKey string    `json:"asterPrivateKey"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// TraderRecord 交易员配置（数据库实体）
type TraderRecord struct {
	ID                  string    `json:"id"`
	UserID              string    `json:"user_id"`
	Name                string    `json:"name"`
	AIModelUniqueID     string    `json:"ai_model_unique_id"`
	ExchangeUniqueID    string    `json:"exchange_unique_id"`
	InitialBalance      float64   `json:"initial_balance"`
	ScanIntervalMinutes int       `json:"scan_interval_minutes"`
	IsRunning           bool      `json:"is_running"`
	CustomPrompt        string    `json:"custom_prompt"`        // 自定义交易策略prompt
	OverrideBasePrompt  bool      `json:"override_base_prompt"` // 是否覆盖基础prompt
	AI500CoinLimit      int       `json:"ai500_coin_limit"`     // AI500分析币种数量（默认20）
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// GenerateOTPSecret 生成OTP密钥
func GenerateOTPSecret() (string, error) {
	secret := make([]byte, 20)
	_, err := rand.Read(secret)
	if err != nil {
		return "", err
	}
	return base32.StdEncoding.EncodeToString(secret), nil
}

// CreateUser 创建用户
func (d *Database) CreateUser(user *User) error {
	_, err := d.db.Exec(`
		INSERT INTO users (id, email, password_hash, otp_secret, otp_verified)
		VALUES (?, ?, ?, ?, ?)
	`, user.ID, user.Email, user.PasswordHash, user.OTPSecret, user.OTPVerified)
	return err
}

// EnsureAdminUser 确保admin用户存在（用于管理员模式）
func (d *Database) EnsureAdminUser() error {
	// 检查admin用户是否已存在
	var count int
	err := d.db.QueryRow(`SELECT COUNT(*) FROM users WHERE id = 'admin'`).Scan(&count)
	if err != nil {
		return err
	}
	
	// 如果已存在，直接返回
	if count > 0 {
		return nil
	}
	
	// 创建admin用户（密码为空，因为管理员模式下不需要密码）
	adminUser := &User{
		ID:           "admin",
		Email:        "admin@localhost",
		PasswordHash: "", // 管理员模式下不使用密码
		OTPSecret:    "",
		OTPVerified:  true,
	}
	
	return d.CreateUser(adminUser)
}

// GetUserByEmail 通过邮箱获取用户
func (d *Database) GetUserByEmail(email string) (*User, error) {
	var user User
	err := d.db.QueryRow(`
		SELECT id, email, password_hash, otp_secret, otp_verified, created_at, updated_at
		FROM users WHERE email = ?
	`, email).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.OTPSecret, 
		&user.OTPVerified, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByID 通过ID获取用户
func (d *Database) GetUserByID(userID string) (*User, error) {
	var user User
	err := d.db.QueryRow(`
		SELECT id, email, password_hash, otp_secret, otp_verified, created_at, updated_at
		FROM users WHERE id = ?
	`, userID).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.OTPSecret, 
		&user.OTPVerified, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// UpdateUserOTPVerified 更新用户OTP验证状态
func (d *Database) UpdateUserOTPVerified(userID string, verified bool) error {
	_, err := d.db.Exec(`UPDATE users SET otp_verified = ? WHERE id = ?`, verified, userID)
	return err
}

// GetAIModels 获取用户的AI模型配置
func (d *Database) GetAIModels(userID string) ([]*AIModelConfig, error) {
	rows, err := d.db.Query(`
		SELECT unique_id, id, user_id, name, provider,
		       COALESCE(config_alias, '') as config_alias,
		       enabled, api_key, created_at, updated_at
		FROM ai_models WHERE user_id = ? ORDER BY id, created_at
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// 初始化为空切片而不是nil，确保JSON序列化为[]而不是null
	models := make([]*AIModelConfig, 0)
	for rows.Next() {
		var model AIModelConfig
		err := rows.Scan(
			&model.UniqueID, &model.ID, &model.UserID, &model.Name, &model.Provider,
			&model.ConfigAlias, &model.Enabled, &model.APIKey,
			&model.CreatedAt, &model.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		models = append(models, &model)
	}

	return models, nil
}

// UpdateAIModel 更新AI模型配置（基于unique_id）
func (d *Database) UpdateAIModel(uniqueID string, enabled bool, apiKey string, configAlias string) error {
	_, err := d.db.Exec(`
		UPDATE ai_models
		SET enabled = ?, api_key = ?, config_alias = ?, updated_at = datetime('now')
		WHERE unique_id = ?
	`, enabled, apiKey, configAlias, uniqueID)
	return err
}

// CreateAIModelConfig 创建新的AI模型配置
func (d *Database) CreateAIModelConfig(userID, modelID, configAlias, apiKey string) (*AIModelConfig, error) {
	// 获取模型的基本信息
	var name, provider string
	err := d.db.QueryRow(`
		SELECT name, provider FROM ai_models WHERE id = ? AND user_id = 'default' LIMIT 1
	`, modelID).Scan(&name, &provider)
	if err != nil {
		// 如果找不到基本信息，使用默认值
		if modelID == "deepseek" {
			name = "DeepSeek"
			provider = "deepseek"
		} else if modelID == "qwen" {
			name = "Qwen"
			provider = "qwen"
		} else {
			return nil, fmt.Errorf("未知的模型类型: %s", modelID)
		}
	}

	// 生成unique_id
	uniqueID := fmt.Sprintf("%s_%s_%d", userID, modelID, time.Now().UnixNano())

	// 创建配置
	_, err = d.db.Exec(`
		INSERT INTO ai_models (unique_id, id, user_id, name, provider, config_alias, enabled, api_key, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 1, ?, datetime('now'), datetime('now'))
	`, uniqueID, modelID, userID, name, provider, configAlias, apiKey)
	if err != nil {
		return nil, err
	}

	// 返回新创建的配置
	return &AIModelConfig{
		UniqueID:    uniqueID,
		ID:          modelID,
		UserID:      userID,
		Name:        name,
		Provider:    provider,
		ConfigAlias: configAlias,
		Enabled:     true,
		APIKey:      apiKey,
	}, nil
}

// DeleteAIModel 删除AI模型配置（真删除）
func (d *Database) DeleteAIModel(uniqueID, userID string) error {
	// 检查是否有交易员正在使用这个配置
	var count int
	err := d.db.QueryRow(`
		SELECT COUNT(*) FROM traders WHERE ai_model_unique_id = ? AND user_id = ?
	`, uniqueID, userID).Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		return fmt.Errorf("该模型配置正被 %d 个交易员使用，无法删除", count)
	}

	// 删除配置
	result, err := d.db.Exec(`DELETE FROM ai_models WHERE unique_id = ? AND user_id = ?`, uniqueID, userID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("配置不存在或无权限删除")
	}

	return nil
}

// GetExchanges 获取用户的交易所配置
func (d *Database) GetExchanges(userID string) ([]*ExchangeConfig, error) {
	rows, err := d.db.Query(`
		SELECT unique_id, id, user_id, name, type,
		       COALESCE(config_alias, '') as config_alias,
		       enabled, api_key, secret_key, testnet,
		       COALESCE(hyperliquid_wallet_addr, '') as hyperliquid_wallet_addr,
		       COALESCE(aster_user, '') as aster_user,
		       COALESCE(aster_signer, '') as aster_signer,
		       COALESCE(aster_private_key, '') as aster_private_key,
		       created_at, updated_at
		FROM exchanges WHERE user_id = ? ORDER BY id, created_at
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// 初始化为空切片而不是nil，确保JSON序列化为[]而不是null
	exchanges := make([]*ExchangeConfig, 0)
	for rows.Next() {
		var exchange ExchangeConfig
		err := rows.Scan(
			&exchange.UniqueID, &exchange.ID, &exchange.UserID, &exchange.Name, &exchange.Type,
			&exchange.ConfigAlias, &exchange.Enabled, &exchange.APIKey, &exchange.SecretKey, &exchange.Testnet,
			&exchange.HyperliquidWalletAddr, &exchange.AsterUser,
			&exchange.AsterSigner, &exchange.AsterPrivateKey,
			&exchange.CreatedAt, &exchange.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		exchanges = append(exchanges, &exchange)
	}

	return exchanges, nil
}

// UpdateExchange 更新交易所配置（基于unique_id）
func (d *Database) UpdateExchange(uniqueID string, enabled bool, apiKey, secretKey string, testnet bool, hyperliquidWalletAddr, asterUser, asterSigner, asterPrivateKey, configAlias string) error {
	_, err := d.db.Exec(`
		UPDATE exchanges
		SET enabled = ?, api_key = ?, secret_key = ?, testnet = ?,
		    hyperliquid_wallet_addr = ?, aster_user = ?, aster_signer = ?, aster_private_key = ?,
		    config_alias = ?, updated_at = datetime('now')
		WHERE unique_id = ?
	`, enabled, apiKey, secretKey, testnet, hyperliquidWalletAddr, asterUser, asterSigner, asterPrivateKey, configAlias, uniqueID)
	return err
}

// CreateExchangeConfig 创建新的交易所配置
func (d *Database) CreateExchangeConfig(userID, exchangeID, configAlias, apiKey, secretKey string, testnet bool, hyperliquidWalletAddr, asterUser, asterSigner, asterPrivateKey string) (*ExchangeConfig, error) {
	// 获取交易所的基本信息
	var name, typ string
	err := d.db.QueryRow(`
		SELECT name, type FROM exchanges WHERE id = ? AND user_id = 'default' LIMIT 1
	`, exchangeID).Scan(&name, &typ)
	if err != nil {
		// 如果找不到基本信息，使用默认值
		if exchangeID == "binance" {
			name = "Binance Futures"
			typ = "cex"
		} else if exchangeID == "hyperliquid" {
			name = "Hyperliquid"
			typ = "dex"
		} else if exchangeID == "aster" {
			name = "Aster DEX"
			typ = "dex"
		} else {
			return nil, fmt.Errorf("未知的交易所类型: %s", exchangeID)
		}
	}

	// 生成unique_id
	uniqueID := fmt.Sprintf("%s_%s_%d", userID, exchangeID, time.Now().UnixNano())

	// 创建配置
	_, err = d.db.Exec(`
		INSERT INTO exchanges (unique_id, id, user_id, name, type, config_alias, enabled, api_key, secret_key, testnet,
		                       hyperliquid_wallet_addr, aster_user, aster_signer, aster_private_key, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 1, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))
	`, uniqueID, exchangeID, userID, name, typ, configAlias, apiKey, secretKey, testnet, hyperliquidWalletAddr, asterUser, asterSigner, asterPrivateKey)
	if err != nil {
		return nil, err
	}

	// 返回新创建的配置
	return &ExchangeConfig{
		UniqueID:              uniqueID,
		ID:                    exchangeID,
		UserID:                userID,
		Name:                  name,
		Type:                  typ,
		ConfigAlias:           configAlias,
		Enabled:               true,
		APIKey:                apiKey,
		SecretKey:             secretKey,
		Testnet:               testnet,
		HyperliquidWalletAddr: hyperliquidWalletAddr,
		AsterUser:             asterUser,
		AsterSigner:           asterSigner,
		AsterPrivateKey:       asterPrivateKey,
	}, nil
}

// DeleteExchange 删除交易所配置（真删除）
func (d *Database) DeleteExchange(uniqueID, userID string) error {
	// 检查是否有交易员正在使用这个配置
	var count int
	err := d.db.QueryRow(`
		SELECT COUNT(*) FROM traders WHERE exchange_unique_id = ? AND user_id = ?
	`, uniqueID, userID).Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		return fmt.Errorf("该交易所配置正被 %d 个交易员使用，无法删除", count)
	}

	// 删除配置
	result, err := d.db.Exec(`DELETE FROM exchanges WHERE unique_id = ? AND user_id = ?`, uniqueID, userID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("配置不存在或无权限删除")
	}

	return nil
}

// CreateTrader 创建交易员
func (d *Database) CreateTrader(trader *TraderRecord) error {
	_, err := d.db.Exec(`
		INSERT INTO traders (id, user_id, name, ai_model_unique_id, exchange_unique_id, initial_balance, scan_interval_minutes, is_running, custom_prompt, override_base_prompt)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, trader.ID, trader.UserID, trader.Name, trader.AIModelUniqueID, trader.ExchangeUniqueID, trader.InitialBalance, trader.ScanIntervalMinutes, trader.IsRunning, trader.CustomPrompt, trader.OverrideBasePrompt)
	return err
}

// GetTraders 获取用户的交易员
func (d *Database) GetTraders(userID string) ([]*TraderRecord, error) {
	rows, err := d.db.Query(`
		SELECT id, user_id, name,
		       COALESCE(ai_model_unique_id, '') as ai_model_unique_id,
		       COALESCE(exchange_unique_id, '') as exchange_unique_id,
		       initial_balance, scan_interval_minutes, is_running,
		       COALESCE(custom_prompt, '') as custom_prompt,
		       COALESCE(override_base_prompt, 0) as override_base_prompt,
		       COALESCE(ai500_coin_limit, 20) as ai500_coin_limit,
		       created_at, updated_at
		FROM traders WHERE user_id = ? ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var traders []*TraderRecord
	for rows.Next() {
		var trader TraderRecord
		err := rows.Scan(
			&trader.ID, &trader.UserID, &trader.Name, &trader.AIModelUniqueID, &trader.ExchangeUniqueID,
			&trader.InitialBalance, &trader.ScanIntervalMinutes, &trader.IsRunning,
			&trader.CustomPrompt, &trader.OverrideBasePrompt, &trader.AI500CoinLimit,
			&trader.CreatedAt, &trader.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		traders = append(traders, &trader)
	}

	return traders, nil
}

// UpdateTraderStatus 更新交易员状态
func (d *Database) UpdateTraderStatus(userID, id string, isRunning bool) error {
	_, err := d.db.Exec(`UPDATE traders SET is_running = ? WHERE id = ? AND user_id = ?`, isRunning, id, userID)
	return err
}

// UpdateTraderCustomPrompt 更新交易员自定义Prompt
func (d *Database) UpdateTraderCustomPrompt(userID, id string, customPrompt string, overrideBase bool) error {
	_, err := d.db.Exec(`UPDATE traders SET custom_prompt = ?, override_base_prompt = ? WHERE id = ? AND user_id = ?`, customPrompt, overrideBase, id, userID)
	return err
}

// DeleteTrader 删除交易员
func (d *Database) DeleteTrader(userID, id string) error {
	_, err := d.db.Exec(`DELETE FROM traders WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

// GetTraderConfig 获取交易员完整配置（包含AI模型和交易所信息）
func (d *Database) GetTraderConfig(userID, traderID string) (*TraderRecord, *AIModelConfig, *ExchangeConfig, error) {
	var trader TraderRecord
	var aiModel AIModelConfig
	var exchange ExchangeConfig

	err := d.db.QueryRow(`
		SELECT
			t.id, t.user_id, t.name, t.ai_model_unique_id, t.exchange_unique_id, t.initial_balance, t.scan_interval_minutes, t.is_running, t.created_at, t.updated_at,
			a.unique_id, a.id, a.user_id, a.name, a.provider, COALESCE(a.config_alias, ''), a.enabled, a.api_key, a.created_at, a.updated_at,
			e.unique_id, e.id, e.user_id, e.name, e.type, COALESCE(e.config_alias, ''), e.enabled, e.api_key, e.secret_key, e.testnet,
			COALESCE(e.hyperliquid_wallet_addr, '') as hyperliquid_wallet_addr,
			COALESCE(e.aster_user, '') as aster_user,
			COALESCE(e.aster_signer, '') as aster_signer,
			COALESCE(e.aster_private_key, '') as aster_private_key,
			e.created_at, e.updated_at
		FROM traders t
		JOIN ai_models a ON t.ai_model_unique_id = a.unique_id
		JOIN exchanges e ON t.exchange_unique_id = e.unique_id
		WHERE t.id = ? AND t.user_id = ?
	`, traderID, userID).Scan(
		&trader.ID, &trader.UserID, &trader.Name, &trader.AIModelUniqueID, &trader.ExchangeUniqueID,
		&trader.InitialBalance, &trader.ScanIntervalMinutes, &trader.IsRunning,
		&trader.CreatedAt, &trader.UpdatedAt,
		&aiModel.UniqueID, &aiModel.ID, &aiModel.UserID, &aiModel.Name, &aiModel.Provider, &aiModel.ConfigAlias, &aiModel.Enabled, &aiModel.APIKey,
		&aiModel.CreatedAt, &aiModel.UpdatedAt,
		&exchange.UniqueID, &exchange.ID, &exchange.UserID, &exchange.Name, &exchange.Type, &exchange.ConfigAlias, &exchange.Enabled,
		&exchange.APIKey, &exchange.SecretKey, &exchange.Testnet,
		&exchange.HyperliquidWalletAddr, &exchange.AsterUser, &exchange.AsterSigner, &exchange.AsterPrivateKey,
		&exchange.CreatedAt, &exchange.UpdatedAt,
	)

	if err != nil {
		return nil, nil, nil, err
	}

	return &trader, &aiModel, &exchange, nil
}

// GetSystemConfig 获取系统配置
func (d *Database) GetSystemConfig(key string) (string, error) {
	var value string
	err := d.db.QueryRow(`SELECT value FROM system_config WHERE key = ?`, key).Scan(&value)
	return value, err
}

// SetSystemConfig 设置系统配置
func (d *Database) SetSystemConfig(key, value string) error {
	_, err := d.db.Exec(`
		INSERT OR REPLACE INTO system_config (key, value) VALUES (?, ?)
	`, key, value)
	return err
}

// Close 关闭数据库连接
func (d *Database) Close() error {
	return d.db.Close()
}