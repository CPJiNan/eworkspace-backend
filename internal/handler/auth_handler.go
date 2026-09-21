package handler

import (
	"github.com/gin-gonic/gin"

	"eworkspace/internal/pkg/apperr"
	"eworkspace/internal/service"
)

type AuthHandler struct {
	base
}

type loginRequest struct {
	StudentID string `json:"studentId"`
	Password  string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type changePasswordRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := bindJSON(c, &req); err != nil {
		Fail(c, err)
		return
	}
	if req.StudentID == "" || req.Password == "" {
		Fail(c, apperr.Validation("学号与密码不能为空"))
		return
	}

	result, err := h.svc.Auth.Login(c.Request.Context(), service.LoginInput{
		StudentID: req.StudentID,
		Password:  req.Password,
	})
	if err != nil {
		Fail(c, err)
		return
	}

	OK(c, result)
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req refreshRequest
	if err := bindJSON(c, &req); err != nil {
		Fail(c, err)
		return
	}
	result, err := h.svc.Auth.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, result)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	var req refreshRequest
	if err := bindJSON(c, &req); err != nil {
		Fail(c, err)
		return
	}
	if err := h.svc.Auth.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		Fail(c, err)
		return
	}
	OK(c, gin.H{"message": "已登出"})
}

func (h *AuthHandler) Me(c *gin.Context) {
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

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	u, err := mustUser(c)
	if err != nil {
		Fail(c, err)
		return
	}
	var req changePasswordRequest
	if err := bindJSON(c, &req); err != nil {
		Fail(c, err)
		return
	}
	if err := h.svc.Auth.ChangePassword(c.Request.Context(), u, req.OldPassword, req.NewPassword); err != nil {
		Fail(c, err)
		return
	}
	OK(c, gin.H{"message": "密码修改成功，请重新登录"})
}
