package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gbtreehole/backend/internal/config"
	"github.com/gbtreehole/backend/internal/database"
	"github.com/gbtreehole/backend/internal/handler"
	"github.com/gbtreehole/backend/internal/middleware"
	"github.com/gbtreehole/backend/internal/repository"
	"github.com/gbtreehole/backend/internal/router"
	"github.com/gbtreehole/backend/internal/service"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}

	db, err := database.NewMySQL(cfg)
	if err != nil {
		logger.Error("connect mysql", "error", err)
		os.Exit(1)
	}
	if err := database.Migrate(db); err != nil {
		logger.Error("migrate database", "error", err)
		os.Exit(1)
	}
	if err := database.Seed(db); err != nil {
		logger.Error("seed database", "error", err)
		os.Exit(1)
	}

	redisClient, err := database.NewRedis(cfg)
	if err != nil {
		logger.Error("connect redis", "error", err)
		os.Exit(1)
	}

	// Repositories
	identityRepo := repository.NewIdentityRepository(db)
	postRepo := repository.NewPostRepository(db)
	commentRepo := repository.NewCommentRepository(db)
	tagRepo := repository.NewTagRepository(db)
	likeRepo := repository.NewLikeRepository(db)
	sensitiveRepo := repository.NewSensitiveWordRepository(db)
	reviewRepo := repository.NewReviewQueueRepository(db)

	// Services
	tokenService := service.NewTokenService(cfg.JWTSecret, cfg.JWTExpireMin)
	identityService := service.NewIdentityService(identityRepo, tokenService, logger)
	tagService := service.NewTagService(tagRepo)
	sensitiveService := service.NewSensitiveWordService(sensitiveRepo)
	reviewService := service.NewReviewService(reviewRepo, postRepo, commentRepo, logger)
	postService := service.NewPostService(postRepo, tagService, sensitiveService, reviewService, logger)
	commentService := service.NewCommentService(commentRepo, postRepo, sensitiveService, reviewService, logger)
	likeService := service.NewLikeService(likeRepo, postRepo, commentRepo, logger)
	_ = service.NewHeatService(postRepo, logger)
	_ = service.NewCacheService(redisClient)

	// Handlers
	authHandler := handler.NewAuthHandler(identityService, logger)
	postHandler := handler.NewPostHandler(postService, likeService, logger)
	commentHandler := handler.NewCommentHandler(commentService, likeService, logger)
	tagHandler := handler.NewTagHandler(tagService, logger)
	likeHandler := handler.NewLikeHandler(likeService, logger)
	adminHandler := handler.NewAdminHandler(reviewService, postService, sensitiveService, tagService, logger)

	// Middleware
	identityMW := middleware.NewIdentityAuthMiddleware(tokenService)
	sensitiveMW := middleware.NewSensitiveWordMiddleware(sensitiveService, logger)

	engine := router.New(logger, authHandler, postHandler, commentHandler, tagHandler, likeHandler, adminHandler, identityMW, sensitiveMW)

	srv := &http.Server{
		Addr:              ":" + cfg.BackendPort,
		Handler:           engine,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("server starting", "port", cfg.BackendPort)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server listen", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("shutting down server")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("server shutdown", "error", err)
	}
}
