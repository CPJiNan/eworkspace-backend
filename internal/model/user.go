package model

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	StudentID          string     `gorm:"primaryKey;type:varchar(32)" json:"studentId"`
	Name               string     `gorm:"type:varchar(50);not null" json:"name"`
	Phone              string     `gorm:"type:varchar(20);not null" json:"phone"`
	WeChat             string     `gorm:"column:wechat;type:varchar(64);not null" json:"wechat"`
	QQ                 string     `gorm:"type:varchar(20)" json:"qq"`
	Email              string     `gorm:"type:varchar(128);not null" json:"email"`
	PasswordHash       string     `gorm:"column:password_hash;type:varchar(100);not null" json:"-"`
	Role               Role       `gorm:"not null;index" json:"role"`
	MustChangePassword bool       `gorm:"column:must_change_password;not null" json:"mustChangePassword"`
	Banned             bool       `gorm:"not null;index" json:"banned"`
	BannedAt           *time.Time `gorm:"column:banned_at" json:"bannedAt,omitempty"`

	CreatedAt time.Time      `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
}

func (User) TableName() string { return "users" }

const DefaultEmailDomain = "m.fudan.edu.cn"

func DefaultEmail(studentID string) string {
	return studentID + "@" + DefaultEmailDomain
}

func DefaultPasswordFor(studentID string) string {
	suffix := studentID
	if len(suffix) > 5 {
		suffix = suffix[len(suffix)-5:]
	}
	return "Fdu" + suffix
}

type RefreshToken struct {
	ID        uint   `gorm:"primaryKey" json:"-"`
	JTI       string `gorm:"type:varchar(64);uniqueIndex;not null" json:"-"`
	TokenHash string `gorm:"type:varchar(64);not null" json:"-"`
	StudentID string `gorm:"type:varchar(32);index;not null" json:"-"`

	IssuedAt  time.Time `json:"-"`
	ExpiresAt time.Time `gorm:"index" json:"-"`
	Revoked   bool      `gorm:"not null;default:false" json:"-"`

	CreatedAt time.Time `json:"-"`
}

func (RefreshToken) TableName() string { return "refresh_tokens" }
