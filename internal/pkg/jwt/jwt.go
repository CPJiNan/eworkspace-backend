package jwt

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"

	"eworkspace/internal/config"
	"eworkspace/internal/model"
)

type TokenType string

const (
	TypeAccess  TokenType = "access"
	TypeRefresh TokenType = "refresh"
)

type Claims struct {
	StudentID string     `json:"sid"`
	Role      model.Role `json:"role"`
	TokenType TokenType  `json:"typ"`
	jwtlib.RegisteredClaims
}

const issuer = "eworkspace"

type Manager struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewManager(cfg config.JWTConfig) *Manager {
	return &Manager{
		secret:     []byte(cfg.Secret),
		accessTTL:  time.Duration(cfg.AccessTTLMinutes) * time.Minute,
		refreshTTL: time.Duration(cfg.RefreshTTLHours) * time.Hour,
	}
}

type TokenPair struct {
	AccessToken  string    `json:"accessToken"`
	RefreshToken string    `json:"refreshToken"`
	TokenType    string    `json:"tokenType"`
	ExpiresIn    int64     `json:"expiresIn"`
	AccessExpiry time.Time `json:"accessExpiresAt"`
	RefreshJTI   string    `json:"-"`
	RefreshHash  string    `json:"-"`
	RefreshExp   time.Time `json:"-"`
}

func (m *Manager) Issue(studentID string, role model.Role) (*TokenPair, error) {
	now := time.Now()

	accessExp := now.Add(m.accessTTL)
	access := Claims{
		StudentID: studentID,
		Role:      role,
		TokenType: TypeAccess,
		RegisteredClaims: jwtlib.RegisteredClaims{
			Issuer:    issuer,
			Subject:   studentID,
			IssuedAt:  jwtlib.NewNumericDate(now),
			NotBefore: jwtlib.NewNumericDate(now),
			ExpiresAt: jwtlib.NewNumericDate(accessExp),
		},
	}
	accessToken, err := m.sign(access)
	if err != nil {
		return nil, err
	}

	refreshExp := now.Add(m.refreshTTL)
	refresh := Claims{
		StudentID: studentID,
		Role:      role,
		TokenType: TypeRefresh,
		RegisteredClaims: jwtlib.RegisteredClaims{
			Issuer:    issuer,
			Subject:   studentID,
			ID:        newJTI(studentID, now),
			IssuedAt:  jwtlib.NewNumericDate(now),
			NotBefore: jwtlib.NewNumericDate(now),
			ExpiresAt: jwtlib.NewNumericDate(refreshExp),
		},
	}
	refreshToken, err := m.sign(refresh)
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(m.accessTTL.Seconds()),
		AccessExpiry: accessExp,
		RefreshJTI:   refresh.ID,
		RefreshHash:  HashToken(refreshToken),
		RefreshExp:   refreshExp,
	}, nil
}

func (m *Manager) Parse(token string, expected TokenType) (*Claims, error) {
	claims := &Claims{}
	parsed, err := jwtlib.ParseWithClaims(token, claims, func(t *jwtlib.Token) (any, error) {
		if _, ok := t.Method.(*jwtlib.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("非预期的签名算法: %v", t.Header["alg"])
		}
		return m.secret, nil
	}, jwtlib.WithIssuer(issuer), jwtlib.WithValidMethods([]string{"HS256"}))
	if err != nil {
		if errors.Is(err, jwtlib.ErrTokenExpired) {
			return nil, ErrExpired
		}
		return nil, ErrInvalid
	}
	if !parsed.Valid || claims.TokenType != expected {
		return nil, ErrInvalid
	}
	return claims, nil
}

var (
	ErrExpired = errors.New("token 已过期")
	ErrInvalid = errors.New("token 非法")
)

func (m *Manager) sign(claims Claims) (string, error) {
	token := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

func newJTI(studentID string, now time.Time) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%d|%s", studentID, now.UnixNano(), studentID)))
	return hex.EncodeToString(sum[:16])
}

func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
