package handler

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"eworkspace/internal/model"
	"eworkspace/internal/pkg/apperr"
	"eworkspace/internal/service"
)

func RequestID() gin.HandlerFunc {
	var seq uint64
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-Id")
		if id == "" {
			seq++
			id = time.Now().Format("20060102150405") + "-" + formatUint(uint(seq))
		}
		c.Set(ctxKeyRequestID, id)
		c.Header("X-Request-Id", id)
		c.Next()
	}
}

func Security() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "SAMEORIGIN")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'")
		c.Next()
	}
}

func AccessLog(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		level := slog.LevelInfo
		if c.Writer.Status() >= 500 {
			level = slog.LevelError
		} else if c.Writer.Status() >= 400 {
			level = slog.LevelWarn
		}
		log.Log(c.Request.Context(), level, "http",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"cost", time.Since(start).String(),
			"ip", clientIP(c),
			"requestId", c.GetString(ctxKeyRequestID),
		)
	}
}

func Recover(log *slog.Logger) gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(nil, func(c *gin.Context, recovered any) {
		log.Error("请求处理发生 panic",
			"path", c.Request.URL.Path,
			"err", recovered,
			"requestId", c.GetString(ctxKeyRequestID),
		)
		c.AbortWithStatusJSON(http.StatusInternalServerError, Response{
			Code:    apperr.ErrInternal.Code,
			Message: apperr.ErrInternal.Message,
		})
	})
}

func Auth(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := bearerToken(c)
		if token == "" {
			Fail(c, apperr.ErrUnauthorized)
			return
		}
		user, err := svc.Auth.Authenticate(c.Request.Context(), token)
		if err != nil {
			Fail(c, err)
			return
		}
		c.Set(ctxKeyUser, user)
		c.Next()
	}
}

func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		u := currentUser(c)
		if u == nil {
			Fail(c, apperr.ErrUnauthorized)
			return
		}
		if !u.Role.CanManage() {
			Fail(c, apperr.ErrForbidden)
			return
		}
		c.Next()
	}
}

func RequireSuperAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		u := currentUser(c)
		if u == nil {
			Fail(c, apperr.ErrUnauthorized)
			return
		}
		if u.Role != model.RoleSuperAdmin {
			Fail(c, apperr.ErrSuperAdminOnly)
			return
		}
		c.Next()
	}
}

func RequirePasswordChanged() gin.HandlerFunc {
	return func(c *gin.Context) {
		u := currentUser(c)
		if u != nil && u.MustChangePassword {
			c.AbortWithStatusJSON(http.StatusForbidden, Response{
				Code:    "PASSWORD_CHANGE_REQUIRED",
				Message: "首次登录请先修改初始密码",
			})
			return
		}
		c.Next()
	}
}

func NoRoute() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.AbortWithStatusJSON(http.StatusNotFound, Response{
			Code:    "NOT_FOUND",
			Message: "接口不存在",
		})
	}
}

func bearerToken(c *gin.Context) string {
	header := c.GetHeader("Authorization")
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
