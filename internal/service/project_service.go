package service

import (
	"context"
	"strings"
	"time"

	"eworkspace/internal/model"
	"eworkspace/internal/pkg/apperr"
	"eworkspace/internal/pkg/validate"
	"eworkspace/internal/repository"
)

type AssignmentInput struct {
	ID          *uint
	Name        string
	Capacity    int
	TagID       *uint
	Description string
	Deadline    *time.Time
}

type AssignmentDTO struct {
	ID           uint        `json:"id"`
	Name         string      `json:"name"`
	Description  string      `json:"description"`
	TagID        *uint       `json:"tagId,omitempty"`
	Capacity     int         `json:"capacity"`
	ClaimedCount int         `json:"claimedCount"`
	Remaining    int         `json:"remaining"`
	Full         bool        `json:"full"`
	Deadline     *time.Time  `json:"deadline,omitempty"`
	Members      []MemberDTO `json:"members,omitempty"`
	Mine         bool        `json:"mine"`
}

type MemberDTO struct {
	StudentID   string    `json:"studentId"`
	StudentName string    `json:"studentName"`
	Assigned    bool      `json:"assigned"`
	AssignedBy  string    `json:"assignedBy,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

type DiscussionDTO struct {
	ID         uint      `json:"id"`
	AuthorID   string    `json:"authorId"`
	AuthorName string    `json:"authorName"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
	Editable   bool      `json:"editable"`
}

type ProjectDTO struct {
	ID          uint                `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Deadline    time.Time           `json:"deadline"`
	Status      model.ProjectStatus `json:"status"`
	StatusName  string              `json:"statusName"`
	CreatorID   string              `json:"creatorId"`
	CreatorName string              `json:"creatorName"`
	CreatedAt   time.Time           `json:"createdAt"`
	UpdatedAt   time.Time           `json:"updatedAt"`

	Semesters       []SemesterDTO   `json:"semesters"`
	Tags            []TagDTO        `json:"tags"`
	Assignments     []AssignmentDTO `json:"assignments"`
	Discussions     []DiscussionDTO `json:"discussions,omitempty"`
	DiscussionCount int64           `json:"discussionCount"`

	TotalCapacity   int    `json:"totalCapacity"`
	TotalClaimed    int    `json:"totalClaimed"`
	Mine            bool   `json:"mine"`
	MyAssignmentIDs []uint `json:"myAssignmentIds"`
}

type ListProjectsInput struct {
	Keyword      string
	SemesterID   *uint
	TagID        *uint
	Status       *model.ProjectStatus
	DeadlineFrom *time.Time
	DeadlineTo   *time.Time
	IncludeAll   bool
	Page         int
	Size         int
}

type CreateProjectInput struct {
	Name        string
	Description string
	Deadline    time.Time
	SemesterIDs []uint
	TagIDs      []uint
	Assignments []AssignmentInput
}

type UpdateProjectInput struct {
	Name            *string
	Description     *string
	Deadline        *time.Time
	Status          *model.ProjectStatus
	SemesterIDs     *[]uint
	TagIDs          *[]uint
	Assignments     []AssignmentInput
	SyncAssignments bool
}

type CreateDiscussionInput struct {
	Content string
}

type ProjectService interface {
	List(ctx context.Context, viewer *model.User, in ListProjectsInput) ([]ProjectDTO, PageMeta, error)
	Get(ctx context.Context, viewer *model.User, id uint) (*ProjectDTO, error)
	GetAssignmentView(ctx context.Context, viewer *model.User, assignmentID uint) (*AssignmentDTO, error)
	Create(ctx context.Context, operator *model.User, in CreateProjectInput) (*ProjectDTO, error)
	Update(ctx context.Context, operator *model.User, id uint, in UpdateProjectInput) (*ProjectDTO, error)
	SetStatus(ctx context.Context, operator *model.User, id uint, status model.ProjectStatus) error
	Delete(ctx context.Context, operator *model.User, id uint, reason string) error

	AddDiscussion(ctx context.Context, operator *model.User, projectID uint, in CreateDiscussionInput) (*DiscussionDTO, error)
	UpdateDiscussion(ctx context.Context, operator *model.User, discussionID uint, in CreateDiscussionInput) (*DiscussionDTO, error)
	DeleteDiscussion(ctx context.Context, operator *model.User, discussionID uint) error
}

type projectService struct {
	meta   *meta
	repos  *repository.Repositories
	log    Logger
	notify NotificationService
}

func NewProjectService(m *meta, repos *repository.Repositories, log Logger, notify NotificationService) ProjectService {
	return &projectService{meta: m, repos: repos, log: log, notify: notify}
}

func (s *projectService) checker() *validate.Checker {
	return validate.New()
}

func (s *projectService) List(ctx context.Context, viewer *model.User, in ListProjectsInput) ([]ProjectDTO, PageMeta, error) {
	statuses := make([]model.ProjectStatus, 0, 1)
	if in.Status != nil {
		if !in.Status.IsValid() {
			return nil, PageMeta{}, apperr.Validation("状态取值不合法")
		}
		statuses = append(statuses, *in.Status)
	}

	projects, total, err := s.repos.Project.List(ctx, repository.ProjectFilter{
		Keyword:      in.Keyword,
		SemesterID:   in.SemesterID,
		TagID:        in.TagID,
		Statuses:     statuses,
		DeadlineFrom: in.DeadlineFrom,
		DeadlineTo:   in.DeadlineTo,
		IncludeAll:   in.IncludeAll,
		Page:         in.Page,
		Size:         in.Size,
	})
	if err != nil {
		return nil, PageMeta{}, apperr.Internal(err)
	}

	dtos := make([]ProjectDTO, 0, len(projects))
	for i := range projects {
		dto, err := s.assemble(ctx, viewer, &projects[i], false)
		if err != nil {
			return nil, PageMeta{}, err
		}
		dtos = append(dtos, *dto)
	}
	return dtos, NewPageMeta(in.Page, in.Size, total), nil
}

func (s *projectService) Get(ctx context.Context, viewer *model.User, id uint) (*ProjectDTO, error) {
	project, err := s.repos.Project.GetByID(ctx, id)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if project == nil {
		return nil, apperr.NotFound("项目不存在")
	}

	if !viewer.Role.CanManage() {
		participant, err := s.isParticipant(ctx, id, viewer.StudentID)
		if err != nil {
			return nil, err
		}
		if project.Status != model.ProjectStatusActive && !participant {
			return nil, apperr.NotFound("项目不存在")
		}
	}

	return s.assemble(ctx, viewer, project, true)
}

func (s *projectService) GetAssignmentView(ctx context.Context, viewer *model.User, assignmentID uint) (*AssignmentDTO, error) {
	assignment, err := s.repos.Assignment.GetByID(ctx, assignmentID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if assignment == nil {
		return nil, apperr.NotFound("分工不存在")
	}
	project, err := s.repos.Project.GetByID(ctx, assignment.ProjectID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if project == nil {
		return nil, apperr.NotFound("项目不存在")
	}

	dto, err := s.assemble(ctx, viewer, project, false)
	if err != nil {
		return nil, err
	}
	for _, a := range dto.Assignments {
		if a.ID == assignmentID {
			result := a
			return &result, nil
		}
	}
	return nil, apperr.NotFound("分工不存在")
}

func (s *projectService) Create(ctx context.Context, operator *model.User, in CreateProjectInput) (*ProjectDTO, error) {
	if !operator.Role.CanManage() {
		return nil, apperr.ErrForbidden
	}

	c := s.checker()
	name := c.Short("项目名称", in.Name)
	description := c.Long("项目描述", in.Description)
	if in.Deadline.IsZero() {
		c.Fail("截止时间不能为空")
	}
	if len(in.Assignments) == 0 {
		c.Fail("项目至少需要一个具体分工")
	}
	if len(in.Assignments) > 50 {
		c.Fail("单个项目的分工数量不能超过 50 个")
	}

	specs := make([]repository.AssignmentSpec, 0, len(in.Assignments))
	for i, a := range in.Assignments {
		label := "第 " + itoa(i+1) + " 个分工"
		aName := c.Short(label+"名称", a.Name)
		aDescription := c.Long(label+"描述", a.Description)
		capacity := c.Positive(label+"人数", a.Capacity, 100)
		if a.TagID != nil {
			if _, err := s.repos.Tag.GetByID(ctx, *a.TagID); err != nil {
				return nil, apperr.Internal(err)
			}
		}
		specs = append(specs, repository.AssignmentSpec{
			Name:        aName,
			Description: aDescription,
			Capacity:    capacity,
			TagID:       a.TagID,
			Deadline:    a.Deadline,
		})
	}
	if c.HasError() {
		return nil, apperr.Validation(c.Err())
	}

	project := &model.Project{
		Name:        name,
		Description: description,
		Deadline:    in.Deadline,
		Status:      model.ProjectStatusActive,
		CreatorID:   operator.StudentID,
		CreatorName: operator.Name,
	}
	if err := s.repos.Assignment.CreateWithProject(ctx, project, specs); err != nil {
		return nil, apperr.Internal(err)
	}
	if in.SemesterIDs != nil {
		if err := s.repos.Project.ReplaceSemesters(ctx, project.ID, in.SemesterIDs); err != nil {
			return nil, apperr.Internal(err)
		}
	}
	if in.TagIDs != nil {
		if err := s.repos.Project.ReplaceTags(ctx, project.ID, in.TagIDs); err != nil {
			return nil, apperr.Internal(err)
		}
	}

	created, err := s.repos.Project.GetByID(ctx, project.ID)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	if err := s.notify.NotifyProjectPublished(ctx, created); err != nil {
		s.log.Warn("发布项目站内短信发送失败", "projectId", project.ID, "err", err)
	}

	return s.assemble(ctx, operator, created, true)
}

func (s *projectService) Update(ctx context.Context, operator *model.User, id uint, in UpdateProjectInput) (*ProjectDTO, error) {
	if !operator.Role.CanManage() {
		return nil, apperr.ErrForbidden
	}
	project, err := s.repos.Project.GetByID(ctx, id)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if project == nil {
		return nil, apperr.NotFound("项目不存在")
	}

	c := s.checker()
	fields := map[string]any{}
	if in.Name != nil {
		fields["name"] = c.Short("项目名称", *in.Name)
	}
	if in.Description != nil {
		fields["description"] = c.Long("项目描述", *in.Description)
	}
	if in.Deadline != nil {
		if in.Deadline.IsZero() {
			c.Fail("截止时间不合法")
		} else {
			fields["deadline"] = *in.Deadline
		}
	}
	if in.Status != nil {
		if !in.Status.IsValid() {
			c.Fail("状态取值不合法")
		} else {
			fields["status"] = *in.Status
		}
	}

	specs := make([]repository.AssignmentSpec, 0, len(in.Assignments))
	var preparedAssignments *[]repository.AssignmentSpec
	if in.SyncAssignments {
		if len(in.Assignments) == 0 {
			c.Fail("项目至少需要一个具体分工")
		}
		if len(in.Assignments) > 50 {
			c.Fail("单个项目的分工数量不能超过 50 个")
		}
		for i, a := range in.Assignments {
			label := "第 " + itoa(i+1) + " 个分工"
			aName := c.Short(label+"名称", a.Name)
			aDescription := c.Long(label+"描述", a.Description)
			capacity := c.Positive(label+"人数", a.Capacity, 100)
			if a.TagID != nil {
				if _, err := s.repos.Tag.GetByID(ctx, *a.TagID); err != nil {
					return nil, apperr.Internal(err)
				}
			}
			specs = append(specs, repository.AssignmentSpec{
				ID: a.ID, Name: aName, Description: aDescription,
				Capacity: capacity, TagID: a.TagID, Deadline: a.Deadline,
			})
		}
		preparedAssignments = &specs
	}
	if c.HasError() {
		return nil, apperr.Validation(c.Err())
	}

	removed, err := s.repos.Assignment.UpdateWithProject(
		ctx, project, fields, in.SemesterIDs, in.TagIDs, preparedAssignments,
	)
	if err != nil {
		if strings.Contains(err.Error(), "人数不能少于") {
			return nil, apperr.Conflict(err.Error())
		}
		if strings.Contains(err.Error(), "不属于项目") {
			return nil, apperr.Validation("分工不属于当前项目")
		}
		return nil, apperr.Internal(err)
	}
	if len(removed) > 0 {
		s.log.Info("项目修改时移除了分工", "projectId", id, "assignmentIds", removed)
	}

	updated, err := s.repos.Project.GetByID(ctx, id)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return s.assemble(ctx, operator, updated, true)
}

func (s *projectService) SetStatus(ctx context.Context, operator *model.User, id uint, status model.ProjectStatus) error {
	if !operator.Role.CanManage() {
		return apperr.ErrForbidden
	}
	if !status.IsValid() {
		return apperr.Validation("状态取值不合法")
	}
	project, err := s.repos.Project.GetByID(ctx, id)
	if err != nil {
		return apperr.Internal(err)
	}
	if project == nil {
		return apperr.NotFound("项目不存在")
	}
	return s.repos.Project.UpdateFields(ctx, id, map[string]any{"status": status})
}

func (s *projectService) Delete(ctx context.Context, operator *model.User, id uint, reason string) error {
	if !operator.Role.CanManage() {
		return apperr.ErrForbidden
	}
	c := s.checker()
	reason = c.Required("删除原因", reason)
	if c.HasError() {
		return apperr.Validation(c.Err())
	}

	project, err := s.repos.Project.GetByID(ctx, id)
	if err != nil {
		return apperr.Internal(err)
	}
	if project == nil {
		return apperr.NotFound("项目不存在")
	}

	if err := s.repos.Project.Delete(ctx, id); err != nil {
		return apperr.Internal(err)
	}
	s.log.Info("项目已删除", "projectId", id, "operator", operator.StudentID, "reason", reason)
	return nil
}

func (s *projectService) AddDiscussion(ctx context.Context, operator *model.User, projectID uint, in CreateDiscussionInput) (*DiscussionDTO, error) {
	project, err := s.repos.Project.GetByID(ctx, projectID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if project == nil {
		return nil, apperr.NotFound("项目不存在")
	}
	if !operator.Role.CanManage() {
		participant, err := s.isParticipant(ctx, projectID, operator.StudentID)
		if err != nil {
			return nil, err
		}
		if !participant {
			return nil, apperr.ErrForbidden
		}
	}

	c := s.checker()
	content := c.Discussion("讨论内容", in.Content)
	if c.HasError() {
		return nil, apperr.Validation(c.Err())
	}

	discussion := &model.Discussion{
		ProjectID:  projectID,
		AuthorID:   operator.StudentID,
		AuthorName: operator.Name,
		Content:    content,
	}
	if err := s.repos.Discussion.Create(ctx, discussion); err != nil {
		return nil, apperr.Internal(err)
	}
	dto := ToDiscussionDTO(discussion, operator)
	return &dto, nil
}

func (s *projectService) UpdateDiscussion(ctx context.Context, operator *model.User, discussionID uint, in CreateDiscussionInput) (*DiscussionDTO, error) {
	discussion, err := s.repos.Discussion.GetByID(ctx, discussionID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if discussion == nil {
		return nil, apperr.NotFound("讨论不存在")
	}
	if !canModifyDiscussion(operator, discussion) {
		return nil, apperr.ErrForbidden
	}

	c := s.checker()
	content := c.Discussion("讨论内容", in.Content)
	if c.HasError() {
		return nil, apperr.Validation(c.Err())
	}
	if err := s.repos.Discussion.Update(ctx, discussionID, content); err != nil {
		return nil, apperr.Internal(err)
	}
	discussion.Content = content
	dto := ToDiscussionDTO(discussion, operator)
	return &dto, nil
}

func (s *projectService) DeleteDiscussion(ctx context.Context, operator *model.User, discussionID uint) error {
	discussion, err := s.repos.Discussion.GetByID(ctx, discussionID)
	if err != nil {
		return apperr.Internal(err)
	}
	if discussion == nil {
		return apperr.NotFound("讨论不存在")
	}
	if !canModifyDiscussion(operator, discussion) {
		return apperr.ErrForbidden
	}
	if err := s.repos.Discussion.Delete(ctx, discussionID); err != nil {
		return apperr.Internal(err)
	}
	return nil
}

func (s *projectService) assemble(ctx context.Context, viewer *model.User, project *model.Project, withDiscussions bool) (*ProjectDTO, error) {
	members, err := s.repos.Member.ListByProject(ctx, project.ID)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	memberMap := make(map[uint][]model.ProjectMember, len(members))
	myAssignmentIDs := make([]uint, 0, 1)
	for _, m := range members {
		memberMap[m.AssignmentID] = append(memberMap[m.AssignmentID], m)
		if m.StudentID == viewer.StudentID {
			myAssignmentIDs = append(myAssignmentIDs, m.AssignmentID)
		}
	}

	canSeeMembers := viewer.Role.CanManage() || len(myAssignmentIDs) > 0

	totalCapacity, totalClaimed := 0, 0
	assignments := make([]AssignmentDTO, 0, len(project.Assignments))
	for _, a := range project.Assignments {
		claimed := len(memberMap[a.ID])
		dto := AssignmentDTO{
			ID:           a.ID,
			Name:         a.Name,
			Description:  a.Description,
			TagID:        a.TagID,
			Capacity:     a.Capacity,
			ClaimedCount: claimed,
			Remaining:    maxInt(a.Capacity-claimed, 0),
			Full:         claimed >= a.Capacity,
			Deadline:     a.Deadline,
		}
		for _, m := range memberMap[a.ID] {
			if m.StudentID == viewer.StudentID {
				dto.Mine = true
			}
			if canSeeMembers {
				dto.Members = append(dto.Members, MemberDTO{
					StudentID:   m.StudentID,
					StudentName: m.StudentName,
					Assigned:    m.Assigned,
					AssignedBy:  m.AssignedBy,
					CreatedAt:   m.CreatedAt,
				})
			}
		}
		totalCapacity += a.Capacity
		totalClaimed += claimed
		assignments = append(assignments, dto)
	}

	discussionCount, err := s.repos.Discussion.CountByProject(ctx, project.ID)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	dto := &ProjectDTO{
		ID:              project.ID,
		Name:            project.Name,
		Description:     project.Description,
		Deadline:        project.Deadline,
		Status:          project.Status,
		StatusName:      project.Status.String(),
		CreatorID:       project.CreatorID,
		CreatorName:     project.CreatorName,
		CreatedAt:       project.CreatedAt,
		UpdatedAt:       project.UpdatedAt,
		Semesters:       ToSemesterDTOs(project.Semesters),
		Tags:            ToTagDTOs(project.Tags),
		Assignments:     assignments,
		DiscussionCount: discussionCount,
		TotalCapacity:   totalCapacity,
		TotalClaimed:    totalClaimed,
		Mine:            len(myAssignmentIDs) > 0,
		MyAssignmentIDs: myAssignmentIDs,
	}

	if withDiscussions {
		discussions, err := s.repos.Discussion.ListByProject(ctx, project.ID)
		if err != nil {
			return nil, apperr.Internal(err)
		}
		dto.Discussions = make([]DiscussionDTO, 0, len(discussions))
		for i := range discussions {
			dto.Discussions = append(dto.Discussions, ToDiscussionDTO(&discussions[i], viewer))
		}
	}
	return dto, nil
}

func (s *projectService) isParticipant(ctx context.Context, projectID uint, studentID string) (bool, error) {
	count, err := s.repos.Member.CountByProject(ctx, projectID, studentID)
	if err != nil {
		return false, apperr.Internal(err)
	}
	return count > 0, nil
}

func ToDiscussionDTO(d *model.Discussion, viewer *model.User) DiscussionDTO {
	return DiscussionDTO{
		ID:         d.ID,
		AuthorID:   d.AuthorID,
		AuthorName: d.AuthorName,
		Content:    d.Content,
		CreatedAt:  d.CreatedAt,
		UpdatedAt:  d.UpdatedAt,
		Editable:   canModifyDiscussion(viewer, d),
	}
}

func canModifyDiscussion(viewer *model.User, d *model.Discussion) bool {
	if viewer == nil {
		return false
	}
	return viewer.Role.CanManage() || viewer.StudentID == d.AuthorID
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
