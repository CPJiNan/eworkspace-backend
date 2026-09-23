package handler

import (
	"github.com/gin-gonic/gin"

	"eworkspace/internal/model"
	"eworkspace/internal/pkg/apperr"
	"eworkspace/internal/service"
)

type UserHandler struct {
	base
}

type updateProfileRequest struct {
	Name   *string `json:"name"`
	Phone  *string `json:"phone"`
	WeChat *string `json:"wechat"`
	QQ     *string `json:"qq"`
	Email  *string `json:"email"`
}

type updateMemberRequest struct {
	Name   *string     `json:"name"`
	Phone  *string     `json:"phone"`
	WeChat *string     `json:"wechat"`
	QQ     *string     `json:"qq"`
	Email  *string     `json:"email"`
	Role   *model.Role `json:"role"`
}

type createAccountsRequest struct {
	StudentIDs []string    `json:"studentIds"`
	Name       string      `json:"name"`
	Phone      string      `json:"phone"`
	WeChat     string      `json:"wechat"`
	QQ         string      `json:"qq"`
	Email      string      `json:"email"`
	Role       *model.Role `json:"role"`
}

type banRequest struct {
	Banned bool `json:"banned"`
}

type resetPasswordRequest struct {
	NewPassword string `json:"newPassword"`
}

func (h *UserHandler) Me(c *gin.Context) {
	u, err := mustUser(c)
	if err != nil {
		Fail(c, err)
		return
	}
	dto, err := h.svc.User.Get(c.Request.Context(), u, u.StudentID)
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, dto)
}

func (h *UserHandler) UpdateMe(c *gin.Context) {
	u, err := mustUser(c)
	if err != nil {
		Fail(c, err)
		return
	}
	var req updateProfileRequest
	if err := bindJSON(c, &req); err != nil {
		Fail(c, err)
		return
	}

	dto, err := h.svc.User.UpdateProfile(c.Request.Context(), u, service.UpdateUserInput{
		Name:   req.Name,
		Phone:  req.Phone,
		WeChat: req.WeChat,
		QQ:     req.QQ,
		Email:  req.Email,
	})
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, dto)
}

func (h *UserHandler) ListMembers(c *gin.Context) {
	actor, err := mustUser(c)
	if err != nil {
		Fail(c, err)
		return
	}

	var role *model.Role
	if v, err := intQueryOptional(c, "role"); err != nil {
		Fail(c, err)
		return
	} else if v != nil {
		r := model.Role(*v)
		if !r.IsValid() {
			Fail(c, apperr.Validation("角色参数不合法"))
			return
		}
		role = &r
	}

	var banned *bool
	if c.Query("banned") != "" {
		v := boolQuery(c, "banned")
		banned = &v
	}

	page, size := pageQuery(c)
	items, meta, err := h.svc.User.List(c.Request.Context(), actor, service.ListUsersInput{
		Keyword: c.Query("keyword"),
		Role:    role,
		Banned:  banned,
		Page:    page,
		Size:    size,
	})
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, NewPageResult(items, meta))
}

func (h *UserHandler) GetMember(c *gin.Context) {
	actor, err := mustUser(c)
	if err != nil {
		Fail(c, err)
		return
	}
	studentID := c.Param("studentId")
	dto, err := h.svc.User.Get(c.Request.Context(), actor, studentID)
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, dto)
}

func (h *UserHandler) CreateAccounts(c *gin.Context) {
	actor, err := mustUser(c)
	if err != nil {
		Fail(c, err)
		return
	}
	var req createAccountsRequest
	if err := bindJSON(c, &req); err != nil {
		Fail(c, err)
		return
	}

	results, err := h.svc.User.CreateAccounts(c.Request.Context(), actor, service.CreateAccountInput{
		StudentIDs: req.StudentIDs,
		Name:       req.Name,
		Phone:      req.Phone,
		WeChat:     req.WeChat,
		QQ:         req.QQ,
		Email:      req.Email,
		Role:       req.Role,
	})
	if err != nil {
		Fail(c, err)
		return
	}

	success, failed := 0, 0
	for _, r := range results {
		if r.Created {
			success++
			h.writeLog(c, actor, model.OpCreateAccount, model.TargetUser, r.StudentID, r.Name, "")
			if r.Role == model.RoleAdmin {
				h.writeLog(c, actor, model.OpCreateAdmin, model.TargetUser, r.StudentID, r.Name, "")
			}
		} else {
			failed++
		}
	}
	OK(c, gin.H{"results": results, "success": success, "failed": failed})
}

func (h *UserHandler) UpdateMember(c *gin.Context) {
	actor, err := mustUser(c)
	if err != nil {
		Fail(c, err)
		return
	}
	studentID := c.Param("studentId")

	var req updateMemberRequest
	if err := bindJSON(c, &req); err != nil {
		Fail(c, err)
		return
	}

	updated, roleChanged, err := h.svc.User.UpdateMember(c.Request.Context(), actor, studentID, service.UpdateUserInput{
		Name:   req.Name,
		Phone:  req.Phone,
		WeChat: req.WeChat,
		QQ:     req.QQ,
		Email:  req.Email,
		Role:   req.Role,
	})
	if err != nil {
		Fail(c, err)
		return
	}
	detail := ""
	if roleChanged {
		detail = "身份调整为" + updated.RoleName
	}
	h.writeLog(c, actor, model.OpUpdateMember, model.TargetUser, studentID, updated.Name, detail)
	OK(c, updated)
}

func (h *UserHandler) SetBanned(c *gin.Context) {
	actor, err := mustUser(c)
	if err != nil {
		Fail(c, err)
		return
	}
	studentID := c.Param("studentId")
	var req banRequest
	if err := bindJSON(c, &req); err != nil {
		Fail(c, err)
		return
	}

	if err := h.svc.User.SetBanned(c.Request.Context(), actor, studentID, req.Banned); err != nil {
		Fail(c, err)
		return
	}

	op := model.OpUnbanAccount
	detail := "解除封禁"
	if req.Banned {
		op = model.OpBanAccount
		detail = "封禁账号"
	}
	h.writeLog(c, actor, op, model.TargetUser, studentID, "", detail)
	OK(c, gin.H{"studentId": studentID, "banned": req.Banned})
}

func (h *UserHandler) ResetPassword(c *gin.Context) {
	actor, err := mustUser(c)
	if err != nil {
		Fail(c, err)
		return
	}
	studentID := c.Param("studentId")
	var req resetPasswordRequest
	if err := bindJSON(c, &req); err != nil {
		Fail(c, err)
		return
	}

	password, err := h.svc.User.ResetPassword(c.Request.Context(), studentID, req.NewPassword)
	if err != nil {
		Fail(c, err)
		return
	}
	h.writeLog(c, actor, model.OpResetPassword, model.TargetUser, studentID, "", "重置为默认密码并要求首次登录修改")
	OK(c, gin.H{"studentId": studentID, "initialPassword": password})
}

func (h *UserHandler) DeleteMember(c *gin.Context) {
	actor, err := mustUser(c)
	if err != nil {
		Fail(c, err)
		return
	}
	studentID := c.Param("studentId")

	target, err := h.svc.User.Get(c.Request.Context(), actor, studentID)
	if err != nil {
		Fail(c, err)
		return
	}

	if err := h.svc.User.DeleteAccount(c.Request.Context(), actor, studentID); err != nil {
		Fail(c, err)
		return
	}

	op := model.OpDeleteAdmin
	detail := "删除管理员账号"
	if target.Role != model.RoleAdmin {
		op = model.OpDeleteMember
		detail = "删除成员账号"
	}
	h.writeLog(c, actor, op, model.TargetUser, studentID, target.Name, detail)
	OK(c, gin.H{"studentId": studentID, "deleted": true})
}
