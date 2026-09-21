package service

import (
	"context"
	"time"

	"eworkspace/internal/model"
	"eworkspace/internal/pkg/apperr"
	"eworkspace/internal/repository"
)

type NotificationDTO struct {
	ID           uint                   `json:"id"`
	Title        string                 `json:"title"`
	Content      string                 `json:"content"`
	Read         bool                   `json:"read"`
	Type         model.NotificationType `json:"type"`
	ProjectID    *uint                  `json:"projectId,omitempty"`
	AssignmentID *uint                  `json:"assignmentId,omitempty"`
	CreatedAt    time.Time              `json:"createdAt"`
}

type NotificationListDTO struct {
	Items       []NotificationDTO `json:"items"`
	Page        PageMeta          `json:"page"`
	UnreadCount int64             `json:"unreadCount"`
}

type NotificationService interface {
	NotifyProjectPublished(ctx context.Context, project *model.Project) error
	NotifyAssigned(ctx context.Context, target *model.User, project *model.Project, assignment *model.Assignment) error

	List(ctx context.Context, operator *model.User, page, size int) (*NotificationListDTO, error)
	MarkRead(ctx context.Context, operator *model.User, id uint) error
	MarkAllRead(ctx context.Context, operator *model.User) error
	Delete(ctx context.Context, operator *model.User, id uint) error
	DeleteMany(ctx context.Context, operator *model.User, ids []uint) (int64, error)
	Clear(ctx context.Context, operator *model.User) (int64, error)
}

type notificationService struct {
	repos *repository.Repositories
	log   Logger
}

func NewNotificationService(repos *repository.Repositories, log Logger) NotificationService {
	return &notificationService{repos: repos, log: log}
}

func (s *notificationService) NotifyProjectPublished(ctx context.Context, project *model.Project) error {
	users, _, err := s.repos.User.List(ctx, repository.UserFilter{Page: 1, Size: 1000})
	if err != nil {
		return err
	}

	deadline := project.Deadline.Format("2006-01-02 15:04")
	items := make([]model.Notification, 0, len(users))
	for i := range users {
		u := users[i]
		projectID := project.ID
		items = append(items, model.Notification{
			ReceiverID: u.StudentID,
			Type:       model.NotificationTypeProjectPublished,
			Title:      "新任务：" + project.Name,
			Content:    "截止时间 " + deadline,
			ProjectID:  &projectID,
		})
	}
	return s.repos.Notification.CreateBatch(ctx, items)
}

func (s *notificationService) NotifyAssigned(ctx context.Context, target *model.User, project *model.Project,
	assignment *model.Assignment) error {
	projectID := project.ID
	assignmentID := assignment.ID
	item := model.Notification{
		ReceiverID:   target.StudentID,
		Type:         model.NotificationTypeAssigned,
		Title:        "你被指派了分工：" + assignment.Name,
		Content:      "项目「" + project.Name + "」，截止时间 " + project.Deadline.Format("2006-01-02 15:04") + "。",
		ProjectID:    &projectID,
		AssignmentID: &assignmentID,
	}
	return s.repos.Notification.Create(ctx, &item)
}

func (s *notificationService) List(ctx context.Context, operator *model.User, page, size int) (*NotificationListDTO, error) {
	items, total, err := s.repos.Notification.ListByUser(ctx, operator.StudentID, page, size)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	unread, err := s.repos.Notification.UnreadCount(ctx, operator.StudentID)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	out := make([]NotificationDTO, 0, len(items))
	for i := range items {
		out = append(out, ToNotificationDTO(&items[i]))
	}
	return &NotificationListDTO{
		Items:       out,
		Page:        NewPageMeta(page, size, total),
		UnreadCount: unread,
	}, nil
}

func (s *notificationService) MarkRead(ctx context.Context, operator *model.User, id uint) error {
	item, err := s.repos.Notification.GetByID(ctx, id)
	if err != nil {
		return apperr.Internal(err)
	}
	if item == nil || item.ReceiverID != operator.StudentID {
		return apperr.NotFound("短信不存在")
	}
	if err := s.repos.Notification.MarkRead(ctx, id, operator.StudentID); err != nil {
		return apperr.Internal(err)
	}
	return nil
}

func (s *notificationService) MarkAllRead(ctx context.Context, operator *model.User) error {
	if err := s.repos.Notification.MarkAllRead(ctx, operator.StudentID); err != nil {
		return apperr.Internal(err)
	}
	return nil
}

func (s *notificationService) Delete(ctx context.Context, operator *model.User, id uint) error {
	item, err := s.repos.Notification.GetByID(ctx, id)
	if err != nil {
		return apperr.Internal(err)
	}
	if item == nil || item.ReceiverID != operator.StudentID {
		return apperr.NotFound("短信不存在")
	}
	if err := s.repos.Notification.Delete(ctx, id, operator.StudentID); err != nil {
		return apperr.Internal(err)
	}
	return nil
}

func (s *notificationService) DeleteMany(ctx context.Context, operator *model.User, ids []uint) (int64, error) {
	if len(ids) == 0 {
		return 0, apperr.Validation("请选择要删除的短信")
	}
	affected, err := s.repos.Notification.DeleteMany(ctx, ids, operator.StudentID)
	if err != nil {
		return 0, apperr.Internal(err)
	}
	return affected, nil
}

func (s *notificationService) Clear(ctx context.Context, operator *model.User) (int64, error) {
	affected, err := s.repos.Notification.Clear(ctx, operator.StudentID)
	if err != nil {
		return 0, apperr.Internal(err)
	}
	return affected, nil
}

func ToNotificationDTO(n *model.Notification) NotificationDTO {
	return NotificationDTO{
		ID:           n.ID,
		Title:        n.Title,
		Content:      n.Content,
		Read:         n.Read,
		Type:         n.Type,
		ProjectID:    n.ProjectID,
		AssignmentID: n.AssignmentID,
		CreatedAt:    n.CreatedAt,
	}
}
