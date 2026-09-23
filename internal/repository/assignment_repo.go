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

var (
	ErrCapacityFull   = errors.New("分工名额已满")
	ErrAlreadyClaimed = errors.New("已申领该分工")
	ErrNotClaimed     = errors.New("未申领该分工")
	ErrDuplicate      = errors.New("数据已存在")
)

type AssignmentSpec struct {
	ID          *uint
	Name        string
	Capacity    int
	Workload    *int
	TagID       *uint
	Description string
	Deadline    *time.Time
}

func workloadOr(workload *int) int {
	if workload == nil {
		return 1
	}
	return *workload
}

type AssignmentRepo interface {
	GetByID(ctx context.Context, id uint) (*model.Assignment, error)
	ListByProject(ctx context.Context, projectID uint) ([]model.Assignment, error)
	Create(ctx context.Context, a *model.Assignment) error
	Update(ctx context.Context, a *model.Assignment) error
	CreateWithProject(ctx context.Context, project *model.Project, specs []AssignmentSpec) error
	UpdateWithProject(ctx context.Context, project *model.Project, fields map[string]any,
		semesterIDs, tagIDs *[]uint, specs *[]AssignmentSpec) ([]uint, error)
	Delete(ctx context.Context, id uint) error
	Claim(ctx context.Context, assignmentID uint, studentID, studentName string, assignedBy string) error
	Cancel(ctx context.Context, assignmentID uint, studentID string) error
}

type assignmentRepo struct{ db *gorm.DB }

func NewAssignmentRepo(db *gorm.DB) AssignmentRepo { return &assignmentRepo{db: db} }

func (r *assignmentRepo) GetByID(ctx context.Context, id uint) (*model.Assignment, error) {
	var a model.Assignment
	err := r.db.WithContext(ctx).First(&a, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *assignmentRepo) ListByProject(ctx context.Context, projectID uint) ([]model.Assignment, error) {
	list := make([]model.Assignment, 0)
	err := r.db.WithContext(ctx).Where("project_id = ?", projectID).Order("id ASC").Find(&list).Error
	return list, err
}

func (r *assignmentRepo) Create(ctx context.Context, a *model.Assignment) error {
	return wrapErr("创建分工", r.db.WithContext(ctx).Create(a).Error)
}

func (r *assignmentRepo) Update(ctx context.Context, a *model.Assignment) error {
	return wrapErr("更新分工", r.db.WithContext(ctx).Model(&model.Assignment{}).
		Where("id = ?", a.ID).
		Select("name", "description", "capacity", "workload", "tag_id", "deadline", "updated_at").
		Updates(a).Error)
}

func (r *assignmentRepo) Delete(ctx context.Context, id uint) error {
	return wrapErr("删除分工", r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var a model.Assignment
		if err := tx.First(&a, id).Error; err != nil {
			return err
		}
		if err := tx.Where("assignment_id = ?", id).Delete(&model.ProjectMember{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Assignment{}, id).Error
	}))
}

func (r *assignmentRepo) CreateWithProject(ctx context.Context, project *model.Project, specs []AssignmentSpec) error {
	return withRetry(ctx, func() error {
		return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(project).Error; err != nil {
				return err
			}
			for _, spec := range specs {
				a := model.Assignment{
					ProjectID:   project.ID,
					Name:        spec.Name,
					Description: spec.Description,
					Capacity:    spec.Capacity,
					Workload:    workloadOr(spec.Workload),
					TagID:       spec.TagID,
					Deadline:    spec.Deadline,
				}
				if err := tx.Create(&a).Error; err != nil {
					return err
				}
			}
			return nil
		})
	})
}

func (r *assignmentRepo) UpdateWithProject(
	ctx context.Context,
	project *model.Project,
	fields map[string]any,
	semesterIDs, tagIDs *[]uint,
	specs *[]AssignmentSpec,
) ([]uint, error) {
	removed := make([]uint, 0)
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if len(fields) > 0 {
			fields["updated_at"] = time.Now()
			if err := tx.Model(&model.Project{}).Where("id = ?", project.ID).Updates(fields).Error; err != nil {
				return err
			}
		}
		if semesterIDs != nil {
			if err := replaceSemesters(tx, project.ID, *semesterIDs); err != nil {
				return err
			}
		}
		if tagIDs != nil {
			if err := replaceTags(tx, project.ID, *tagIDs); err != nil {
				return err
			}
		}
		if specs == nil {
			return nil
		}

		keep := make([]uint, 0, len(*specs))
		for _, spec := range *specs {
			if spec.ID == nil {
				created := model.Assignment{
					ProjectID:   project.ID,
					Name:        spec.Name,
					Description: spec.Description,
					Capacity:    spec.Capacity,
					Workload:    workloadOr(spec.Workload),
					TagID:       spec.TagID,
					Deadline:    spec.Deadline,
				}
				if err := tx.Create(&created).Error; err != nil {
					return err
				}
				keep = append(keep, created.ID)
				continue
			}

			var existing model.Assignment
			if err := tx.First(&existing, *spec.ID).Error; err != nil {
				return err
			}
			if existing.ProjectID != project.ID {
				return fmt.Errorf("分工 %d 不属于项目 %d", existing.ID, project.ID)
			}
			if spec.Capacity < existing.ClaimedCount {
				return fmt.Errorf("分工“%s”的人数不能少于已申领人数（%d）", existing.Name, existing.ClaimedCount)
			}
			updates := map[string]any{
				"name":        spec.Name,
				"description": spec.Description,
				"capacity":    spec.Capacity,
				"tag_id":      spec.TagID,
				"deadline":    spec.Deadline,
				"updated_at":  time.Now(),
			}
			if spec.Workload != nil {
				updates["workload"] = *spec.Workload
			}
			if err := tx.Model(&model.Assignment{}).Where("id = ?", existing.ID).
				Updates(updates).Error; err != nil {
				return err
			}
			keep = append(keep, existing.ID)
		}

		q := tx.Model(&model.Assignment{}).Where("project_id = ?", project.ID)
		if len(keep) > 0 {
			q = q.Where("id NOT IN ?", keep)
		}
		var stale []model.Assignment
		if err := q.Find(&stale).Error; err != nil {
			return err
		}
		for _, a := range stale {
			if err := tx.Where("assignment_id = ?", a.ID).Delete(&model.ProjectMember{}).Error; err != nil {
				return err
			}
			if err := tx.Delete(&model.Assignment{}, a.ID).Error; err != nil {
				return err
			}
			removed = append(removed, a.ID)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return removed, nil
}

func replaceSemesters(tx *gorm.DB, projectID uint, semesterIDs []uint) error {
	if err := tx.Where("project_id = ?", projectID).Delete(&model.ProjectSemester{}).Error; err != nil {
		return err
	}
	if len(semesterIDs) == 0 {
		return nil
	}
	rows := make([]model.ProjectSemester, 0, len(semesterIDs))
	for _, sid := range semesterIDs {
		rows = append(rows, model.ProjectSemester{ProjectID: projectID, SemesterID: sid})
	}
	return tx.Create(&rows).Error
}

func replaceTags(tx *gorm.DB, projectID uint, tagIDs []uint) error {
	if err := tx.Where("project_id = ?", projectID).Delete(&model.ProjectTag{}).Error; err != nil {
		return err
	}
	if len(tagIDs) == 0 {
		return nil
	}
	rows := make([]model.ProjectTag, 0, len(tagIDs))
	for _, tid := range tagIDs {
		rows = append(rows, model.ProjectTag{ProjectID: projectID, TagID: tid})
	}
	return tx.Create(&rows).Error
}

func (r *assignmentRepo) Claim(ctx context.Context, assignmentID uint, studentID, studentName, assignedBy string) error {
	return withRetry(ctx, func() error {
		return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var assignment model.Assignment
			if err := tx.First(&assignment, assignmentID).Error; err != nil {
				return err
			}

			res := tx.Model(&model.Assignment{}).
				Where("id = ? AND claimed_count < capacity", assignmentID).
				UpdateColumn("claimed_count", gorm.Expr("claimed_count + 1"))
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				return ErrCapacityFull
			}

			member := model.ProjectMember{
				ProjectID:    assignment.ProjectID,
				AssignmentID: assignmentID,
				StudentID:    studentID,
				StudentName:  studentName,
				Assigned:     assignedBy != "",
				AssignedBy:   assignedBy,
			}
			if err := tx.Create(&member).Error; err != nil {
				if isUniqueViolation(err) {
					return ErrAlreadyClaimed
				}
				return err
			}
			return nil
		})
	})
}

func (r *assignmentRepo) Cancel(ctx context.Context, assignmentID uint, studentID string) error {
	return withRetry(ctx, func() error {
		return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			res := tx.Where("assignment_id = ? AND student_id = ?", assignmentID, studentID).
				Delete(&model.ProjectMember{})
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				return ErrNotClaimed
			}
			return tx.Model(&model.Assignment{}).
				Where("id = ? AND claimed_count > 0", assignmentID).
				UpdateColumn("claimed_count", gorm.Expr("claimed_count - 1")).Error
		})
	})
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}
