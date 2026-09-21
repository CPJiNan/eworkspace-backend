package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"eworkspace/internal/model"
)

type SemesterRepo interface {
	Create(ctx context.Context, s *model.Semester) error
	GetByID(ctx context.Context, id uint) (*model.Semester, error)
	GetByName(ctx context.Context, name string) (*model.Semester, error)
	List(ctx context.Context) ([]model.Semester, error)
	UpdateName(ctx context.Context, id uint, name string) error
	Delete(ctx context.Context, id uint) error
}

type semesterRepo struct{ db *gorm.DB }

func NewSemesterRepo(db *gorm.DB) SemesterRepo { return &semesterRepo{db: db} }

func (r *semesterRepo) Create(ctx context.Context, s *model.Semester) error {
	err := r.db.WithContext(ctx).Create(s).Error
	if isUniqueViolation(err) {
		return ErrDuplicate
	}
	return wrapErr("创建学期", err)
}

func (r *semesterRepo) GetByID(ctx context.Context, id uint) (*model.Semester, error) {
	var s model.Semester
	err := r.db.WithContext(ctx).First(&s, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *semesterRepo) GetByName(ctx context.Context, name string) (*model.Semester, error) {
	var s model.Semester
	err := r.db.WithContext(ctx).Where("name = ?", name).First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *semesterRepo) List(ctx context.Context) ([]model.Semester, error) {
	list := make([]model.Semester, 0)
	err := r.db.WithContext(ctx).Order("id DESC").Find(&list).Error
	return list, err
}

func (r *semesterRepo) UpdateName(ctx context.Context, id uint, name string) error {
	err := r.db.WithContext(ctx).Model(&model.Semester{}).
		Where("id = ?", id).
		Updates(map[string]any{"name": name, "updated_at": time.Now()}).Error
	if isUniqueViolation(err) {
		return ErrDuplicate
	}
	return wrapErr("重命名学期", err)
}

func (r *semesterRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("semester_id = ?", id).Delete(&model.ProjectSemester{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Semester{}, id).Error
	})
}

type TagRepo interface {
	Create(ctx context.Context, t *model.Tag) error
	GetByID(ctx context.Context, id uint) (*model.Tag, error)
	GetByName(ctx context.Context, name string, t model.TagType) (*model.Tag, error)
	List(ctx context.Context, t *model.TagType) ([]model.Tag, error)
	Rename(ctx context.Context, id uint, name string) error
	Delete(ctx context.Context, id uint) error
}

type tagRepo struct{ db *gorm.DB }

func NewTagRepo(db *gorm.DB) TagRepo { return &tagRepo{db: db} }

func (r *tagRepo) Create(ctx context.Context, t *model.Tag) error {
	err := r.db.WithContext(ctx).Create(t).Error
	if isUniqueViolation(err) {
		return ErrDuplicate
	}
	return wrapErr("创建标签", err)
}

func (r *tagRepo) GetByID(ctx context.Context, id uint) (*model.Tag, error) {
	var t model.Tag
	err := r.db.WithContext(ctx).First(&t, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *tagRepo) GetByName(ctx context.Context, name string, t model.TagType) (*model.Tag, error) {
	var tag model.Tag
	err := r.db.WithContext(ctx).Where("name = ? AND type = ?", strings.TrimSpace(name), t).First(&tag).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &tag, nil
}

func (r *tagRepo) List(ctx context.Context, t *model.TagType) ([]model.Tag, error) {
	q := r.db.WithContext(ctx).Model(&model.Tag{})
	if t != nil {
		q = q.Where("type = ?", *t)
	}
	list := make([]model.Tag, 0)
	if err := q.Order("type ASC, name ASC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (r *tagRepo) Rename(ctx context.Context, id uint, name string) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var tag model.Tag
		if err := tx.First(&tag, id).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Tag{}).Where("id = ?", id).
			Updates(map[string]any{"name": name, "updated_at": time.Now()}).Error; err != nil {
			return err
		}
		if tag.Type == model.TagTypeDivision {
			if err := tx.Model(&model.Assignment{}).Where("tag_id = ?", id).
				Updates(map[string]any{"name": name, "updated_at": time.Now()}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if isUniqueViolation(err) {
		return ErrDuplicate
	}
	return wrapErr("重命名标签", err)
}

func (r *tagRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("tag_id = ?", id).Delete(&model.ProjectTag{}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Assignment{}).Where("tag_id = ?", id).
			Updates(map[string]any{"tag_id": nil, "updated_at": time.Now()}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Tag{}, id).Error
	})
}
