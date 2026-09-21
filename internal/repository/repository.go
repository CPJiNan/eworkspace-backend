package repository

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"
)

const sqliteBusyRetry = 5

func isBusyError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "database is locked") ||
		strings.Contains(msg, "SQLITE_BUSY") ||
		strings.Contains(msg, "database table is locked")
}

func withRetry(ctx context.Context, fn func() error) error {
	var err error
	for attempt := 0; attempt < sqliteBusyRetry; attempt++ {
		err = fn()
		if err == nil || !isBusyError(err) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 20 * time.Millisecond):
		}
	}
	return err
}

type Repositories struct {
	User         UserRepo
	RefreshToken RefreshTokenRepo
	Semester     SemesterRepo
	Tag          TagRepo
	Project      ProjectRepo
	Assignment   AssignmentRepo
	Member       ProjectMemberRepo
	Discussion   DiscussionRepo
	Notification NotificationRepo
	OperationLog OperationLogRepo
}

func New(db *gorm.DB) *Repositories {
	return &Repositories{
		User:         NewUserRepo(db),
		RefreshToken: NewRefreshTokenRepo(db),
		Semester:     NewSemesterRepo(db),
		Tag:          NewTagRepo(db),
		Project:      NewProjectRepo(db),
		Assignment:   NewAssignmentRepo(db),
		Member:       NewProjectMemberRepo(db),
		Discussion:   NewDiscussionRepo(db),
		Notification: NewNotificationRepo(db),
		OperationLog: NewOperationLogRepo(db),
	}
}
