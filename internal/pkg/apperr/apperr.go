package apperr

import (
	"errors"
	"fmt"
	"net/http"
)

type Error struct {
	Status  int
	Code    string
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *Error) Unwrap() error { return e.Err }

func New(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

func Wrap(status int, code, message string, err error) *Error {
	return &Error{Status: status, Code: code, Message: message, Err: err}
}

var (
	ErrUnauthorized         = New(http.StatusUnauthorized, "UNAUTHORIZED", "未登录或登录状态已失效")
	ErrTokenInvalid         = New(http.StatusUnauthorized, "TOKEN_INVALID", "令牌无效或已过期")
	ErrTokenRevoked         = New(http.StatusUnauthorized, "TOKEN_REVOKED", "令牌已失效，请重新登录")
	ErrForbidden            = New(http.StatusForbidden, "FORBIDDEN", "没有权限执行该操作")
	ErrSuperAdminOnly       = New(http.StatusForbidden, "SUPER_ADMIN_ONLY", "该操作仅超级管理员可执行")
	ErrAccountBanned        = New(http.StatusForbidden, "ACCOUNT_BANNED", "账号已被封禁，无法登录")
	ErrBadCredentials       = New(http.StatusUnauthorized, "BAD_CREDENTIALS", "学号或密码错误")
	ErrPasswordUnchanged    = New(http.StatusBadRequest, "PASSWORD_UNCHANGED", "新密码不能与当前密码相同")
	ErrCapacityFull         = New(http.StatusConflict, "ASSIGNMENT_FULL", "该分工名额已满")
	ErrAlreadyClaimed       = New(http.StatusConflict, "ALREADY_CLAIMED", "已申领该分工")
	ErrNotClaimed           = New(http.StatusConflict, "NOT_CLAIMED", "未申领该分工")
	ErrInternal             = New(http.StatusInternalServerError, "INTERNAL_ERROR", "服务器内部错误")
	ErrProjectClosed        = New(http.StatusConflict, "PROJECT_CLOSED", "项目已结束或已取消，无法申领")
	ErrLastSuperAdmin       = New(http.StatusConflict, "LAST_SUPER_ADMIN", "超级管理员账号不可删除或封禁")
	ErrCannotDeleteSelf     = New(http.StatusConflict, "CANNOT_DELETE_SELF", "不能删除当前登录的账号")
	ErrCannotChangeSelfRole = New(http.StatusConflict, "CANNOT_CHANGE_SELF_ROLE", "不能修改当前登录账号的角色")
	ErrDuplicateName        = New(http.StatusConflict, "DUPLICATE_NAME", "名称已存在")
)

func Validation(msg string) *Error {
	return New(http.StatusBadRequest, "VALIDATION_FAILED", msg)
}

func NotFound(msg string) *Error {
	return New(http.StatusNotFound, "NOT_FOUND", msg)
}

func Conflict(msg string) *Error {
	return New(http.StatusConflict, "CONFLICT", msg)
}

func Internal(err error) *Error {
	return Wrap(http.StatusInternalServerError, "INTERNAL_ERROR", "服务器内部错误", err)
}

func From(err error) *Error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return Internal(err)
}
