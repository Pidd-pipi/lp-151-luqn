package router

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/gbtreehole/backend/internal/handler"
	"github.com/gbtreehole/backend/internal/middleware"
)

func New(
	logger *slog.Logger,
	auth *handler.AuthHandler,
	post *handler.PostHandler,
	comment *handler.CommentHandler,
	tag *handler.TagHandler,
	like *handler.LikeHandler,
	admin *handler.AdminHandler,
	identityMW *middleware.IdentityAuthMiddleware,
	sensitiveMW *middleware.SensitiveWordMiddleware,
) *gin.Engine {
	r := gin.New()
	r.Use(middleware.Recovery(logger), middleware.RequestID(), middleware.RequestLogger(logger), middleware.CORS())
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	// OpenAPI 文档，本地文件由容器内挂载/构建复制
	r.GET("/api/openapi.yaml", func(c *gin.Context) {
		c.File("api/openapi.yaml")
	})

	api := r.Group("/api/v1")
	api.Use(sensitiveMW.Detect())
	{
		authGroup := api.Group("/auth")
		{
			authGroup.POST("/identities", auth.CreateIdentity)
			authGroup.POST("/login", auth.LoginIdentity)
		}

		postGroup := api.Group("/posts")
		postGroup.Use(identityMW.OptionalAuth())
		{
			postGroup.POST("", identityMW.RequireAuth(), post.CreatePost)
			postGroup.GET("", post.ListPosts)
			postGroup.GET("/hot", post.HotPosts)
			postGroup.GET("/featured", post.FeaturedPosts)
			postGroup.GET("/:id", post.GetPost)
			postGroup.GET("/:id/comments", comment.ListComments)
		}

		commentGroup := api.Group("/comments")
		commentGroup.Use(identityMW.RequireAuth())
		{
			commentGroup.POST("", comment.CreateComment)
		}

		tagGroup := api.Group("/tags")
		{
			tagGroup.GET("", tag.ListTags)
			tagGroup.POST("", identityMW.RequireAuth(), tag.CreateTag)
		}

		likeGroup := api.Group("/likes")
		likeGroup.Use(identityMW.RequireAuth())
		{
			likeGroup.POST("/toggle", like.ToggleLike)
		}

		adminGroup := api.Group("/admin")
		adminGroup.Use(identityMW.RequireAuth())
		{
			adminGroup.GET("/reviews", admin.ListReviews)
			adminGroup.POST("/reviews/action", admin.ReviewItem)
			adminGroup.GET("/sensitive-words", admin.ListSensitiveWords)
			adminGroup.POST("/sensitive-words", admin.CreateSensitiveWord)
			adminGroup.DELETE("/sensitive-words/:id", admin.DeleteSensitiveWord)
			adminGroup.POST("/feature", admin.SetFeatured)
			adminGroup.GET("/tags", admin.ListAllTags)
		}
	}
	return r
}
