package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/auth"
	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/database"
	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/dictionary"
	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/httpapi"
	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/logging"
	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/logquery"
	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/media"
	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/rbac"
	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/user"
)

const (
	serverAddress         = ":8080"
	readHeaderTimeout     = 5 * time.Second
	readTimeout           = 10 * time.Second
	writeTimeout          = 15 * time.Second
	idleTimeout           = 60 * time.Second
	gracefulShutdownLimit = 10 * time.Second
	systemLogCleanupEvery = 24 * time.Hour
)

// miniappCodeCacheAdapter 将现有媒体对象存储收敛为小程序码所需的最小读写能力。
type miniappCodeCacheAdapter struct {
	storage media.Storage
}

// Ready 返回底层私有对象存储是否可用。
func (adapter miniappCodeCacheAdapter) Ready() bool {
	return adapter.storage != nil && adapter.storage.Ready()
}

// Put 将小程序码 PNG 写入现有私有对象存储。
func (adapter miniappCodeCacheAdapter) Put(ctx context.Context, key, contentType string, size int64, reader io.Reader) error {
	return adapter.storage.Put(ctx, key, contentType, size, reader)
}

// Get 从现有私有对象存储读取小程序码流和大小。
func (adapter miniappCodeCacheAdapter) Get(ctx context.Context, key string) (io.ReadCloser, int64, error) {
	object, err := adapter.storage.Get(ctx, key)
	if err != nil {
		return nil, 0, err
	}
	if object == nil || object.Body == nil {
		return nil, 0, errors.New("小程序码缓存对象无效")
	}
	return object.Body, object.Size, nil
}

// main 是 Go HTTP 服务进程入口，负责记录启动失败并结束进程。
func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

// run 加载配置和外部连接，启动 HTTP 服务，并在系统信号到来时优雅退出。
func run() error {
	// Go 学习提示：Context 用于把“进程正在退出”的信号传给数据库、Redis 和后台任务。
	// stop 是清理信号监听的函数，defer 会保证 run 返回前一定调用它。
	rootContext, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	authConfig, err := auth.LoadConfig()
	if err != nil {
		return fmt.Errorf("读取认证配置失败: %w", err)
	}
	databaseConfig, err := database.LoadConfig()
	if err != nil {
		return fmt.Errorf("读取数据库配置失败: %w", err)
	}
	logConfig, err := logging.LoadConfig()
	if err != nil {
		return fmt.Errorf("读取日志配置失败: %w", err)
	}
	var developmentHTTPLogger *logging.DevelopmentHTTPLogger
	if logConfig.DevelopmentHTTPEnabled {
		developmentHTTPLogger, err = logging.NewDevelopmentHTTPLogger("logs")
		if err != nil {
			return fmt.Errorf("初始化开发 HTTP 文件日志失败: %w", err)
		}
		defer func() {
			if closeErr := developmentHTTPLogger.Close(); closeErr != nil {
				log.Printf("关闭开发 HTTP 文件日志失败: %v", closeErr)
			}
		}()
	}
	db, sqlDB, err := database.Open(rootContext, databaseConfig)
	if err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}
	logStore := logging.NewStore(db, logConfig.EventDBEnabled)
	logService := logging.NewService(logStore)
	// Go 学习提示：GORM 的 *gorm.DB 负责构造查询，底层 *sql.DB 才持有真实连接池，
	// 因此服务退出时需要关闭 sqlDB；匿名函数让我们能够顺便记录 Close 返回的错误。
	defer func() {
		if closeErr := sqlDB.Close(); closeErr != nil {
			log.Printf("关闭数据库连接失败: %v", closeErr)
		}
	}()

	captchaManager, err := auth.NewCaptchaManager(rootContext, authConfig)
	if err != nil {
		return fmt.Errorf("初始化验证码服务失败: %w", err)
	}
	defer func() {
		if closeErr := captchaManager.Close(); closeErr != nil {
			log.Printf("关闭 Redis 连接失败: %v", closeErr)
		}
	}()
	if !captchaManager.Enabled() {
		logService.RecordEvent(rootContext, "warn", "后台登录验证码已关闭，仅建议本地开发使用", nil)
	}
	loginLimiter, err := auth.NewLoginLimiter(rootContext, authConfig)
	if err != nil {
		return fmt.Errorf("初始化登录限流服务失败: %w", err)
	}
	defer func() {
		if closeErr := loginLimiter.Close(); closeErr != nil {
			log.Printf("关闭登录限流 Redis 连接失败: %v", closeErr)
		}
	}()
	if !authConfig.LoginRateLimitEnabled {
		logService.RecordEvent(rootContext, "warn", "后台登录限流已关闭，仅建议本地开发使用", nil)
	}

	// Go 学习提示：这里是“依赖组装”位置。Store 负责数据访问，Handler 负责 HTTP，
	// 构造函数把具体实现显式传进去，业务代码无需自己创建数据库或外部客户端。
	employeeStore := auth.NewEmployeeStore(db)
	authTokenManager := auth.NewTokenManager(authConfig.JWTSecret)
	authService := auth.NewService(captchaManager, authTokenManager, employeeStore)
	authService.ConfigureLoginSecurity(loginLimiter, logService)
	authAuthorizationService := auth.NewAuthorizationService(authTokenManager, employeeStore)
	authHandler := auth.NewHandlerWithServices(authService, authAuthorizationService)
	rbacStore := rbac.NewStore(db)
	employeeService := rbac.NewEmployeeService(rbacStore)
	rbacHandler := rbac.NewHandler(employeeService)
	roleService := rbac.NewRoleService(rbacStore)
	roleHandler := rbac.NewRoleHandler(roleService)
	menuService := rbac.NewMenuService(rbacStore)
	menuHandler := rbac.NewMenuHandler(menuService)
	departmentService := rbac.NewDepartmentService(rbacStore)
	departmentHandler := rbac.NewDepartmentHandler(departmentService)
	tenantService := rbac.NewTenantService(rbacStore)
	tenantHandler := rbac.NewTenantHandler(tenantService)
	dictionaryStore := dictionary.NewStore(db)
	dictionaryService := dictionary.NewService(dictionaryStore)
	dictionaryHandler := dictionary.NewHandler(dictionaryService)
	userStore := user.NewStore(db)
	wechatConfig, err := user.LoadWechatConfig()
	if err != nil {
		return fmt.Errorf("读取微信小程序配置失败: %w", err)
	}
	wechatClient := user.NewWechatClient(wechatConfig)
	if developmentHTTPLogger != nil {
		wechatClient.ConfigureHTTPTransport(developmentHTTPLogger.WrapTransport(http.DefaultTransport))
	}
	miniappService := user.NewMiniappService(userStore, wechatClient, user.NewTokenManager(authConfig.JWTSecret))
	miniappHandler := user.NewHandler(miniappService)
	userAdminService := user.NewUserAdminService(userStore)
	userAdminHandler := user.NewAdminHandler(userAdminService)
	miniappAdminService := user.NewMiniappAdminService(userStore, wechatClient, wechatConfig.SecretConfigured())
	miniappAdminHandler := user.NewMiniappAdminHandler(miniappAdminService)
	mediaStore := media.NewStore(db)
	// Go 学习提示：变量声明为 Storage 接口后，可以在运行时放入 MinIO 实现或不可用兜底实现。
	// 业务 Handler 只依赖接口能力，不需要判断具体实现类型。
	var mediaStorage media.Storage = media.UnavailableStorage{}
	mediaConfig := media.LoadConfig()
	if mediaConfig.Complete() {
		configuredStorage, storageErr := media.NewMinioStorage(rootContext, mediaConfig)
		if storageErr != nil {
			logService.RecordEvent(rootContext, "error", "图片存储初始化失败，图片接口将暂时返回不可用", nil)
		} else {
			mediaStorage = configuredStorage
		}
	} else {
		logService.RecordEvent(rootContext, "warn", "图片存储配置不完整，图片接口将暂时返回不可用", nil)
	}
	miniappAdminService.ConfigureMiniappCodeCache(
		miniappCodeCacheAdapter{storage: mediaStorage},
		wechatConfig.MiniappCodeEnvVersion,
	)
	mediaService := media.NewService(mediaStore, mediaStorage)
	mediaHandler := media.NewHandler(mediaService)
	authService.ConfigureAvatarURLs(mediaService)
	miniappService.ConfigureAvatarURLs(mediaService)
	userAdminService.ConfigureAvatarURLs(mediaService)
	logQueryStore := logquery.NewStore(db)
	logQueryService := logquery.NewService(logQueryStore)
	logQueryHandler := logquery.NewHandler(logQueryService)
	startSystemLogCleanup(rootContext, logService, logConfig.Retention)
	router := httpapi.NewRouter(buildHTTPRoutes(httpRouteHandlers{
		requestRecorder: logStore,
		auditRecorder:   logService,
		developmentHTTP: developmentHTTPLogger,
		logConfig:       logConfig,
		readiness:       newReadinessCheck(sqlDB, captchaManager, loginLimiter),
		auth:            authHandler,
		dictionary:      dictionaryHandler,
		employee:        rbacHandler,
		role:            roleHandler,
		menu:            menuHandler,
		department:      departmentHandler,
		tenant:          tenantHandler,
		miniapp:         miniappHandler,
		userAdmin:       userAdminHandler,
		miniappAdmin:    miniappAdminHandler,
		media:           mediaHandler,
		logQuery:        logQueryHandler,
	}))
	trustedProxies := loadTrustedProxies()
	if err := router.SetTrustedProxies(trustedProxies); err != nil {
		return fmt.Errorf("读取可信代理配置失败: %w", err)
	}
	server := &http.Server{
		Addr:              serverAddress,
		Handler:           router,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	// Go 学习提示：ListenAndServe 会阻塞当前 goroutine，因此放到新的 goroutine 中运行；
	// 带一个缓冲位的 channel 用来把启动或运行错误送回主流程。
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.ListenAndServe()
	}()
	logService.RecordEvent(rootContext, "info", "HTTP 服务已启动", map[string]any{"address": serverAddress})

	// Go 学习提示：select 会等待最先发生的 channel 事件：服务器异常退出，或系统发出退出信号。
	select {
	case serverErr := <-serverErrors:
		if errors.Is(serverErr, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("启动 HTTP 服务失败: %w", serverErr)
	case <-rootContext.Done():
	}

	// 业务约束：优雅退出最多等待十秒，让正在处理的请求先结束，超时后返回错误。
	shutdownContext, cancel := context.WithTimeout(context.Background(), gracefulShutdownLimit)
	defer cancel()
	if err := server.Shutdown(shutdownContext); err != nil {
		return fmt.Errorf("HTTP 服务优雅退出失败: %w", err)
	}
	return nil
}

// startSystemLogCleanup 按配置保留时间启动受进程生命周期控制的系统日志清理任务。
func startSystemLogCleanup(ctx context.Context, service *logging.Service, retention time.Duration) {
	// Go 学习提示：后台 goroutine 与传入的 ctx 共用生命周期；服务退出时 ctx.Done() 被关闭，
	// select 随即结束循环，避免产生无法停止的后台任务。
	go func() {
		cleanup := func() {
			deleted, err := service.CleanupSystemLogs(ctx, time.Now().UTC().Add(-retention))
			if err != nil {
				logging.WriteEventOutput("error", "过期系统日志清理失败", "")
				return
			}
			if deleted > 0 {
				logging.WriteEventOutput("info", fmt.Sprintf("已清理 %d 条过期系统日志", deleted), "")
			}
		}
		cleanup()
		// Ticker 会按固定间隔向 C channel 发送时间；defer Stop 用于释放其内部计时资源。
		ticker := time.NewTicker(systemLogCleanupEvery)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cleanup()
			}
		}
	}()
}

// loadTrustedProxies 读取逗号分隔的可信代理 IP 或 CIDR；空值表示不信任任何代理。
func loadTrustedProxies() []string {
	raw := strings.TrimSpace(os.Getenv("TRUSTED_PROXIES"))
	if raw == "" {
		return nil
	}
	values := strings.Split(raw, ",")
	proxies := make([]string, 0, len(values))
	for _, value := range values {
		if proxy := strings.TrimSpace(value); proxy != "" {
			proxies = append(proxies, proxy)
		}
	}
	return proxies
}
