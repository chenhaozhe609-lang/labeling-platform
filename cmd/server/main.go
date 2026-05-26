// Command server 是标注平台后端入口。
//
// 用法：
//
//	server                                      启动 HTTP 服务
//	server migrate up|down                      执行/回滚数据库迁移
//	server createuser <用户名> <密码> <角色>       在默认组织(#1)创建用户（角色：annotator|reviewer|admin）
//	server createsuperadmin <邮箱> <密码>         创建平台超管（跨组织、org_id=NULL）
//	server seed                                 生成 demo 数据（source 表 + dataset + tasks）
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/chenhaozhe609-lang/labeling-platform/internal/config"
	"github.com/chenhaozhe609-lang/labeling-platform/internal/domain"
	"github.com/chenhaozhe609-lang/labeling-platform/internal/handler"
	"github.com/chenhaozhe609-lang/labeling-platform/internal/job"
	"github.com/chenhaozhe609-lang/labeling-platform/internal/middleware"
	"github.com/chenhaozhe609-lang/labeling-platform/internal/platform/db"
	jwtpkg "github.com/chenhaozhe609-lang/labeling-platform/internal/platform/jwt"
	"github.com/chenhaozhe609-lang/labeling-platform/internal/platform/pgrestore"
	"github.com/chenhaozhe609-lang/labeling-platform/internal/repository/source"
	"github.com/chenhaozhe609-lang/labeling-platform/internal/repository/store"
	"github.com/chenhaozhe609-lang/labeling-platform/internal/service"
)

const migrationsSource = "file://migrations"

func main() {
	cfg, err := config.Load()
	if err != nil {
		fatal("加载配置失败", err)
	}

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "migrate":
			runMigrate(cfg)
			return
		case "createuser":
			runCreateUser(cfg)
			return
		case "createsuperadmin":
			runCreateSuperadmin(cfg)
			return
		case "seed":
			runSeed(cfg)
			return
		}
	}
	runServer(cfg)
}

func runMigrate(cfg config.Config) {
	down := len(os.Args) > 2 && os.Args[2] == "down"
	if err := db.Migrate(cfg.DatabaseURL, migrationsSource, down); err != nil {
		fatal("迁移失败", err)
	}
	slog.Info("迁移完成", "down", down)
}

// defaultOrgID 是回填迁移建立的默认组织（migrations/002）。CLI createuser 默认建到此组织。
const defaultOrgID int64 = 1

func runCreateUser(cfg config.Config) {
	if len(os.Args) < 5 {
		fmt.Fprintln(os.Stderr, "用法: server createuser <用户名> <密码> <角色>")
		os.Exit(2)
	}
	username, password := os.Args[2], os.Args[3]
	role := domain.Role(os.Args[4])
	if !role.Valid() {
		fatal("非法角色（annotator|reviewer|admin）", fmt.Errorf("got %q", role))
	}
	if len(password) < 8 {
		fatal("密码至少 8 位", fmt.Errorf("len=%d", len(password)))
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fatal("密码加密失败", err)
	}
	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		fatal("连接数据库失败", err)
	}
	defer pool.Close()
	orgID := defaultOrgID
	u, err := store.New(pool).CreateUser(ctx, store.NewUser{
		Username:     username,
		Email:        strings.ToLower(username) + "@local", // 占位邮箱，对齐回填约定；上线前应改真实邮箱
		PasswordHash: string(hash),
		Role:         role,
		OrgID:        &orgID,
	})
	if err != nil {
		fatal("创建用户失败", err)
	}
	fmt.Printf("已创建用户 #%d %s <%s> (%s) @org %d\n", u.ID, u.Username, u.Email, u.Role, defaultOrgID)
}

// runCreateSuperadmin 创建平台超管（org_id=NULL、is_superadmin=true），用邮箱登录。
func runCreateSuperadmin(cfg config.Config) {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "用法: server createsuperadmin <邮箱> <密码>")
		os.Exit(2)
	}
	email, password := os.Args[2], os.Args[3]
	if len(password) < 8 {
		fatal("密码至少 8 位", fmt.Errorf("len=%d", len(password)))
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fatal("密码加密失败", err)
	}
	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		fatal("连接数据库失败", err)
	}
	defer pool.Close()
	u, err := store.New(pool).CreateUser(ctx, store.NewUser{
		Username:     email,
		Email:        email,
		PasswordHash: string(hash),
		Role:         domain.RoleAdmin,
		IsSuperadmin: true, // OrgID 留 nil
	})
	if err != nil {
		fatal("创建超管失败", err)
	}
	fmt.Printf("已创建平台超管 #%d <%s>\n", u.ID, u.Email)
}

func runServer(cfg config.Config) {
	ctx := context.Background()
	metaPool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		fatal("连接 meta-db 失败", err)
	}
	defer metaPool.Close()

	sourcePool, err := db.NewPool(ctx, cfg.SourceDatabaseURL)
	if err != nil {
		fatal("连接 source-db 失败", err)
	}
	defer sourcePool.Close()

	srcAdminPool, err := db.NewPool(ctx, cfg.SourceAdminURL)
	if err != nil {
		fatal("连接 source-db(admin) 失败", err)
	}
	defer srcAdminPool.Close()

	st := store.New(metaPool)
	src := source.New(sourcePool)
	jm := jwtpkg.NewManager(cfg.JWTSecret, cfg.AccessTTL, cfg.RefreshTTL)
	restorer := pgrestore.New(pgrestore.Config{
		Mode: cfg.SandboxMode, Container: cfg.SandboxContainer, DB: cfg.SandboxDB,
		Host: cfg.SandboxHost, Port: cfg.SandboxPort,
		User: cfg.SandboxUser, Password: cfg.SandboxPassword, Timeout: cfg.RestoreTimeout,
	})
	loginLimiter := middleware.NewLoginLimiter()
	authH := handler.NewAuthHandler(st, jm, loginLimiter)

	// LLM 预填：配了 key 用真模型（DeepSeek 等 OpenAI 兼容），否则回退占位 stub（PRD §24.5）。
	var prefiller service.Prefiller = service.StubPrefiller{}
	if cfg.LLMAPIKey != "" {
		prefiller = service.NewLLMPrefiller(service.LLMConfig{
			BaseURL: cfg.LLMBaseURL, APIKey: cfg.LLMAPIKey, Model: cfg.LLMModel, Timeout: cfg.LLMTimeout,
		})
		slog.Info("LLM 预填已启用", "model", cfg.LLMModel, "base", cfg.LLMBaseURL)
	} else {
		slog.Info("LLM 预填未配置，使用占位 StubPrefiller（设 LLM_API_KEY 启用）")
	}
	taskH := handler.NewTaskHandler(st, src, prefiller, cfg.LeaseMinutes)
	datasetH := handler.NewDatasetHandler(st, srcAdminPool, restorer, cfg.UploadDir, cfg.UploadMaxBytes, "labeling_reader")
	reviewH := handler.NewReviewHandler(st, src)
	exportH := handler.NewExportHandler(st, src)
	userH := handler.NewUserHandler(st)
	inviteH := handler.NewInviteHandler(st, 7*24*time.Hour)
	platformH := handler.NewPlatformHandler(st)

	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// lease reaper
	go job.NewReaper(st, time.Minute).Run(rootCtx)

	if cfg.Env == "prod" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.MaxMultipartMemory = 16 << 20 // 16MB 内存上限，更大转临时盘
	r.Use(middleware.Recover(), middleware.Logger(), middleware.CORS(cfg.CORSOrigin))
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	api := r.Group("/api")
	api.Use(middleware.RateLimit(50, 100))
	{
		auth := api.Group("/auth")
		auth.POST("/signup", authH.Signup)
		auth.POST("/login", authH.Login)
		auth.POST("/refresh", authH.Refresh)
		auth.POST("/accept-invite", authH.AcceptInvite)

		authed := api.Group("")
		authed.Use(middleware.RequireAuth(jm))
		authed.GET("/me", authH.Me)
		authed.POST("/auth/logout-all", authH.LogoutAll)
		authed.GET("/me/tasks", taskH.MyTasks)

		authed.GET("/datasets", taskH.ListDatasets)
		authed.GET("/datasets/:id", datasetH.Detail)
		authed.POST("/datasets", middleware.RequireRole(domain.RoleAdmin), datasetH.Upload)
		authed.POST("/datasets/:id/sync", middleware.RequireRole(domain.RoleAdmin), datasetH.Sync)
		authed.POST("/datasets/:id/generate-tasks", middleware.RequireRole(domain.RoleAdmin), datasetH.GenerateTasks)
		authed.POST("/datasets/:id/pause", middleware.RequireRole(domain.RoleAdmin), datasetH.Pause)
		authed.POST("/datasets/:id/resume", middleware.RequireRole(domain.RoleAdmin), datasetH.Resume)
		authed.PUT("/datasets/:id/form-schema", middleware.RequireRole(domain.RoleAdmin), datasetH.UpdateFormSchema)
		authed.GET("/datasets/:id/export", middleware.RequireRole(domain.RoleAdmin, domain.RoleReviewer), exportH.Export)
		authed.GET("/admin/dashboard", middleware.RequireRole(domain.RoleAdmin), datasetH.Dashboard)

		adminUsers := authed.Group("/admin/users", middleware.RequireRole(domain.RoleAdmin))
		adminUsers.GET("", userH.List)
		adminUsers.POST("", userH.Create)
		adminUsers.PATCH("/:id", userH.Update)
		adminUsers.DELETE("/:id", userH.Delete)

		adminInvites := authed.Group("/admin/invites", middleware.RequireRole(domain.RoleAdmin))
		adminInvites.GET("", inviteH.List)
		adminInvites.POST("", inviteH.Create)
		adminInvites.DELETE("/:id", inviteH.Delete)

		platform := authed.Group("/platform", middleware.RequireSuperadmin())
		platform.GET("/orgs", platformH.ListOrgs)

		tasks := authed.Group("/tasks")
		tasks.POST("/claim", taskH.Claim)
		tasks.GET("/:id", taskH.Get)
		tasks.POST("/:id/heartbeat", taskH.Heartbeat)
		tasks.POST("/:id/submit", taskH.Submit)
		tasks.POST("/:id/release", taskH.Release)

		reviews := authed.Group("/reviews")
		reviews.Use(middleware.RequireRole(domain.RoleReviewer, domain.RoleAdmin))
		reviews.GET("/queue", reviewH.Queue)
		reviews.POST("/:id/decision", reviewH.Decision)
		reviews.POST("/:id/edit", reviewH.Edit)
	}

	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: r}
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fatal("HTTP server 异常", err)
		}
	}()
	slog.Info("服务已启动", "addr", cfg.HTTPAddr, "env", cfg.Env)

	<-rootCtx.Done()
	slog.Info("正在优雅关闭…")
	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		slog.Error("关闭超时", "error", err)
	}
}

func fatal(msg string, err error) {
	slog.Error(msg, "error", err)
	os.Exit(1)
}
