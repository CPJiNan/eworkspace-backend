package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"eworkspace/internal/model"
)

type MemberFilter struct {
	StudentID    string
	ProjectID    *uint
	AssignmentID *uint
	Keyword      string
	Page         int
	Size         int
}

type WorkloadRow struct {
	StudentID   string
	StudentName string
	Workload    int
}

type ProjectMemberRepo interface {
	Get(ctx context.Context, assignmentID uint, studentID string) (*model.ProjectMember, error)
	List(ctx context.Context, f MemberFilter) ([]model.ProjectMember, int64, error)
	ListByAssignment(ctx context.Context, assignmentID uint) ([]model.ProjectMember, error)
	ListByProject(ctx context.Context, projectID uint) ([]model.ProjectMember, error)
	CountByProject(ctx context.Context, projectID uint, studentID string) (int64, error)
	WorkloadRanking(ctx context.Context, semesterIDs []uint) ([]WorkloadRow, error)
}

type memberRepo struct{ db *gorm.DB }

func NewProjectMemberRepo(db *gorm.DB) ProjectMemberRepo { return &memberRepo{db: db} }

func (r *memberRepo) Get(ctx context.Context, assignmentID uint, studentID string) (*model.ProjectMember, error) {
	var m model.ProjectMember
	err := r.db.WithContext(ctx).
		Where("assignment_id = ? AND student_id = ?", assignmentID, studentID).
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *memberRepo) List(ctx context.Context, f MemberFilter) ([]model.ProjectMember, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.ProjectMember{})

	if f.StudentID != "" {
		q = q.Where("project_members.student_id = ?", f.StudentID)
	}
	if f.ProjectID != nil {
		q = q.Where("project_members.project_id = ?", *f.ProjectID)
	}
	if f.AssignmentID != nil {
		q = q.Where("project_members.assignment_id = ?", *f.AssignmentID)
	}
	if kw := likePattern(f.Keyword); kw != "" {
		q = q.Where(`EXISTS (
			SELECT 1 FROM assignments a JOIN projects p ON p.id = a.project_id
			WHERE a.id = project_members.assignment_id
			  AND p.deleted_at IS NULL
			  AND (LOWER(a.name) LIKE ? OR LOWER(p.name) LIKE ?)
		)`, kw, kw)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	list := make([]model.ProjectMember, 0)
	if err := q.
		Preload("Project").
		Preload("Assignment").
		Order("project_members.created_at DESC, project_members.id DESC").
		Offset(offset(f.Page, f.Size)).Limit(limit(f.Size)).
		Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (r *memberRepo) ListByAssignment(ctx context.Context, assignmentID uint) ([]model.ProjectMember, error) {
	list := make([]model.ProjectMember, 0)
	err := r.db.WithContext(ctx).
		Where("assignment_id = ?", assignmentID).
		Order("created_at ASC").
		Find(&list).Error
	return list, err
}

func (r *memberRepo) ListByProject(ctx context.Context, projectID uint) ([]model.ProjectMember, error) {
	list := make([]model.ProjectMember, 0)
	err := r.db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("assignment_id ASC, created_at ASC").
		Find(&list).Error
	return list, err
}

func (r *memberRepo) CountByProject(ctx context.Context, projectID uint, studentID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.ProjectMember{}).
		Where("project_id = ? AND student_id = ?", projectID, studentID).Count(&count).Error
	return count, err
}

func (r *memberRepo) WorkloadRanking(ctx context.Context, semesterIDs []uint) ([]WorkloadRow, error) {
	q := r.db.WithContext(ctx).Model(&model.ProjectMember{}).
		Select(`project_members.student_id AS student_id,
			MAX(project_members.student_name) AS student_name,
			SUM(assignments.workload) AS workload`).
		Joins("JOIN assignments ON assignments.id = project_members.assignment_id").
		Joins("JOIN projects ON projects.id = project_members.project_id").
		Where("projects.deleted_at IS NULL")
	if len(semesterIDs) > 0 {
		q = q.Where(`EXISTS (
			SELECT 1 FROM project_semesters ps
			WHERE ps.project_id = project_members.project_id
			  AND ps.semester_id IN ?
		)`, semesterIDs)
	}

	list := make([]WorkloadRow, 0)
	err := q.Group("project_members.student_id").
		Having("SUM(assignments.workload) > 0").
		Order("workload DESC, project_members.student_id ASC").
		Scan(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

type DiscussionRepo interface {
	Create(ctx context.Context, d *model.Discussion) error
	GetByID(ctx context.Context, id uint) (*model.Discussion, error)
	Update(ctx context.Context, id uint, content string) error
	Delete(ctx context.Context, id uint) error
	ListByProject(ctx context.Context, projectID uint) ([]model.Discussion, error)
	CountByProject(ctx context.Context, projectID uint) (int64, error)
}

type discussionRepo struct{ db *gorm.DB }

func NewDiscussionRepo(db *gorm.DB) DiscussionRepo { return &discussionRepo{db: db} }

func (r *discussionRepo) Create(ctx context.Context, d *model.Discussion) error {
	return wrapErr("发表讨论", r.db.WithContext(ctx).Create(d).Error)
}

func (r *discussionRepo) GetByID(ctx context.Context, id uint) (*model.Discussion, error) {
	var d model.Discussion
	err := r.db.WithContext(ctx).First(&d, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *discussionRepo) Update(ctx context.Context, id uint, content string) error {
	return wrapErr("编辑讨论", r.db.WithContext(ctx).Model(&model.Discussion{}).
		Where("id = ?", id).Update("content", content).Error)
}

func (r *discussionRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&model.Discussion{}, id).Error
}

func (r *discussionRepo) ListByProject(ctx context.Context, projectID uint) ([]model.Discussion, error) {
	list := make([]model.Discussion, 0)
	err := r.db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("created_at ASC, id ASC").
		Find(&list).Error
	return list, err
}

func (r *discussionRepo) CountByProject(ctx context.Context, projectID uint) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Discussion{}).
		Where("project_id = ?", projectID).Count(&count).Error
	return count, err
}
