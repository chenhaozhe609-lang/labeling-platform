// Command server 是标注平台后端入口。
//
// 用法：
//
//	server                                 启动 HTTP 服务
//	server migrate up|down                 执行/回滚数据库迁移
//	server createuser <用户名> <密码> <角色>  创建用户（角色：annotator|reviewer|admin）
//	server seed                            生成 demo 数据（source 表 + dataset + tasks）
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
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
	u, err := store.New(pool).CreateUser(ctx, username, string(hash), role)
	if err != nil {
		fatal("创建用户失败", err)
	}
	fmt.Printf("已创建用户 #%d %s (%s)\n", u.ID, u.Username, u.Role)
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
		User: cfg.SandboxUser, Password: cfg.SandboxPassword, Timeout: cfg.RestoreTimeout,
	})
	authH := handler.NewAuthHandler(st, jm)
	taskH := handler.NewTaskHandler(st, src, cfg.LeaseMinutes)
	datasetH := handler.NewDatasetHandler(st, srcAdminPool, restorer, cfg.UploadDir, cfg.UploadMaxBytes, "labeling_reader")

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
		auth.POST("/login", authH.Login)
		auth.POST("/refresh", authH.Refresh)

		authed := api.Group("")
		authed.Use(middleware.RequireAuth(jm))
		authed.GET("/me", authH.Me)

		authed.GET("/datasets", taskH.ListDatasets)
		authed.GET("/datasets/:id", datasetH.Detail)
		authed.POST("/datasets", middleware.RequireRole(domain.RoleAdmin), datasetH.Upload)
		authed.PUT("/datasets/:id/form-schema", middleware.RequireRole(domain.RoleAdmin), datasetH.UpdateFormSchema)

		tasks := authed.Group("/tasks")
		tasks.POST("/claim", taskH.Claim)
		tasks.GET("/:id", taskH.Get)
		tasks.POST("/:id/heartbeat", taskH.Heartbeat)
		tasks.POST("/:id/submit", taskH.Submit)
		tasks.POST("/:id/release", taskH.Release)
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
