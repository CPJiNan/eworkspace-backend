package service

import (
	"log/slog"
	"math"
	"strconv"

	"eworkspace/internal/config"
	"eworkspace/internal/pkg/jwt"
	"eworkspace/internal/pkg/password"
	"eworkspace/internal/repository"
	"eworkspace/internal/worker"
)

type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

type PageMeta struct {
	Page       int   `json:"page"`
	Size       int   `json:"size"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"totalPages"`
}

func NewPageMeta(page, size int, total int64) PageMeta {
	p, sz := NormalizePage(page, size)
	pages := 0
	if sz > 0 {
		pages = int(math.Ceil(float64(total) / float64(sz)))
	}
	return PageMeta{Page: p, Size: sz, Total: total, TotalPages: pages}
}

func itoa(n int) string { return strconv.Itoa(n) }

func NormalizePage(page, size int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size <= 0 {
		size = DefaultPageSize
	}
	if size > MaxPageSize {
		size = MaxPageSize
	}
	return page, size
}

const (
	DefaultPageSize = 10
	MaxPageSize     = 100
)

type meta struct {
	SuperAdminStudentID string
}

type AuthResult struct {
	AccessToken  string  `json:"accessToken"`
	RefreshToken string  `json:"refreshToken"`
	TokenType    string  `json:"tokenType"`
	ExpiresIn    int64   `json:"expiresIn"`
	User         UserDTO `json:"user"`
}

type Service struct {
	Auth          AuthService
	User          UserService
	Project       ProjectService
	Member        MemberService
	Tag           TagService
	Notification  NotificationService
	OperationLog  OperationLogService
	DeadlineTimer *worker.DeadlineTimer
}

func New(cfg *config.Config, repos *repository.Repositories, log *slog.Logger) *Service {
	m := &meta{
		SuperAdminStudentID: cfg.Bootstrap.SuperAdminStudentID,
	}

	tokens := jwt.NewManager(cfg.JWT)
	hasher := password.Default

	notification := NewNotificationService(repos, log)
	operationLog := NewOperationLogService(repos, log)
	project := NewProjectService(m, repos, log, notification)

	return &Service{
		Auth:          NewAuthService(m, repos, log, tokens, hasher),
		User:          NewUserService(m, repos, log, hasher),
		Project:       project,
		Member:        NewMemberService(m, repos, log, notification, project),
		Tag:           NewTagService(m, repos, log),
		Notification:  notification,
		OperationLog:  operationLog,
		DeadlineTimer: worker.NewDeadlineTimer(repos, log),
	}
}
