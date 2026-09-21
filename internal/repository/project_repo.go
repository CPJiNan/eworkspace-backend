package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"eworkspace/internal/model"
)

type ProjectFilter struct {
	Keyword      string
	SemesterID   *uint
	TagID        *uint
	Statuses     []model.ProjectStatus
	DeadlineFrom *time.Time
	DeadlineTo   *time.Time
	IncludeAll   bool
	Page         int
	Size         int
}

type ProjectRepo interface {
	Create(ctx context.Context, p *model.Project) error
	GetByID(ctx context.Context, id uint) (*model.Project, error)
	Update(ctx context.Context, p *model.Project) error
	UpdateFields(ctx context.Context, id uint, fields map[string]any) error
	ReplaceSemesters(ctx context.Context, id uint, semesterIDs []uint) error
	ReplaceTags(ctx context.Context, id uint, tagIDs []uint) error
	Delete(ctx context.Context, id uint) error
	List(ctx context.Context, f ProjectFilter) ([]model.Project, int64, error)
	ListExpiredActive(ctx context.Context, now time.Time) ([]model.Project, error)
}

type projectRepo struct{ db *gorm.DB }

func NewProjectRepo(db *gorm.DB) ProjectRepo { return &projectRepo{db: db} }

func (r *projectRepo) Create(ctx context.Context, p *model.Project) error {
	return wrapErr("创建项目", r.db.WithContext(ctx).Create(p).Error)
}

func (r *projectRepo) GetByID(ctx context.Context, id uint) (*model.Project, error) {
	var p model.Project
	err := r.db.WithContext(ctx).
		Preload("Semesters").
		Preload("Tags").
		Preload("Assignments", func(db *gorm.DB) *gorm.DB {
			return db.Order("assignments.id ASC")
		}).
		First(&p, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *projectRepo) Update(ctx context.Context, p *model.Project) error {
	return wrapErr("更新项目", r.db.WithContext(ctx).Model(&model.Project{}).
		Where("id = ?", p.ID).
		Select("name", "description", "deadline", "status", "deleted_reason", "updated_at").
		Updates(p).Error)
}

func (r *projectRepo) UpdateFields(ctx context.Context, id uint, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	fields["updated_at"] = time.Now()
	return r.db.WithContext(ctx).Model(&model.Project{}).Where("id = ?", id).Updates(fields).Error
}

func (r *projectRepo) ReplaceSemesters(ctx context.Context, id uint, semesterIDs []uint) error {
	return wrapErr("更新项目学期", r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("project_id = ?", id).Delete(&model.ProjectSemester{}).Error; err != nil {
			return err
		}
		if len(semesterIDs) == 0 {
			return nil
		}
		rows := make([]model.ProjectSemester, 0, len(semesterIDs))
		for _, sid := range semesterIDs {
			rows = append(rows, model.ProjectSemester{ProjectID: id, SemesterID: sid})
		}
		return tx.Create(&rows).Error
	}))
}

func (r *projectRepo) ReplaceTags(ctx context.Context, id uint, tagIDs []uint) error {
	return wrapErr("更新项目标签", r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("project_id = ?", id).Delete(&model.ProjectTag{}).Error; err != nil {
			return err
		}
		if len(tagIDs) == 0 {
			return nil
		}
		rows := make([]model.ProjectTag, 0, len(tagIDs))
		for _, tid := range tagIDs {
			rows = append(rows, model.ProjectTag{ProjectID: id, TagID: tid})
		}
		return tx.Create(&rows).Error
	}))
}

func (r *projectRepo) Delete(ctx context.Context, id uint) error {
	return withRetry(ctx, func() error {
		return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Unscoped().Where("project_id = ?", id).Delete(&model.Discussion{}).Error; err != nil {
				return err
			}
			if err := tx.Where("project_id = ?", id).Delete(&model.ProjectMember{}).Error; err != nil {
				return err
			}
			if err := tx.Where("project_id = ?", id).Delete(&model.Notification{}).Error; err != nil {
				return err
			}
			if err := tx.Where("project_id = ?", id).Delete(&model.Assignment{}).Error; err != nil {
				return err
			}
			if err := tx.Where("project_id = ?", id).Delete(&model.ProjectSemester{}).Error; err != nil {
				return err
			}
			if err := tx.Where("project_id = ?", id).Delete(&model.ProjectTag{}).Error; err != nil {
				return err
			}
			return tx.Delete(&model.Project{}, id).Error
		})
	})
}

func (r *projectRepo) List(ctx context.Context, f ProjectFilter) ([]model.Project, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.Project{})

	if kw := strings.ToLower(strings.TrimSpace(f.Keyword)); kw != "" {
		like := "%" + kw + "%"
		q = q.Where(`(
			LOWER(projects.name) LIKE ? OR LOWER(projects.description) LIKE ?
			OR EXISTS (SELECT 1 FROM assignments a WHERE a.project_id = projects.id AND LOWER(a.name) LIKE ?)
			OR EXISTS (SELECT 1 FROM project_tags pt JOIN tags t ON t.id = pt.tag_id
			           WHERE pt.project_id = projects.id AND LOWER(t.name) LIKE ?)
			OR EXISTS (SELECT 1 FROM project_semesters ps JOIN semesters s ON s.id = ps.semester_id
			           WHERE ps.project_id = projects.id AND LOWER(s.name) LIKE ?)
		)`, like, like, like, like, like)
	}

	if f.SemesterID != nil {
		q = q.Where(`EXISTS (SELECT 1 FROM project_semesters ps WHERE ps.project_id = projects.id AND ps.semester_id = ?)`, *f.SemesterID)
	}
	if f.TagID != nil {
		q = q.Where(`EXISTS (SELECT 1 FROM project_tags pt WHERE pt.project_id = projects.id AND pt.tag_id = ?)`, *f.TagID)
	}

	if len(f.Statuses) > 0 {
		q = q.Where("projects.status IN ?", f.Statuses)
	} else if !f.IncludeAll {
		q = q.Where("projects.status = ?", model.ProjectStatusActive)
	}

	if f.DeadlineFrom != nil {
		q = q.Where("projects.deadline >= ?", *f.DeadlineFrom)
	}
	if f.DeadlineTo != nil {
		q = q.Where("projects.deadline <= ?", *f.DeadlineTo)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	list := make([]model.Project, 0)
	if err := q.
		Preload("Semesters").
		Preload("Tags").
		Preload("Assignments", func(db *gorm.DB) *gorm.DB {
			return db.Order("assignments.id ASC")
		}).
		Order("CASE WHEN projects.status = 'active' THEN 0 ELSE 1 END ASC").
		Order("projects.created_at DESC").
		Offset(offset(f.Page, f.Size)).Limit(limit(f.Size)).
		Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (r *projectRepo) ListExpiredActive(ctx context.Context, now time.Time) ([]model.Project, error) {
	list := make([]model.Project, 0)
	err := r.db.WithContext(ctx).
		Where("status = ? AND deadline <= ?", model.ProjectStatusActive, now).
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}
