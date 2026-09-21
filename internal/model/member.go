package model

import "time"

type ProjectMember struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	ProjectID    uint   `gorm:"not null;index" json:"projectId"`
	AssignmentID uint   `gorm:"not null;index" json:"assignmentId"`
	StudentID    string `gorm:"type:varchar(32);not null;index" json:"studentId"`
	StudentName  string `gorm:"type:varchar(50)" json:"studentName"`
	Assigned     bool   `gorm:"not null;default:false" json:"assigned"`
	AssignedBy   string `gorm:"type:varchar(32)" json:"assignedBy,omitempty"`

	Project    *Project    `gorm:"foreignKey:ProjectID;references:ID" json:"project,omitempty"`
	Assignment *Assignment `gorm:"foreignKey:AssignmentID;references:ID" json:"assignment,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
}

func (ProjectMember) TableName() string { return "project_members" }
