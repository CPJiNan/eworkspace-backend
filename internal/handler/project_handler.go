package handler

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"eworkspace/internal/model"
	"eworkspace/internal/pkg/apperr"
	"eworkspace/internal/service"
)

type ProjectHandler struct {
	base
}

type assignmentPayload struct {
	ID          *uint   `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Capacity    int     `json:"capacity"`
	Workload    *int    `json:"workload"`
	TagID       *uint   `json:"tagId"`
	Deadline    *string `json:"deadline"`
}

type createProjectRequest struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Deadline    string              `json:"deadline"`
	SemesterIDs []uint              `json:"semesterIds"`
	TagIDs      []uint              `json:"tagIds"`
	Assignments []assignmentPayload `json:"assignments"`
}

type updateProjectRequest struct {
	Name        *string              `json:"name"`
	Description *string              `json:"description"`
	Deadline    *string              `json:"deadline"`
	Status      *model.ProjectStatus `json:"status"`
	SemesterIDs *[]uint              `json:"semesterIds"`
	TagIDs      *[]uint              `json:"tagIds"`
	Assignments *[]assignmentPayload `json:"assignments"`
}

type setStatusRequest struct {
	Status model.ProjectStatus `json:"status"`
}

type deleteProjectRequest struct {
	Confirm bool   `json:"confirm"`
	Reason  string `json:"reason"`
}

type discussionRequest struct {
	Content string `json:"content"`
}

type assignMembersRequest struct {
	StudentIDs []string `json:"studentIds"`
}

func (h *ProjectHandler) List(c *gin.Context) {
	actor, err := mustUser(c)
	if err != nil {
		Fail(c, err)
		return
	}

	semesterID, err := uintQueryOptional(c, "semesterId")
	if err != nil {
		Fail(c, err)
		return
	}
	tagID, err := uintQueryOptional(c, "tagId")
	if err != nil {
		Fail(c, err)
		return
	}

	var status *model.ProjectStatus
	if raw := c.Query("status"); raw != "" {
		s := model.ProjectStatus(raw)
		if !s.IsValid() {
			Fail(c, apperr.Validation("状态筛选参数不合法"))
			return
		}
		status = &s
	}

	from, err := parseTimeQuery(c, "deadlineFrom", false)
	if err != nil {
		Fail(c, err)
		return
	}
	to, err := parseTimeQuery(c, "deadlineTo", true)
	if err != nil {
		Fail(c, err)
		return
	}

	page, size := pageQuery(c)
	items, meta, err := h.svc.Project.List(c.Request.Context(), actor, service.ListProjectsInput{
		Keyword:      c.Query("keyword"),
		SemesterID:   semesterID,
		TagID:        tagID,
		Status:       status,
		DeadlineFrom: from,
		DeadlineTo:   to,
		IncludeAll:   boolQuery(c, "includeAll"),
		Page:         page,
		Size:         size,
	})
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, NewPageResult(items, meta))
}

func (h *ProjectHandler) Get(c *gin.Context) {
	actor, err := mustUser(c)
	if err != nil {
		Fail(c, err)
		return
	}
	id, err := uintParam(c, "id")
	if err != nil {
		Fail(c, err)
		return
	}
	dto, err := h.svc.Project.Get(c.Request.Context(), actor, id)
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, dto)
}

func (h *ProjectHandler) Create(c *gin.Context) {
	actor, err := mustUser(c)
	if err != nil {
		Fail(c, err)
		return
	}
	var req createProjectRequest
	if err := bindJSON(c, &req); err != nil {
		Fail(c, err)
		return
	}

	deadline, err := parseTime(req.Deadline, false)
	if err != nil {
		Fail(c, err)
		return
	}

	assignments := make([]service.AssignmentInput, 0, len(req.Assignments))
	for _, a := range req.Assignments {
		deadline, err := parseOptionalTime(a.Deadline)
		if err != nil {
			Fail(c, err)
			return
		}
		assignments = append(assignments, service.AssignmentInput{
			ID:          a.ID,
			Name:        a.Name,
			Description: a.Description,
			Capacity:    a.Capacity,
			Workload:    workloadOf(a.Workload),
			TagID:       a.TagID,
			Deadline:    deadline,
		})
	}

	dto, err := h.svc.Project.Create(c.Request.Context(), actor, service.CreateProjectInput{
		Name:        req.Name,
		Description: req.Description,
		Deadline:    deadline,
		SemesterIDs: req.SemesterIDs,
		TagIDs:      req.TagIDs,
		Assignments: assignments,
	})
	if err != nil {
		Fail(c, err)
		return
	}
	h.writeLog(c, actor, model.OpCreateProject, model.TargetProject, formatUint(dto.ID), dto.Name, "")
	Created(c, dto)
}

func (h *ProjectHandler) Update(c *gin.Context) {
	actor, err := mustUser(c)
	if err != nil {
		Fail(c, err)
		return
	}
	id, err := uintParam(c, "id")
	if err != nil {
		Fail(c, err)
		return
	}

	var req updateProjectRequest
	if err := bindJSON(c, &req); err != nil {
		Fail(c, err)
		return
	}

	in := service.UpdateProjectInput{
		Name:        req.Name,
		Description: req.Description,
		Status:      req.Status,
		SemesterIDs: req.SemesterIDs,
		TagIDs:      req.TagIDs,
	}
	if req.Deadline != nil {
		deadline, err := parseTime(*req.Deadline, false)
		if err != nil {
			Fail(c, err)
			return
		}
		in.Deadline = &deadline
	}
	if req.Assignments != nil {
		in.SyncAssignments = true
		in.Assignments = make([]service.AssignmentInput, 0, len(*req.Assignments))
		for _, a := range *req.Assignments {
			deadline, err := parseOptionalTime(a.Deadline)
			if err != nil {
				Fail(c, err)
				return
			}
			in.Assignments = append(in.Assignments, service.AssignmentInput{
				ID:          a.ID,
				Name:        a.Name,
				Description: a.Description,
				Capacity:    a.Capacity,
				Workload:    workloadOf(a.Workload),
				TagID:       a.TagID,
				Deadline:    deadline,
			})
		}
	}

	dto, err := h.svc.Project.Update(c.Request.Context(), actor, id, in)
	if err != nil {
		Fail(c, err)
		return
	}
	h.writeLog(c, actor, model.OpUpdateProject, model.TargetProject, formatUint(id), dto.Name, "")
	OK(c, dto)
}

func (h *ProjectHandler) SetStatus(c *gin.Context) {
	actor, err := mustUser(c)
	if err != nil {
		Fail(c, err)
		return
	}
	id, err := uintParam(c, "id")
	if err != nil {
		Fail(c, err)
		return
	}
	var req setStatusRequest
	if err := bindJSON(c, &req); err != nil {
		Fail(c, err)
		return
	}

	if err := h.svc.Project.SetStatus(c.Request.Context(), actor, id, req.Status); err != nil {
		Fail(c, err)
		return
	}
	h.writeLog(c, actor, model.OpUpdateProject, model.TargetProject, formatUint(id), "",
		"状态切换为"+req.Status.String())
	OK(c, gin.H{"id": id, "status": req.Status})
}

func (h *ProjectHandler) Delete(c *gin.Context) {
	actor, err := mustUser(c)
	if err != nil {
		Fail(c, err)
		return
	}
	id, err := uintParam(c, "id")
	if err != nil {
		Fail(c, err)
		return
	}

	var req deleteProjectRequest
	if err := bindJSON(c, &req); err != nil {
		Fail(c, err)
		return
	}
	if !req.Confirm {
		Fail(c, apperr.Validation("请先二次确认删除操作"))
		return
	}

	dto, err := h.svc.Project.Get(c.Request.Context(), actor, id)
	if err != nil {
		Fail(c, err)
		return
	}
	if err := h.svc.Project.Delete(c.Request.Context(), actor, id, req.Reason); err != nil {
		Fail(c, err)
		return
	}
	h.writeLog(c, actor, model.OpDeleteProject, model.TargetProject, formatUint(id), dto.Name, "删除原因："+req.Reason)
	OK(c, gin.H{"id": id, "deleted": true})
}

func (h *ProjectHandler) Claim(c *gin.Context) {
	actor, err := mustUser(c)
	if err != nil {
		Fail(c, err)
		return
	}
	assignmentID, err := uintParam(c, "id")
	if err != nil {
		Fail(c, err)
		return
	}

	dto, err := h.svc.Member.Claim(c.Request.Context(), actor, assignmentID)
	if err != nil {
		Fail(c, err)
		return
	}
	h.writeLog(c, actor, model.OpClaimAssignment, model.TargetAssignment, formatUint(assignmentID), dto.Name, "")
	OK(c, dto)
}

func (h *ProjectHandler) CancelClaim(c *gin.Context) {
	actor, err := mustUser(c)
	if err != nil {
		Fail(c, err)
		return
	}
	assignmentID, err := uintParam(c, "id")
	if err != nil {
		Fail(c, err)
		return
	}

	dto, err := h.svc.Member.CancelClaim(c.Request.Context(), actor, assignmentID)
	if err != nil {
		Fail(c, err)
		return
	}
	h.writeLog(c, actor, model.OpCancelAssignment, model.TargetAssignment, formatUint(assignmentID), dto.Name, "")
	OK(c, dto)
}

func (h *ProjectHandler) Assign(c *gin.Context) {
	actor, err := mustUser(c)
	if err != nil {
		Fail(c, err)
		return
	}
	assignmentID, err := uintParam(c, "id")
	if err != nil {
		Fail(c, err)
		return
	}
	var req assignMembersRequest
	if err := bindJSON(c, &req); err != nil {
		Fail(c, err)
		return
	}

	members, err := h.svc.Member.Assign(c.Request.Context(), actor, assignmentID, service.AssignInput{
		StudentIDs: req.StudentIDs,
	})
	if err != nil {
		Fail(c, err)
		return
	}
	for _, m := range members {
		h.writeLog(c, actor, model.OpAssignAssignment, model.TargetAssignment,
			formatUint(assignmentID), m.StudentName, "指派 "+m.StudentID)
	}
	OK(c, gin.H{"members": members})
}

func (h *ProjectHandler) Members(c *gin.Context) {
	actor, err := mustUser(c)
	if err != nil {
		Fail(c, err)
		return
	}
	assignmentID, err := uintParam(c, "id")
	if err != nil {
		Fail(c, err)
		return
	}
	members, err := h.svc.Member.AssignmentMembers(c.Request.Context(), actor, assignmentID)
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, gin.H{"members": members})
}

func (h *ProjectHandler) MyTasks(c *gin.Context) {
	actor, err := mustUser(c)
	if err != nil {
		Fail(c, err)
		return
	}
	page, size := pageQuery(c)
	items, meta, err := h.svc.Member.MyTasks(c.Request.Context(), actor, service.MyTasksInput{
		Keyword: c.Query("keyword"),
		Page:    page,
		Size:    size,
	})
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, NewPageResult(items, meta))
}

func (h *ProjectHandler) WorkloadRanking(c *gin.Context) {
	actor, err := mustUser(c)
	if err != nil {
		Fail(c, err)
		return
	}
	semesterIDs, err := uintList(c, "semesterIds")
	if err != nil {
		Fail(c, err)
		return
	}
	items, err := h.svc.Member.WorkloadRanking(c.Request.Context(), actor, semesterIDs)
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, gin.H{"items": items})
}

func (h *ProjectHandler) AddDiscussion(c *gin.Context) {
	actor, err := mustUser(c)
	if err != nil {
		Fail(c, err)
		return
	}
	projectID, err := uintParam(c, "id")
	if err != nil {
		Fail(c, err)
		return
	}
	var req discussionRequest
	if err := bindJSON(c, &req); err != nil {
		Fail(c, err)
		return
	}

	dto, err := h.svc.Project.AddDiscussion(c.Request.Context(), actor, projectID, service.CreateDiscussionInput{
		Content: req.Content,
	})
	if err != nil {
		Fail(c, err)
		return
	}
	Created(c, dto)
}

func (h *ProjectHandler) UpdateDiscussion(c *gin.Context) {
	actor, err := mustUser(c)
	if err != nil {
		Fail(c, err)
		return
	}
	discussionID, err := uintParam(c, "id")
	if err != nil {
		Fail(c, err)
		return
	}
	var req discussionRequest
	if err := bindJSON(c, &req); err != nil {
		Fail(c, err)
		return
	}

	dto, err := h.svc.Project.UpdateDiscussion(c.Request.Context(), actor, discussionID, service.CreateDiscussionInput{
		Content: req.Content,
	})
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, dto)
}

func (h *ProjectHandler) DeleteDiscussion(c *gin.Context) {
	actor, err := mustUser(c)
	if err != nil {
		Fail(c, err)
		return
	}
	discussionID, err := uintParam(c, "id")
	if err != nil {
		Fail(c, err)
		return
	}
	if err := h.svc.Project.DeleteDiscussion(c.Request.Context(), actor, discussionID); err != nil {
		Fail(c, err)
		return
	}
	OK(c, gin.H{"id": discussionID, "deleted": true})
}

func workloadOf(raw *int) int {
	if raw == nil {
		return 1
	}
	return *raw
}

func parseTimeQuery(c *gin.Context, name string, endOfDay bool) (*time.Time, error) {
	raw := c.Query(name)
	if raw == "" {
		return nil, nil
	}
	t, err := parseTime(raw, endOfDay)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func parseOptionalTime(raw *string) (*time.Time, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, nil
	}
	t, err := parseTime(*raw, false)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func parseTime(raw string, endOfDay bool) (time.Time, error) {
	if raw == "" {
		return time.Time{}, apperr.Validation("时间不能为空")
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
		t, err := time.ParseInLocation(layout, raw, time.Local)
		if err != nil {
			continue
		}
		if layout == "2006-01-02" && endOfDay {
			t = t.Add(24*time.Hour - time.Second)
		}
		return t, nil
	}
	return time.Time{}, apperr.Validation("时间格式不正确，应为 RFC3339 或 2006-01-02")
}
