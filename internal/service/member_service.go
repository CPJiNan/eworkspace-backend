package service

import (
	"context"
	"errors"
	"time"

	"eworkspace/internal/model"
	"eworkspace/internal/pkg/apperr"
	"eworkspace/internal/repository"
)

type AssignInput struct {
	StudentIDs []string
}

type MemberView struct {
	StudentID      string              `json:"studentId"`
	StudentName    string              `json:"studentName"`
	Assigned       bool                `json:"assigned"`
	AssignedBy     string              `json:"assignedBy,omitempty"`
	ClaimedAt      time.Time           `json:"claimedAt"`
	AssignmentID   uint                `json:"assignmentId"`
	AssignmentName string              `json:"assignmentName"`
	ProjectID      uint                `json:"projectId"`
	ProjectName    string              `json:"projectName"`
	ProjectStatus  model.ProjectStatus `json:"projectStatus"`
	Deadline       time.Time           `json:"deadline"`
}

type MyTasksInput struct {
	Keyword string
	Page    int
	Size    int
}

type MemberService interface {
	Claim(ctx context.Context, operator *model.User, assignmentID uint) (*AssignmentDTO, error)
	CancelClaim(ctx context.Context, operator *model.User, assignmentID uint) (*AssignmentDTO, error)
	Assign(ctx context.Context, operator *model.User, assignmentID uint, in AssignInput) ([]MemberDTO, error)
	MyTasks(ctx context.Context, operator *model.User, in MyTasksInput) ([]MemberView, PageMeta, error)
	AssignmentMembers(ctx context.Context, operator *model.User, assignmentID uint) ([]MemberDTO, error)
}

type memberService struct {
	meta    *meta
	repos   *repository.Repositories
	log     Logger
	notify  NotificationService
	project ProjectService
}

func NewMemberService(m *meta, repos *repository.Repositories, log Logger,
	notify NotificationService, project ProjectService) MemberService {
	return &memberService{meta: m, repos: repos, log: log, notify: notify, project: project}
}

func (s *memberService) Claim(ctx context.Context, operator *model.User, assignmentID uint) (*AssignmentDTO, error) {
	assignment, project, err := s.loadAssignment(ctx, assignmentID)
	if err != nil {
		return nil, err
	}
	if err := s.ensureClaimable(project); err != nil {
		return nil, err
	}

	if err := s.repos.Assignment.Claim(ctx, assignment.ID, operator.StudentID, operator.Name, ""); err != nil {
		return nil, s.translateClaimErr(err)
	}
	return s.assignmentAfterChange(ctx, operator, project.ID, assignmentID)
}

func (s *memberService) CancelClaim(ctx context.Context, operator *model.User, assignmentID uint) (*AssignmentDTO, error) {
	assignment, project, err := s.loadAssignment(ctx, assignmentID)
	if err != nil {
		return nil, err
	}

	if err := s.repos.Assignment.Cancel(ctx, assignment.ID, operator.StudentID); err != nil {
		if errors.Is(err, repository.ErrNotClaimed) {
			return nil, apperr.ErrNotClaimed
		}
		return nil, apperr.Internal(err)
	}
	return s.assignmentAfterChange(ctx, operator, project.ID, assignmentID)
}

func (s *memberService) Assign(ctx context.Context, operator *model.User, assignmentID uint, in AssignInput) ([]MemberDTO, error) {
	if !operator.Role.CanManage() {
		return nil, apperr.ErrForbidden
	}
	assignment, project, err := s.loadAssignment(ctx, assignmentID)
	if err != nil {
		return nil, err
	}
	if err := s.ensureClaimable(project); err != nil {
		return nil, err
	}
	if len(in.StudentIDs) == 0 {
		return nil, apperr.Validation("请选择要指派的成员")
	}

	result := make([]MemberDTO, 0, len(in.StudentIDs))
	for _, studentID := range in.StudentIDs {
		target, err := s.repos.User.GetByID(ctx, studentID)
		if err != nil {
			return nil, apperr.Internal(err)
		}
		if target == nil {
			return nil, apperr.NotFound("成员不存在：" + studentID)
		}
		if err := s.repos.Assignment.Claim(ctx, assignment.ID, target.StudentID, target.Name, operator.StudentID); err != nil {
			return nil, s.translateClaimErr(err)
		}
		if err := s.notify.NotifyAssigned(ctx, target, project, assignment); err != nil {
			s.log.Warn("指派站内短信发送失败", "studentId", target.StudentID, "err", err)
		}
		result = append(result, MemberDTO{
			StudentID:   target.StudentID,
			StudentName: target.Name,
			Assigned:    true,
			AssignedBy:  operator.StudentID,
			CreatedAt:   time.Now(),
		})
	}
	return result, nil
}

func (s *memberService) MyTasks(ctx context.Context, operator *model.User, in MyTasksInput) ([]MemberView, PageMeta, error) {
	members, total, err := s.repos.Member.List(ctx, repository.MemberFilter{
		StudentID: operator.StudentID,
		Keyword:   in.Keyword,
		Page:      in.Page,
		Size:      in.Size,
	})
	if err != nil {
		return nil, PageMeta{}, apperr.Internal(err)
	}

	views := make([]MemberView, 0, len(members))
	for i := range members {
		m := members[i]
		view := MemberView{
			StudentID:    m.StudentID,
			StudentName:  m.StudentName,
			Assigned:     m.Assigned,
			AssignedBy:   m.AssignedBy,
			ClaimedAt:    m.CreatedAt,
			AssignmentID: m.AssignmentID,
		}
		if m.Assignment != nil {
			view.AssignmentName = m.Assignment.Name
		}
		if m.Project != nil {
			view.ProjectID = m.Project.ID
			view.ProjectName = m.Project.Name
			view.ProjectStatus = m.Project.Status
			view.Deadline = m.Project.Deadline
		}
		if m.Assignment != nil && m.Assignment.Deadline != nil {
			view.Deadline = *m.Assignment.Deadline
		}
		views = append(views, view)
	}
	return views, NewPageMeta(in.Page, in.Size, total), nil
}

func (s *memberService) AssignmentMembers(ctx context.Context, operator *model.User, assignmentID uint) ([]MemberDTO, error) {
	assignment, project, err := s.loadAssignment(ctx, assignmentID)
	if err != nil {
		return nil, err
	}

	members, err := s.repos.Member.ListByAssignment(ctx, assignment.ID)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	participant := false
	if !operator.Role.CanManage() {
		count, err := s.repos.Member.CountByProject(ctx, project.ID, operator.StudentID)
		if err != nil {
			return nil, apperr.Internal(err)
		}
		participant = count > 0
	}

	out := make([]MemberDTO, 0, len(members))
	for _, m := range members {
		dto := MemberDTO{
			StudentID:   m.StudentID,
			StudentName: m.StudentName,
			CreatedAt:   m.CreatedAt,
		}
		if operator.Role.CanManage() || participant {
			dto.Assigned = m.Assigned
			dto.AssignedBy = m.AssignedBy
		}
		out = append(out, dto)
	}
	return out, nil
}

func (s *memberService) loadAssignment(ctx context.Context, assignmentID uint) (*model.Assignment, *model.Project, error) {
	assignment, err := s.repos.Assignment.GetByID(ctx, assignmentID)
	if err != nil {
		return nil, nil, apperr.Internal(err)
	}
	if assignment == nil {
		return nil, nil, apperr.NotFound("分工不存在")
	}
	project, err := s.repos.Project.GetByID(ctx, assignment.ProjectID)
	if err != nil {
		return nil, nil, apperr.Internal(err)
	}
	if project == nil {
		return nil, nil, apperr.NotFound("项目不存在")
	}
	return assignment, project, nil
}

func (s *memberService) ensureClaimable(project *model.Project) error {
	if project.Status != model.ProjectStatusActive {
		return apperr.ErrProjectClosed
	}
	return nil
}

func (s *memberService) translateClaimErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, repository.ErrCapacityFull):
		return apperr.ErrCapacityFull
	case errors.Is(err, repository.ErrAlreadyClaimed):
		return apperr.ErrAlreadyClaimed
	case errors.Is(err, repository.ErrNotClaimed):
		return apperr.ErrNotClaimed
	default:
		return apperr.Internal(err)
	}
}

func (s *memberService) assignmentAfterChange(ctx context.Context, operator *model.User, projectID, assignmentID uint) (*AssignmentDTO, error) {
	if _, err := s.repos.Project.GetByID(ctx, projectID); err != nil {
		return nil, apperr.Internal(err)
	}
	return s.project.GetAssignmentView(ctx, operator, assignmentID)
}
