package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"eworkspace/internal/model"
)

func likePattern(keyword string) string {
	kw := strings.ToLower(strings.TrimSpace(keyword))
	if kw == "" {
		return ""
	}
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + replacer.Replace(kw) + "%"
}

type NotificationRepo interface {
	CreateBatch(ctx context.Context, list []model.Notification) error
	Create(ctx context.Context, n *model.Notification) error
	ListByUser(ctx context.Context, studentID string, page, size int) ([]model.Notification, int64, error)
	UnreadCount(ctx context.Context, studentID string) (int64, error)
	GetByID(ctx context.Context, id uint) (*model.Notification, error)
	MarkRead(ctx context.Context, id uint, studentID string) error
	MarkAllRead(ctx context.Context, studentID string) error
	Delete(ctx context.Context, id uint, studentID string) error
	DeleteMany(ctx context.Context, ids []uint, studentID string) (int64, error)
	Clear(ctx context.Context, studentID string) (int64, error)
}

type notificationRepo struct{ db *gorm.DB }

func NewNotificationRepo(db *gorm.DB) NotificationRepo { return &notificationRepo{db: db} }

func (r *notificationRepo) Create(ctx context.Context, n *model.Notification) error {
	return wrapErr("创建站内短信", r.db.WithContext(ctx).Create(n).Error)
}

func (r *notificationRepo) CreateBatch(ctx context.Context, list []model.Notification) error {
	if len(list) == 0 {
		return nil
	}
	return wrapErr("批量创建站内短信", r.db.WithContext(ctx).CreateInBatches(list, 100).Error)
}

func (r *notificationRepo) ListByUser(ctx context.Context, studentID string, page, size int) ([]model.Notification, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.Notification{}).Where("receiver_id = ?", studentID)

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	list := make([]model.Notification, 0)
	if err := q.Order("created_at DESC, id DESC").
		Offset(offset(page, size)).Limit(limit(size)).
		Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (r *notificationRepo) UnreadCount(ctx context.Context, studentID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Notification{}).
		Where("receiver_id = ? AND read = ?", studentID, false).Count(&count).Error
	return count, err
}

func (r *notificationRepo) GetByID(ctx context.Context, id uint) (*model.Notification, error) {
	var n model.Notification
	if err := r.db.WithContext(ctx).First(&n, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &n, nil
}

func (r *notificationRepo) MarkRead(ctx context.Context, id uint, studentID string) error {
	return r.db.WithContext(ctx).Model(&model.Notification{}).
		Where("id = ? AND receiver_id = ?", id, studentID).
		Update("read", true).Error
}

func (r *notificationRepo) MarkAllRead(ctx context.Context, studentID string) error {
	return r.db.WithContext(ctx).Model(&model.Notification{}).
		Where("receiver_id = ? AND read = ?", studentID, false).
		Update("read", true).Error
}

func (r *notificationRepo) Delete(ctx context.Context, id uint, studentID string) error {
	return r.db.WithContext(ctx).
		Where("id = ? AND receiver_id = ?", id, studentID).
		Delete(&model.Notification{}).Error
}

func (r *notificationRepo) DeleteMany(ctx context.Context, ids []uint, studentID string) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	res := r.db.WithContext(ctx).
		Where("id IN ? AND receiver_id = ?", ids, studentID).
		Delete(&model.Notification{})
	return res.RowsAffected, res.Error
}

func (r *notificationRepo) Clear(ctx context.Context, studentID string) (int64, error) {
	res := r.db.WithContext(ctx).Where("receiver_id = ?", studentID).Delete(&model.Notification{})
	return res.RowsAffected, res.Error
}

type OpLogFilter struct {
	OperatorID string
	Type       *model.OperationType
	TargetType *model.TargetType
	TargetID   string
	From       *time.Time
	To         *time.Time
	Page       int
	Size       int
}

type OperationLogRepo interface {
	Create(ctx context.Context, log *model.OperationLog) error
	List(ctx context.Context, f OpLogFilter) ([]model.OperationLog, int64, error)
	DeleteMany(ctx context.Context, ids []uint) (int64, error)
	DeleteByRange(ctx context.Context, from, to *time.Time) (int64, error)
}

type operationLogRepo struct{ db *gorm.DB }

func NewOperationLogRepo(db *gorm.DB) OperationLogRepo { return &operationLogRepo{db: db} }

func (r *operationLogRepo) Create(ctx context.Context, log *model.OperationLog) error {
	return wrapErr("写入操作日志", r.db.WithContext(ctx).Create(log).Error)
}

func (r *operationLogRepo) List(ctx context.Context, f OpLogFilter) ([]model.OperationLog, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.OperationLog{})

	q = q.Where("type IN ?", model.LoggableOperations())

	if f.OperatorID != "" {
		q = q.Where("operator_id = ?", f.OperatorID)
	}
	if f.Type != nil {
		q = q.Where("type = ?", *f.Type)
	}
	if f.TargetType != nil {
		q = q.Where("target_type = ?", *f.TargetType)
	}
	if f.TargetID != "" {
		q = q.Where("target_id = ?", f.TargetID)
	}
	if f.From != nil {
		q = q.Where("created_at >= ?", *f.From)
	}
	if f.To != nil {
		q = q.Where("created_at <= ?", *f.To)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	list := make([]model.OperationLog, 0)
	if err := q.Order("created_at DESC, id DESC").
		Offset(offset(f.Page, f.Size)).Limit(limit(f.Size)).
		Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (r *operationLogRepo) DeleteMany(ctx context.Context, ids []uint) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	res := r.db.WithContext(ctx).Where("id IN ?", ids).Delete(&model.OperationLog{})
	return res.RowsAffected, res.Error
}

func (r *operationLogRepo) DeleteByRange(ctx context.Context, from, to *time.Time) (int64, error) {
	q := r.db.WithContext(ctx).Model(&model.OperationLog{})
	if from != nil {
		q = q.Where("created_at >= ?", *from)
	}
	if to != nil {
		q = q.Where("created_at <= ?", *to)
	}
	res := q.Delete(&model.OperationLog{})
	return res.RowsAffected, res.Error
}

type RefreshTokenRepo interface {
	Create(ctx context.Context, t *model.RefreshToken) error
	GetByJTI(ctx context.Context, jti string) (*model.RefreshToken, error)
	Revoke(ctx context.Context, jti string) error
	RevokeByUser(ctx context.Context, studentID string) error
	DeleteExpired(ctx context.Context, before time.Time) (int64, error)
}

type refreshTokenRepo struct{ db *gorm.DB }

func NewRefreshTokenRepo(db *gorm.DB) RefreshTokenRepo { return &refreshTokenRepo{db: db} }

func (r *refreshTokenRepo) Create(ctx context.Context, t *model.RefreshToken) error {
	return wrapErr("保存 Refresh Token", r.db.WithContext(ctx).Create(t).Error)
}

func (r *refreshTokenRepo) GetByJTI(ctx context.Context, jti string) (*model.RefreshToken, error) {
	var t model.RefreshToken
	err := r.db.WithContext(ctx).
		Where("jti = ? AND revoked = ? AND expires_at > ?", jti, false, time.Now()).
		First(&t).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *refreshTokenRepo) Revoke(ctx context.Context, jti string) error {
	return r.db.WithContext(ctx).Model(&model.RefreshToken{}).
		Where("jti = ?", jti).Update("revoked", true).Error
}

func (r *refreshTokenRepo) RevokeByUser(ctx context.Context, studentID string) error {
	return r.db.WithContext(ctx).Model(&model.RefreshToken{}).
		Where("student_id = ? AND revoked = ?", studentID, false).
		Update("revoked", true).Error
}

func (r *refreshTokenRepo) DeleteExpired(ctx context.Context, before time.Time) (int64, error) {
	res := r.db.WithContext(ctx).
		Where("expires_at < ?", before).
		Delete(&model.RefreshToken{})
	return res.RowsAffected, res.Error
}
