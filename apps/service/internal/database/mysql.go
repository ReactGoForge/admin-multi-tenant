package database

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	gormMySQL "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	defaultHost = "127.0.0.1"
	defaultPort = "13306"
	pingTimeout = 5 * time.Second
)

// Config 描述 Go 服务连接当前 MySQL 实例所需的最小配置。
type Config struct {
	// Go 学习提示：结构体把一组相关字段组织成一个类型；这里没有 json 标签，
	// 因为它只在服务内部使用，不会直接序列化给客户端。
	Host     string
	Port     string
	User     string
	Password string
	Database string
}

// LoadConfig 从环境变量读取数据库配置，不加载或修改任何 .env 文件。
func LoadConfig() (Config, error) {
	host := strings.TrimSpace(os.Getenv("MYSQL_HOST"))
	if host == "" {
		host = defaultHost
	}

	port := strings.TrimSpace(os.Getenv("MYSQL_PORT"))
	if port == "" {
		port = defaultPort
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return Config{}, fmt.Errorf("MYSQL_PORT 必须是 1 到 65535 之间的端口号")
	}

	user := strings.TrimSpace(os.Getenv("MYSQL_USER"))
	if user == "" {
		return Config{}, fmt.Errorf("缺少 MYSQL_USER")
	}
	if strings.EqualFold(user, "root") {
		return Config{}, fmt.Errorf("禁止使用 root 账号连接应用数据库")
	}

	password := os.Getenv("MYSQL_PASSWORD")
	if password == "" {
		return Config{}, fmt.Errorf("缺少 MYSQL_PASSWORD")
	}

	databaseName := strings.TrimSpace(os.Getenv("MYSQL_DATABASE"))
	if databaseName == "" {
		return Config{}, fmt.Errorf("缺少 MYSQL_DATABASE")
	}

	return Config{
		Host:     host,
		Port:     port,
		User:     user,
		Password: password,
		Database: databaseName,
	}, nil
}

// dsn 将数据库配置转换为 MySQL 驱动使用的连接字符串。
func (config Config) dsn() string {
	// 安全边界：使用驱动提供的 Config 生成 DSN，避免手工拼接特殊字符导致连接串解析错误。
	driverConfig := mysqlDriver.NewConfig()
	driverConfig.User = config.User
	driverConfig.Passwd = config.Password
	driverConfig.Net = "tcp"
	driverConfig.Addr = net.JoinHostPort(config.Host, config.Port)
	driverConfig.DBName = config.Database
	driverConfig.ParseTime = true
	driverConfig.Loc = time.Local
	driverConfig.Params = map[string]string{"charset": "utf8mb4"}

	return driverConfig.FormatDSN()
}

// Open 创建 GORM 与底层 database/sql 连接，并在五秒超时内验证数据库可用性。
func Open(ctx context.Context, config Config) (*gorm.DB, *sql.DB, error) {
	// Go 学习提示：函数可以返回多个值。这里同时返回 GORM 查询对象、底层连接池和错误；
	// 调用方必须先检查 error，再使用前两个指针。
	db, err := gorm.Open(gormMySQL.Open(config.dsn()), &gorm.Config{
		DisableAutomaticPing: true,
		Logger:               logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("初始化 GORM 失败: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, nil, fmt.Errorf("获取底层数据库连接失败: %w", err)
	}

	// 业务约束：GORM 初始化成功不代表数据库一定可达，所以再用带超时的 Ping 做真实连接验证。
	pingContext, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	if err := sqlDB.PingContext(pingContext); err != nil {
		_ = sqlDB.Close()
		return nil, nil, fmt.Errorf("数据库连接验证失败: %w", err)
	}

	return db, sqlDB, nil
}
