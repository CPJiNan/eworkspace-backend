package model

import (
	"time"

	"gorm.io/gorm"
)

type Semester struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	Name      string     `gorm:"type:varchar(50);not null;uniqueIndex" json:"name"`
	StartDate *time.Time `json:"startDate,omitempty"`
	EndDate   *time.Time `json:"endDate,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (Semester) TableName() string { return "semesters" }

type Tag struct {
	ID   uint    `gorm:"primaryKey" json:"id"`
	Name string  `gorm:"type:varchar(50);not null;index" json:"name"`
	Type TagType `gorm:"type:varchar(16);not null;index" json:"type"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (Tag) TableName() string { return "tags" }

type Project struct {
	ID            uint          `gorm:"primaryKey" json:"id"`
	Name          string        `gorm:"type:varchar(50);not null;index" json:"name"`
	Description   string        `gorm:"type:text" json:"description"`
	Deadline      time.Time     `gorm:"not null;index" json:"deadline"`
	Status        ProjectStatus `gorm:"type:varchar(16);not null;index" json:"status"`
	CreatorID     string        `gorm:"type:varchar(32);not null;index" json:"creatorId"`
	CreatorName   string        `gorm:"type:varchar(50)" json:"creatorName"`
	DeletedReason string        `gorm:"type:varchar(255)" json:"-"`

	Semesters   []Semester   `gorm:"many2many:project_semesters;" json:"semesters,omitempty"`
	Tags        []Tag        `gorm:"many2many:project_tags;" json:"tags,omitempty"`
	Assignments []Assignment `gorm:"foreignKey:ProjectID;constraint:OnDelete:CASCADE" json:"assignments,omitempty"`

	CreatedAt time.Time      `gorm:"index" json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Project) TableName() string { return "projects" }

type ProjectSemester struct {
	ProjectID  uint `gorm:"primaryKey" json:"projectId"`
	SemesterID uint `gorm:"primaryKey" json:"semesterId"`
}

func (ProjectSemester) TableName() string { return "project_semesters" }

type ProjectTag struct {
	ProjectID uint `gorm:"primaryKey" json:"projectId"`
	TagID     uint `gorm:"primaryKey" json:"tagId"`
}

func (ProjectTag) TableName() string { return "project_tags" }

type Assignment struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	ProjectID    uint       `gorm:"not null;index" json:"projectId"`
	Name         string     `gorm:"type:varchar(50);not null" json:"name"`
	Description  string     `gorm:"type:text" json:"description"`
	Capacity     int        `gorm:"not null;default:1" json:"capacity"`
	ClaimedCount int        `gorm:"not null;default:0" json:"claimedCount"`
	TagID        *uint      `gorm:"index" json:"tagId,omitempty"`
	Deadline     *time.Time `gorm:"index" json:"deadline,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (Assignment) TableName() string { return "assignments" }

type Discussion struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	ProjectID  uint   `gorm:"not null;index" json:"projectId"`
	AuthorID   string `gorm:"type:varchar(32);not null;index" json:"authorId"`
	AuthorName string `gorm:"type:varchar(50)" json:"authorName"`
	Content    string `gorm:"type:text;not null" json:"content"`

	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `gorm:"index" json:"-"`
}

func (Discussion) TableName() string { return "discussions" }

type Notification struct {
	ID           uint             `gorm:"primaryKey" json:"id"`
	ReceiverID   string           `gorm:"type:varchar(32);not null;index" json:"receiverId"`
	Type         NotificationType `gorm:"type:varchar(32);not null;index" json:"type"`
	Title        string           `gorm:"type:varchar(100);not null" json:"title"`
	Content      string           `gorm:"type:text" json:"content"`
	ProjectID    *uint            `gorm:"index" json:"projectId,omitempty"`
	AssignmentID *uint            `gorm:"index" json:"assignmentId,omitempty"`
	Read         bool             `gorm:"not null;default:false;index" json:"read"`

	CreatedAt time.Time `gorm:"index" json:"createdAt"`
}

func (Notification) TableName() string { return "notifications" }

type OperationLog struct {
	ID           uint          `gorm:"primaryKey" json:"id"`
	OperatorID   string        `gorm:"type:varchar(32);not null;index" json:"operatorId"`
	OperatorName string        `gorm:"type:varchar(50)" json:"operatorName"`
	Type         OperationType `gorm:"type:varchar(32);not null;index" json:"type"`
	TargetType   TargetType    `gorm:"type:varchar(32);index" json:"targetType"`
	TargetID     string        `gorm:"type:varchar(64);index" json:"targetId"`
	TargetName   string        `gorm:"type:varchar(128)" json:"targetName"`
	Detail       string        `gorm:"type:varchar(500)" json:"detail"`
	IP           string        `gorm:"type:varchar(64)" json:"ip"`
	UserAgent    string        `gorm:"type:varchar(255)" json:"userAgent"`

	CreatedAt time.Time `gorm:"index" json:"createdAt"`
}

func (OperationLog) TableName() string { return "operation_logs" }
