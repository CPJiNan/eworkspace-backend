package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"eworkspace/internal/model"
)

type UserFilter struct {
	Keyword string
	Role    *model.Role
	Banned  *bool
	Page    int
	Size    int
}

type UserRepo interface {
	Create(ctx context.Context, u *model.User) error
	GetByID(ctx context.Context, studentID string) (*model.User, error)
	GetByIDIncludingDeleted(ctx context.Context, studentID string) (*model.User, error)
	Exists(ctx context.Context, studentID string) (bool, error)
	Update(ctx context.Context, u *model.User) error
	UpdateFields(ctx context.Context, studentID string, fields map[string]any) error
	Restore(ctx context.Context, u *model.User) error
	Delete(ctx context.Context, studentID string) error
	List(ctx context.Context, f UserFilter) ([]model.User, int64, error)
}

type userRepo struct{ db *gorm.DB }

func NewUserRepo(db *gorm.DB) UserRepo { return &userRepo{db: db} }

func (r *userRepo) Create(ctx context.Context, u *model.User) error {
	return r.db.WithContext(ctx).Create(u).Error
}

func (r *userRepo) GetByID(ctx context.Context, studentID string) (*model.User, error) {
	var u model.User
	err := r.db.WithContext(ctx).Where("student_id = ?", studentID).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *userRepo) Exists(ctx context.Context, studentID string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.User{}).
		Where("student_id = ?", studentID).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *userRepo) GetByIDIncludingDeleted(ctx context.Context, studentID string) (*model.User, error) {
	var u model.User
	err := r.db.WithContext(ctx).Unscoped().Where("student_id = ?", studentID).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *userRepo) Restore(ctx context.Context, u *model.User) error {
	return r.db.WithContext(ctx).Unscoped().Model(&model.User{}).
		Where("student_id = ?", u.StudentID).
		Updates(map[string]any{
			"name":                 u.Name,
			"phone":                u.Phone,
			"wechat":               u.WeChat,
			"qq":                   u.QQ,
			"email":                u.Email,
			"password_hash":        u.PasswordHash,
			"role":                 u.Role,
			"must_change_password": u.MustChangePassword,
			"banned":               false,
			"banned_at":            nil,
			"deleted_at":           nil,
			"updated_at":           time.Now(),
		}).Error
}

func (r *userRepo) Update(ctx context.Context, u *model.User) error {
	return r.db.WithContext(ctx).Model(&model.User{}).
		Where("student_id = ?", u.StudentID).Updates(u).Error
}

func (r *userRepo) UpdateFields(ctx context.Context, studentID string, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	fields["updated_at"] = time.Now()
	return r.db.WithContext(ctx).Model(&model.User{}).
		Where("student_id = ?", studentID).Updates(fields).Error
}

func (r *userRepo) Delete(ctx context.Context, studentID string) error {
	return r.db.WithContext(ctx).Where("student_id = ?", studentID).Delete(&model.User{}).Error
}

func (r *userRepo) List(ctx context.Context, f UserFilter) ([]model.User, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.User{})

	if kw := strings.TrimSpace(f.Keyword); kw != "" {
		like := "%" + strings.ToLower(kw) + "%"
		q = q.Where(
			`LOWER(student_id) LIKE ? OR LOWER(name) LIKE ? OR LOWER(phone) LIKE ? OR LOWER(wechat) LIKE ? OR LOWER(qq) LIKE ?`,
			like, like, like, like, like,
		)
	}
	if f.Role != nil {
		q = q.Where("role = ?", *f.Role)
	}
	if f.Banned != nil {
		q = q.Where("banned = ?", *f.Banned)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	users := make([]model.User, 0)
	if err := q.Order("role ASC, student_id ASC").
		Offset(offset(f.Page, f.Size)).Limit(limit(f.Size)).
		Find(&users).Error; err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

func offset(page, size int) int {
	if page < 1 {
		page = 1
	}
	return (page - 1) * limit(size)
}

func limit(size int) int {
	if size <= 0 {
		size = 10
	}
	return size
}

func wrapErr(op string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", op, err)
}
