package handler

import (
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"

	"eworkspace/internal/model"
	"eworkspace/internal/pkg/apperr"
	"eworkspace/internal/service"
)

const (
	ctxKeyUser      = "eworkspace.user"
	ctxKeyRequestID = "eworkspace.requestId"
)

type Handlers struct {
	Auth    *AuthHandler
	User    *UserHandler
	Project *ProjectHandler
	Tag     *TagHandler
	Notice  *NotificationHandler
	Log     *LogHandler
}

func New(svc *service.Service, log *slog.Logger) *Handlers {
	b := base{svc: svc, log: log}
	return &Handlers{
		Auth:    &AuthHandler{base: b},
		User:    &UserHandler{base: b},
		Project: &ProjectHandler{base: b},
		Tag:     &TagHandler{base: b},
		Notice:  &NotificationHandler{base: b},
		Log:     &LogHandler{base: b},
	}
}

type base struct {
	svc *service.Service
	log *slog.Logger
}

func currentUser(c *gin.Context) *model.User {
	if v, ok := c.Get(ctxKeyUser); ok {
		if u, ok := v.(*model.User); ok {
			return u
		}
	}
	return nil
}

func mustUser(c *gin.Context) (*model.User, error) {
	u := currentUser(c)
	if u == nil {
		return nil, apperr.ErrUnauthorized
	}
	return u, nil
}

func clientIP(c *gin.Context) string {
	if v := c.GetHeader("X-Forwarded-For"); v != "" {
		return v
	}
	if v := c.GetHeader("X-Real-IP"); v != "" {
		return v
	}
	return c.ClientIP()
}

func (b *base) writeLog(c *gin.Context, actor *model.User, op model.OperationType,
	targetType model.TargetType, targetID, targetName, detail string) {
	if actor == nil {
		return
	}
	b.svc.OperationLog.Write(c.Request.Context(), service.WriteLogInput{
		OperatorID:   actor.StudentID,
		OperatorName: actor.Name,
		Type:         op,
		TargetType:   targetType,
		TargetID:     targetID,
		TargetName:   targetName,
		Detail:       detail,
		IP:           clientIP(c),
		UserAgent:    c.Request.UserAgent(),
	})
}

func formatUint(v uint) string { return strconv.FormatUint(uint64(v), 10) }
