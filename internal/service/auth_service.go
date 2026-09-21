package service

import (
	"context"
	"errors"
	"time"

	"eworkspace/internal/model"
	"eworkspace/internal/pkg/apperr"
	"eworkspace/internal/pkg/jwt"
	"eworkspace/internal/pkg/validate"
	"eworkspace/internal/repository"
)

type LoginInput struct {
	StudentID string
	Password  string
}

type AuthService interface {
	Login(ctx context.Context, in LoginInput) (*AuthResult, error)
	Refresh(ctx context.Context, refreshToken string) (*AuthResult, error)
	Logout(ctx context.Context, refreshToken string) error
	Authenticate(ctx context.Context, accessToken string) (*model.User, error)
	ChangePassword(ctx context.Context, u *model.User, oldPassword, newPassword string) error
}

type authService struct {
	meta   *meta
	repos  *repository.Repositories
	log    Logger
	tokens *jwt.Manager
	hasher passwordHasher
}

func NewAuthService(m *meta, repos *repository.Repositories, log Logger, tokens *jwt.Manager, hasher passwordHasher) AuthService {
	return &authService{meta: m, repos: repos, log: log, tokens: tokens, hasher: hasher}
}

func (s *authService) Login(ctx context.Context, in LoginInput) (*AuthResult, error) {
	c := validate.New()
	studentID := c.Required("学号", in.StudentID)
	password := c.Required("密码", in.Password)
	if c.HasError() {
		return nil, apperr.Validation(c.Err())
	}

	u, err := s.repos.User.GetByID(ctx, studentID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if u == nil || !s.hasher.Verify(u.PasswordHash, password) {
		return nil, apperr.ErrBadCredentials
	}
	if u.Banned {
		return nil, apperr.ErrAccountBanned
	}

	return s.issue(ctx, u)
}

func (s *authService) Refresh(ctx context.Context, refreshToken string) (*AuthResult, error) {
	if refreshToken == "" {
		return nil, apperr.ErrUnauthorized
	}
	claims, err := s.tokens.Parse(refreshToken, jwt.TypeRefresh)
	if err != nil {
		return nil, tokenError(err)
	}

	record, err := s.repos.RefreshToken.GetByJTI(ctx, claims.ID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if record == nil || record.TokenHash != jwt.HashToken(refreshToken) {
		return nil, apperr.ErrTokenRevoked
	}

	u, err := s.repos.User.GetByID(ctx, claims.StudentID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if u == nil {
		return nil, apperr.ErrTokenRevoked
	}
	if u.Banned {
		return nil, apperr.ErrAccountBanned
	}

	if err := s.repos.RefreshToken.Revoke(ctx, claims.ID); err != nil {
		return nil, apperr.Internal(err)
	}
	return s.issue(ctx, u)
}

func (s *authService) Logout(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return nil
	}
	claims, err := s.tokens.Parse(refreshToken, jwt.TypeRefresh)
	if err != nil {
		return nil
	}
	if err := s.repos.RefreshToken.Revoke(ctx, claims.ID); err != nil {
		return apperr.Internal(err)
	}
	return nil
}

func (s *authService) Authenticate(ctx context.Context, accessToken string) (*model.User, error) {
	if accessToken == "" {
		return nil, apperr.ErrUnauthorized
	}
	claims, err := s.tokens.Parse(accessToken, jwt.TypeAccess)
	if err != nil {
		return nil, tokenError(err)
	}
	u, err := s.repos.User.GetByID(ctx, claims.StudentID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if u == nil {
		return nil, apperr.ErrTokenRevoked
	}
	if u.Banned {
		return nil, apperr.ErrAccountBanned
	}
	return u, nil
}

func (s *authService) ChangePassword(ctx context.Context, u *model.User, oldPassword, newPassword string) error {
	if !s.hasher.Verify(u.PasswordHash, oldPassword) {
		return apperr.New(400, "BAD_OLD_PASSWORD", "当前密码错误")
	}
	if oldPassword == newPassword {
		return apperr.ErrPasswordUnchanged
	}

	c := validate.New()
	c.Password("新密码", newPassword)
	if c.HasError() {
		return apperr.Validation(c.Err())
	}

	hash, err := s.hasher.Hash(newPassword)
	if err != nil {
		return apperr.Internal(err)
	}
	if err := s.repos.User.UpdateFields(ctx, u.StudentID, map[string]any{
		"password_hash":        hash,
		"must_change_password": false,
	}); err != nil {
		return apperr.Internal(err)
	}
	if err := s.repos.RefreshToken.RevokeByUser(ctx, u.StudentID); err != nil {
		s.log.Warn("修改密码后撤销 Refresh Token 失败", "studentId", u.StudentID, "err", err)
	}
	return nil
}

func (s *authService) issue(ctx context.Context, u *model.User) (*AuthResult, error) {
	pair, err := s.tokens.Issue(u.StudentID, u.Role)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	record := &model.RefreshToken{
		JTI:       pair.RefreshJTI,
		TokenHash: pair.RefreshHash,
		StudentID: u.StudentID,
		IssuedAt:  time.Now(),
		ExpiresAt: pair.RefreshExp,
	}
	if err := s.repos.RefreshToken.Create(ctx, record); err != nil {
		return nil, apperr.Internal(err)
	}

	if _, err := s.repos.RefreshToken.DeleteExpired(ctx, time.Now().Add(-24*time.Hour)); err != nil {
		s.log.Debug("清理过期 Refresh Token 失败", "err", err)
	}

	return &AuthResult{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		TokenType:    pair.TokenType,
		ExpiresIn:    pair.ExpiresIn,
		User:         ToUserDTO(u, false),
	}, nil
}

func tokenError(err error) *apperr.Error {
	if errors.Is(err, jwt.ErrExpired) {
		return apperr.ErrTokenInvalid
	}
	return apperr.ErrTokenInvalid
}
