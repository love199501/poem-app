package router

import (
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"shijing/internal/handler"
)

// Setup 注册路由与中间件。
func Setup(h *handler.Handler) *gin.Engine {
	r := gin.Default()

	// 跨域：小程序请求不走 CORS，但便于浏览器联调开发，这里放开常用头。
	r.Use(cors.New(cors.Config{
		AllowAllOrigins: true,
		AllowMethods:    []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:    []string{"Origin", "Content-Type", "Content-Length", "X-User-Id"},
		MaxAge:          12 * time.Hour,
	}))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	api := r.Group("/api")
	{
		api.POST("/login", h.Login)

		api.GET("/poems", h.ListPoems)
		api.GET("/poems/facets", h.PoemFacets)
		api.GET("/poems/:id", h.GetPoem)

		api.GET("/poems/:id/comments", h.ListComments)
		api.POST("/poems/:id/comments", h.CreateComment)
		api.POST("/poems/:id/tts", h.SynthesizePoemAudio)
		api.POST("/poems/:id/illustration", h.GenerateIllustration)
		api.POST("/poems/:id/favorite", h.ToggleFavorite)
		api.POST("/poems/:id/like", h.TogglePoemLike)

		api.POST("/comments/:id/like", h.ToggleLike)
		api.POST("/comments/:id/pin", h.PinComment)

		// 当前用户：资料（含等级）、收藏列表、阅读时长上报、今日任务。
		api.GET("/users/me", h.GetMe)
		api.GET("/users/me/favorites", h.ListFavorites)
		api.GET("/users/me/comments", h.ListMyComments)
		api.GET("/users/me/tasks", h.DailyTasks)
		api.POST("/users/me/read", h.ReportRead)
		api.POST("/users/me/member/claim", h.ClaimMember)
	}

	// 管理员后台接口：登录开放，其余需管理员鉴权。
	admin := r.Group("/api/admin")
	{
		admin.POST("/login", h.AdminLogin)

		auth := admin.Group("")
		auth.Use(h.AdminAuth())
		auth.GET("/poems", h.AdminListPoems)
		auth.POST("/poems/generate", h.AdminGenerate)
		auth.POST("/poems", h.AdminCreatePoem)
		auth.PUT("/poems/:id", h.AdminUpdatePoem)
		auth.POST("/poems/:id/publish", h.AdminPublish)
		auth.POST("/poems/:id/unpublish", h.AdminUnpublish)
		auth.DELETE("/poems/:id", h.AdminDeletePoem)
	}

	// 网页后台页面（Go 直接托管）。
	r.StaticFile("/admin", "./web/admin.html")
	// 合成的朗诵音频静态目录。
	r.Static("/audio", "./web/audio")
	// AI 生成的插画静态目录。
	r.Static("/images", "./web/images")

	return r
}
