package router

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"eworkspace/internal/config"
	"eworkspace/internal/handler"
	"eworkspace/internal/service"
)

func New(cfg *config.Config, log *slog.Logger, svc *service.Service, h *handler.Handlers) *gin.Engine {
	switch cfg.Server.Mode {
	case gin.ReleaseMode:
		gin.SetMode(gin.ReleaseMode)
	case gin.TestMode:
		gin.SetMode(gin.TestMode)
	default:
		gin.SetMode(gin.DebugMode)
	}

	engine := gin.New()
	engine.Use(
		handler.RequestID(),
		handler.Security(),
		handler.AccessLog(log),
		handler.Recover(log),
	)
	engine.MaxMultipartMemory = 1 << 20

	engine.NoRoute(handler.NoRoute())

	engine.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, handler.Response{
			Code:    "OK",
			Message: "服务正常",
			Data: gin.H{
				"time": time.Now().Format(time.RFC3339),
				"mode": cfg.Server.Mode,
			},
		})
	})

	api := engine.Group("/api")

	auth := api.Group("/auth")
	{
		auth.POST("/login", h.Auth.Login)
		auth.POST("/refresh", h.Auth.Refresh)
	}

	authed := api.Group("", handler.Auth(svc))
	{
		authed.GET("/auth/me", h.Auth.Me)
		authed.POST("/auth/logout", h.Auth.Logout)
		authed.POST("/auth/password", h.Auth.ChangePassword)
	}

	locked := authed.Group("", handler.RequirePasswordChanged())
	{
		locked.GET("/users/me", h.User.Me)
		locked.PATCH("/users/me", h.User.UpdateMe)

		locked.GET("/projects", h.Project.List)
		locked.GET("/projects/:id", h.Project.Get)
		locked.GET("/my/tasks", h.Project.MyTasks)

		locked.POST("/assignments/:id/claim", h.Project.Claim)
		locked.DELETE("/assignments/:id/claim", h.Project.CancelClaim)
		locked.GET("/assignments/:id/members", h.Project.Members)

		locked.POST("/projects/:id/discussions", h.Project.AddDiscussion)
		locked.PATCH("/discussions/:id", h.Project.UpdateDiscussion)
		locked.DELETE("/discussions/:id", h.Project.DeleteDiscussion)

		locked.GET("/semesters", h.Tag.ListSemesters)
		locked.GET("/tags", h.Tag.ListTags)

		notifications := locked.Group("/notifications")
		{
			notifications.GET("", h.Notice.List)
			notifications.PATCH("/read-all", h.Notice.MarkAllRead)
			notifications.PATCH("/:id/read", h.Notice.MarkRead)
			notifications.POST("/batch-delete", h.Notice.BatchDelete)
			notifications.DELETE("/:id", h.Notice.Delete)
			notifications.DELETE("", h.Notice.Clear)
		}
	}

	admin := locked.Group("", handler.RequireAdmin())
	{
		admin.POST("/projects", h.Project.Create)
		admin.PATCH("/projects/:id", h.Project.Update)
		admin.PATCH("/projects/:id/status", h.Project.SetStatus)
		admin.DELETE("/projects/:id", h.Project.Delete)
		admin.POST("/assignments/:id/assign", h.Project.Assign)

		admin.POST("/semesters", h.Tag.CreateSemester)
		admin.PATCH("/semesters/:id", h.Tag.RenameSemester)
		admin.DELETE("/semesters/:id", h.Tag.DeleteSemester)
		admin.POST("/tags", h.Tag.CreateTag)
		admin.PATCH("/tags/:id", h.Tag.RenameTag)
		admin.DELETE("/tags/:id", h.Tag.DeleteTag)

		admin.GET("/admin/members", h.User.ListMembers)
		admin.POST("/admin/members", h.User.CreateAccounts)
		admin.GET("/admin/members/:studentId", h.User.GetMember)
		admin.PATCH("/admin/members/:studentId", h.User.UpdateMember)
		admin.PATCH("/admin/members/:studentId/ban", h.User.SetBanned)
		admin.POST("/admin/members/:studentId/password/reset", h.User.ResetPassword)
		admin.DELETE("/admin/members/:studentId", h.User.DeleteMember)

		admin.GET("/admin/logs", h.Log.List)
		admin.POST("/admin/logs/batch-delete", h.Log.BatchDelete)
		admin.POST("/admin/logs/clear", h.Log.ClearByRange)
	}

	super := locked.Group("/admin/admins", handler.RequireSuperAdmin())
	{
		super.POST("", h.User.CreateAccounts)
	}

	return engine
}
