package service

import (
	"context"
	"time"

	"eworkspace/internal/model"
	"eworkspace/internal/pkg/apperr"
	"eworkspace/internal/pkg/validate"
	"eworkspace/internal/repository"
)

type UserDTO struct {
	StudentID          string     `json:"studentId"`
	Name               string     `json:"name"`
	Phone              string     `json:"phone,omitempty"`
	WeChat             string     `json:"wechat,omitempty"`
	QQ                 string     `json:"qq,omitempty"`
	Email              string     `json:"email,omitempty"`
	Role               model.Role `json:"role"`
	RoleName           string     `json:"roleName"`
	MustChangePassword bool       `json:"mustChangePassword"`
	Banned             bool       `json:"banned"`
	BannedAt           *time.Time `json:"bannedAt,omitempty"`
	CreatedAt          time.Time  `json:"createdAt"`

	Masked bool `json:"masked"`
}

func ToUserDTO(u *model.User, mask bool) UserDTO {
	dto := UserDTO{
		StudentID:          u.StudentID,
		Name:               u.Name,
		Role:               u.Role,
		RoleName:           u.Role.String(),
		MustChangePassword: u.MustChangePassword,
		Banned:             u.Banned,
		BannedAt:           u.BannedAt,
		CreatedAt:          u.CreatedAt,
		Masked:             mask,
	}
	if !mask {
		dto.Phone = u.Phone
		dto.WeChat = u.WeChat
		dto.QQ = u.QQ
		dto.Email = u.Email
	}
	return dto
}

type CreateAccountInput struct {
	StudentIDs []string
	Name       string
	Phone      string
	WeChat     string
	QQ         string
	Email      string
	Role       *model.Role
}

type CreateAccountResult struct {
	StudentID     string     `json:"studentId"`
	Name          string     `json:"name"`
	Role          model.Role `json:"role"`
	RoleName      string     `json:"roleName"`
	Email         string     `json:"email"`
	InitialPasswd string     `json:"initialPassword"`
	Created       bool       `json:"created"`
	Error         string     `json:"error,omitempty"`
}

type UpdateUserInput struct {
	Name   *string
	Phone  *string
	WeChat *string
	QQ     *string
	Email  *string
}

type ListUsersInput struct {
	Keyword string
	Role    *model.Role
	Banned  *bool
	Page    int
	Size    int
}

type UserService interface {
	List(ctx context.Context, viewer *model.User, in ListUsersInput) ([]UserDTO, PageMeta, error)
	CreateAccounts(ctx context.Context, viewer *model.User, in CreateAccountInput) ([]CreateAccountResult, error)
	Get(ctx context.Context, viewer *model.User, studentID string) (*UserDTO, error)
	UpdateProfile(ctx context.Context, target *model.User, in UpdateUserInput) (*UserDTO, error)
	SetBanned(ctx context.Context, viewer *model.User, studentID string, banned bool) error
	ResetPassword(ctx context.Context, studentID, newPassword string) (string, error)
	DeleteAccount(ctx context.Context, viewer *model.User, studentID string) error
}

type userService struct {
	meta   *meta
	repos  *repository.Repositories
	log    Logger
	hasher passwordHasher
}

type passwordHasher interface {
	Hash(plain string) (string, error)
	Verify(hash, plain string) bool
}

func NewUserService(m *meta, repos *repository.Repositories, log Logger, hasher passwordHasher) UserService {
	return &userService{meta: m, repos: repos, log: log, hasher: hasher}
}

func (s *userService) List(ctx context.Context, viewer *model.User, in ListUsersInput) ([]UserDTO, PageMeta, error) {
	if !viewer.Role.CanManage() {
		return nil, PageMeta{}, apperr.ErrForbidden
	}
	users, total, err := s.repos.User.List(ctx, repository.UserFilter{
		Keyword: in.Keyword,
		Role:    in.Role,
		Banned:  in.Banned,
		Page:    in.Page,
		Size:    in.Size,
	})
	if err != nil {
		return nil, PageMeta{}, apperr.Internal(err)
	}

	out := make([]UserDTO, 0, len(users))
	for i := range users {
		out = append(out, ToUserDTO(&users[i], false))
	}
	return out, NewPageMeta(in.Page, in.Size, total), nil
}

func DefaultPasswordFor(studentID string) string {
	return model.DefaultPasswordFor(studentID)
}

func (s *userService) CreateAccounts(ctx context.Context, viewer *model.User,
	in CreateAccountInput) ([]CreateAccountResult, error) {
	if !viewer.Role.CanManage() {
		return nil, apperr.ErrForbidden
	}

	role := model.RoleUser
	if in.Role != nil {
		role = *in.Role
		if !role.IsValid() {
			return nil, apperr.Validation("角色取值不合法")
		}
		if role == model.RoleAdmin && viewer.Role != model.RoleSuperAdmin {
			return nil, apperr.ErrSuperAdminOnly
		}
		if role == model.RoleSuperAdmin {
			return nil, apperr.Validation("超级管理员为系统内置账号，不允许创建")
		}
	}

	c := validate.New()
	name := c.ShortOptional("姓名", in.Name)
	phone := c.ShortOptional("手机号", in.Phone)
	wechat := c.ShortOptional("微信号", in.WeChat)
	qq := c.Digits("QQ 号", in.QQ, 20)
	email := c.Email("邮箱", in.Email)
	if len(in.StudentIDs) == 0 {
		c.Fail("学号列表不能为空")
	}
	if len(in.StudentIDs) > 100 {
		c.Fail("单次最多新增 100 个账号")
	}
	if c.HasError() {
		return nil, apperr.Validation(c.Err())
	}

	results := make([]CreateAccountResult, 0, len(in.StudentIDs))
	for _, raw := range in.StudentIDs {
		id := raw
		if !isDigits(id) || len(id) > 20 {
			results = append(results, CreateAccountResult{
				StudentID: raw,
				Created:   false,
				Error:     "学号必须是 11 位数字",
			})
			continue
		}
		if id == s.meta.SuperAdminStudentID {
			results = append(results, CreateAccountResult{
				StudentID: id,
				Created:   false,
				Error:     "该学号为系统内置超级管理员",
			})
			continue
		}

		initialPassword := DefaultPasswordFor(id)
		hash, err := s.hasher.Hash(initialPassword)
		if err != nil {
			return nil, apperr.Internal(err)
		}

		exists, err := s.repos.User.Exists(ctx, id)
		if err != nil {
			return nil, apperr.Internal(err)
		}

		u := &model.User{
			StudentID:          id,
			Name:               firstNonEmpty(name, "未命名用户"),
			Phone:              phone,
			WeChat:             wechat,
			QQ:                 qq,
			Email:              firstNonEmpty(email, model.DefaultEmail(id)),
			PasswordHash:       hash,
			Role:               role,
			MustChangePassword: true,
		}

		if !exists {
			deleted, err := s.repos.User.GetByIDIncludingDeleted(ctx, id)
			if err != nil {
				return nil, apperr.Internal(err)
			}
			if deleted != nil {
				if err := s.repos.User.Restore(ctx, u); err != nil {
					return nil, apperr.Internal(err)
				}
				if err := s.repos.RefreshToken.RevokeByUser(ctx, id); err != nil {
					s.log.Warn("恢复账号时撤销 Refresh Token 失败", "studentId", id, "err", err)
				}
				results = append(results, CreateAccountResult{
					StudentID:     id,
					Name:          u.Name,
					Role:          u.Role,
					RoleName:      u.Role.String(),
					Email:         u.Email,
					InitialPasswd: initialPassword,
					Created:       true,
				})
				continue
			}
		}

		if exists {
			results = append(results, CreateAccountResult{
				StudentID: id,
				Created:   false,
				Error:     "学号已存在",
			})
			continue
		}

		if err := s.repos.User.Create(ctx, u); err != nil {
			results = append(results, CreateAccountResult{
				StudentID: id,
				Created:   false,
				Error:     "写入失败",
			})
			continue
		}
		results = append(results, CreateAccountResult{
			StudentID:     id,
			Name:          u.Name,
			Role:          u.Role,
			RoleName:      u.Role.String(),
			Email:         u.Email,
			InitialPasswd: initialPassword,
			Created:       true,
		})
	}
	return results, nil
}

func (s *userService) Get(ctx context.Context, viewer *model.User, studentID string) (*UserDTO, error) {
	u, err := s.repos.User.GetByID(ctx, studentID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if u == nil {
		return nil, apperr.NotFound("账号不存在")
	}
	mask := !viewer.Role.CanManage() && viewer.StudentID != u.StudentID
	dto := ToUserDTO(u, mask)
	return &dto, nil
}

func (s *userService) UpdateProfile(ctx context.Context, target *model.User, in UpdateUserInput) (*UserDTO, error) {
	current, err := s.repos.User.GetByID(ctx, target.StudentID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if current == nil {
		return nil, apperr.NotFound("账号不存在")
	}

	c := validate.New()
	fields := map[string]any{}

	if in.Name != nil {
		fields["name"] = c.Short("姓名", *in.Name)
	}
	if in.Phone != nil {
		fields["phone"] = c.ShortOptional("手机号", *in.Phone)
	}
	if in.WeChat != nil {
		fields["wechat"] = c.ShortOptional("微信号", *in.WeChat)
	}
	if in.QQ != nil {
		fields["qq"] = c.Digits("QQ 号", *in.QQ, 20)
	}
	if in.Email != nil {
		email := c.Email("邮箱", *in.Email)
		if email == "" {
			email = model.DefaultEmail(current.StudentID)
		}
		fields["email"] = email
	}
	if c.HasError() {
		return nil, apperr.Validation(c.Err())
	}
	if len(fields) == 0 {
		return nil, apperr.Validation("没有需要修改的字段")
	}

	if err := s.repos.User.UpdateFields(ctx, current.StudentID, fields); err != nil {
		return nil, apperr.Internal(err)
	}
	updated, err := s.repos.User.GetByID(ctx, current.StudentID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	dto := ToUserDTO(updated, false)
	return &dto, nil
}

func (s *userService) SetBanned(ctx context.Context, viewer *model.User, studentID string, banned bool) error {
	if !viewer.Role.CanManage() {
		return apperr.ErrForbidden
	}
	if studentID == s.meta.SuperAdminStudentID {
		return apperr.ErrLastSuperAdmin
	}
	target, err := s.repos.User.GetByID(ctx, studentID)
	if err != nil {
		return apperr.Internal(err)
	}
	if target == nil {
		return apperr.NotFound("账号不存在")
	}
	if target.Role == model.RoleAdmin && viewer.Role != model.RoleSuperAdmin {
		return apperr.ErrSuperAdminOnly
	}
	if target.Role == model.RoleSuperAdmin {
		return apperr.ErrLastSuperAdmin
	}

	fields := map[string]any{"banned": banned, "banned_at": nil}
	if banned {
		fields["banned_at"] = time.Now()
	}
	if err := s.repos.User.UpdateFields(ctx, studentID, fields); err != nil {
		return apperr.Internal(err)
	}
	if banned {
		if err := s.repos.RefreshToken.RevokeByUser(ctx, studentID); err != nil {
			s.log.Warn("撤销被封禁账号的 Refresh Token 失败", "studentId", studentID, "err", err)
		}
	}
	return nil
}

func (s *userService) ResetPassword(ctx context.Context, studentID, newPassword string) (string, error) {
	c := validate.New()
	if newPassword == "" {
		newPassword = DefaultPasswordFor(studentID)
	}
	c.Password("密码", newPassword)
	if c.HasError() {
		return "", apperr.Validation(c.Err())
	}

	hash, err := s.hasher.Hash(newPassword)
	if err != nil {
		return "", apperr.Internal(err)
	}
	if err := s.repos.User.UpdateFields(ctx, studentID, map[string]any{
		"password_hash":        hash,
		"must_change_password": true,
	}); err != nil {
		return "", apperr.Internal(err)
	}
	if err := s.repos.RefreshToken.RevokeByUser(ctx, studentID); err != nil {
		s.log.Warn("重置密码后撤销 Refresh Token 失败", "studentId", studentID, "err", err)
	}
	return newPassword, nil
}

func (s *userService) DeleteAccount(ctx context.Context, viewer *model.User, studentID string) error {
	if !viewer.Role.CanManage() {
		return apperr.ErrForbidden
	}

	target, err := s.repos.User.GetByID(ctx, studentID)
	if err != nil {
		return apperr.Internal(err)
	}
	if target == nil {
		return apperr.NotFound("账号不存在")
	}
	if target.Role == model.RoleSuperAdmin {
		return apperr.ErrLastSuperAdmin
	}
	if target.StudentID == viewer.StudentID {
		return apperr.ErrCannotDeleteSelf
	}
	if target.Role == model.RoleAdmin && viewer.Role != model.RoleSuperAdmin {
		return apperr.ErrSuperAdminOnly
	}

	if err := s.repos.RefreshToken.RevokeByUser(ctx, studentID); err != nil {
		s.log.Warn("删除账号时撤销 Refresh Token 失败", "studentId", studentID, "err", err)
	}
	if err := s.repos.User.Delete(ctx, studentID); err != nil {
		return apperr.Internal(err)
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
