package service

import (
	"context"
	"time"

	"eworkspace/internal/model"
	"eworkspace/internal/pkg/apperr"
	"eworkspace/internal/repository"
)

type OperationLogDTO struct {
	ID           uint                `json:"id"`
	OperatorID   string              `json:"operatorId"`
	OperatorName string              `json:"operatorName"`
	Type         model.OperationType `json:"type"`
	TargetType   model.TargetType    `json:"targetType"`
	TargetID     string              `json:"targetId"`
	TargetName   string              `json:"targetName"`
	Detail       string              `json:"detail"`
	IP           string              `json:"ip"`
	UserAgent    string              `json:"userAgent"`
	CreatedAt    time.Time           `json:"createdAt"`
}

type WriteLogInput struct {
	OperatorID   string
	OperatorName string
	Type         model.OperationType
	TargetType   model.TargetType
	TargetID     string
	TargetName   string
	Detail       string
	IP           string
	UserAgent    string
}

type ListLogsInput struct {
	OperatorID string
	Type       *model.OperationType
	TargetType *model.TargetType
	TargetID   string
	From       *time.Time
	To         *time.Time
	Page       int
	Size       int
}

type OperationLogService interface {
	Write(ctx context.Context, in WriteLogInput)
	List(ctx context.Context, operator *model.User, in ListLogsInput) ([]OperationLogDTO, PageMeta, error)
	DeleteMany(ctx context.Context, operator *model.User, ids []uint) (int64, error)
	DeleteByRange(ctx context.Context, operator *model.User, from, to *time.Time) (int64, error)
}

type operationLogService struct {
	repos *repository.Repositories
	log   Logger
}

func NewOperationLogService(repos *repository.Repositories, log Logger) OperationLogService {
	return &operationLogService{repos: repos, log: log}
}

func (s *operationLogService) Write(ctx context.Context, in WriteLogInput) {
	if !in.Type.IsLoggable() {
		s.log.Debug("忽略不在日志范围内的操作类型", "type", string(in.Type))
		return
	}

	entry := &model.OperationLog{
		OperatorID:   in.OperatorID,
		OperatorName: in.OperatorName,
		Type:         in.Type,
		TargetType:   in.TargetType,
		TargetID:     in.TargetID,
		TargetName:   in.TargetName,
		Detail:       in.Detail,
		IP:           in.IP,
		UserAgent:    in.UserAgent,
	}
	if err := s.repos.OperationLog.Create(ctx, entry); err != nil {
		s.log.Warn("写入操作日志失败", "type", in.Type, "err", err)
	}
}

func (s *operationLogService) List(ctx context.Context, operator *model.User, in ListLogsInput) ([]OperationLogDTO, PageMeta, error) {
	if !operator.Role.CanManage() {
		return nil, PageMeta{}, apperr.ErrForbidden
	}
	logs, total, err := s.repos.OperationLog.List(ctx, repository.OpLogFilter{
		OperatorID: in.OperatorID,
		Type:       in.Type,
		TargetType: in.TargetType,
		TargetID:   in.TargetID,
		From:       in.From,
		To:         in.To,
		Page:       in.Page,
		Size:       in.Size,
	})
	if err != nil {
		return nil, PageMeta{}, apperr.Internal(err)
	}

	out := make([]OperationLogDTO, 0, len(logs))
	for _, l := range logs {
		out = append(out, OperationLogDTO{
			ID:           l.ID,
			OperatorID:   l.OperatorID,
			OperatorName: l.OperatorName,
			Type:         l.Type,
			TargetType:   l.TargetType,
			TargetID:     l.TargetID,
			TargetName:   l.TargetName,
			Detail:       l.Detail,
			IP:           l.IP,
			UserAgent:    l.UserAgent,
			CreatedAt:    l.CreatedAt,
		})
	}
	return out, NewPageMeta(in.Page, in.Size, total), nil
}

func (s *operationLogService) DeleteMany(ctx context.Context, operator *model.User, ids []uint) (int64, error) {
	if !operator.Role.CanManage() {
		return 0, apperr.ErrForbidden
	}
	if len(ids) == 0 {
		return 0, apperr.Validation("请选择要删除的日志")
	}
	affected, err := s.repos.OperationLog.DeleteMany(ctx, ids)
	if err != nil {
		return 0, apperr.Internal(err)
	}
	return affected, nil
}

func (s *operationLogService) DeleteByRange(ctx context.Context, operator *model.User, from, to *time.Time) (int64, error) {
	if !operator.Role.CanManage() {
		return 0, apperr.ErrForbidden
	}
	if from == nil && to == nil {
		return 0, apperr.Validation("请指定要清理的时间范围")
	}
	affected, err := s.repos.OperationLog.DeleteByRange(ctx, from, to)
	if err != nil {
		return 0, apperr.Internal(err)
	}
	return affected, nil
}
