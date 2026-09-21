package model

type Role int8

const (
	RoleSuperAdmin Role = 0
	RoleAdmin      Role = 1
	RoleUser       Role = 2
)

func (r Role) String() string {
	switch r {
	case RoleSuperAdmin:
		return "超级管理员"
	case RoleAdmin:
		return "管理员"
	case RoleUser:
		return "普通用户"
	default:
		return "未知角色"
	}
}

func (r Role) IsValid() bool {
	return r >= RoleSuperAdmin && r <= RoleUser
}

func (r Role) IsAdmin() bool { return r == RoleAdmin || r == RoleSuperAdmin }

func (r Role) CanManage() bool { return r.IsAdmin() }

type ProjectStatus string

const (
	ProjectStatusActive    ProjectStatus = "active"
	ProjectStatusFinished  ProjectStatus = "finished"
	ProjectStatusCancelled ProjectStatus = "cancelled"
)

func (s ProjectStatus) String() string {
	switch s {
	case ProjectStatusActive:
		return "进行中"
	case ProjectStatusFinished:
		return "已结束"
	case ProjectStatusCancelled:
		return "已取消"
	default:
		return "未知状态"
	}
}

func (s ProjectStatus) IsValid() bool {
	return s == ProjectStatusActive || s == ProjectStatusFinished || s == ProjectStatusCancelled
}

type TagType string

const (
	TagTypeSemester TagType = "semester"
	TagTypeProject  TagType = "project"
	TagTypeDivision TagType = "division"
)

func (t TagType) IsValid() bool {
	return t == TagTypeSemester || t == TagTypeProject || t == TagTypeDivision
}

func (t TagType) String() string {
	switch t {
	case TagTypeSemester:
		return "学期"
	case TagTypeProject:
		return "项目"
	case TagTypeDivision:
		return "分工"
	default:
		return "未知标签"
	}
}

type NotificationType string

const (
	NotificationTypeProjectPublished NotificationType = "project_published"
	NotificationTypeAssigned         NotificationType = "assigned"
)

type OperationType string

const (
	OpClaimAssignment  OperationType = "claim_assignment"
	OpCancelAssignment OperationType = "cancel_assignment"

	OpCreateProject    OperationType = "create_project"
	OpUpdateProject    OperationType = "update_project"
	OpDeleteProject    OperationType = "delete_project"
	OpAssignAssignment OperationType = "assign_assignment"

	OpCreateAccount OperationType = "create_account"
	OpUpdateMember  OperationType = "update_member"
	OpDeleteMember  OperationType = "delete_member"
	OpResetPassword OperationType = "reset_password"
	OpBanAccount    OperationType = "ban_account"
	OpUnbanAccount  OperationType = "unban_account"
	OpCreateAdmin   OperationType = "create_admin"
	OpDeleteAdmin   OperationType = "delete_admin"

	OpCreateTag      OperationType = "create_tag"
	OpRenameTag      OperationType = "rename_tag"
	OpDeleteTag      OperationType = "delete_tag"
	OpCreateSemester OperationType = "create_semester"
	OpRenameSemester OperationType = "rename_semester"
	OpDeleteSemester OperationType = "delete_semester"

	OpClearOperationLog OperationType = "clear_operation_log"
)

var loggableOperations = map[OperationType]struct{}{
	OpClaimAssignment:   {},
	OpCancelAssignment:  {},
	OpCreateProject:     {},
	OpUpdateProject:     {},
	OpDeleteProject:     {},
	OpAssignAssignment:  {},
	OpCreateAccount:     {},
	OpUpdateMember:      {},
	OpDeleteMember:      {},
	OpResetPassword:     {},
	OpBanAccount:        {},
	OpUnbanAccount:      {},
	OpCreateAdmin:       {},
	OpDeleteAdmin:       {},
	OpCreateTag:         {},
	OpRenameTag:         {},
	OpDeleteTag:         {},
	OpCreateSemester:    {},
	OpRenameSemester:    {},
	OpDeleteSemester:    {},
	OpClearOperationLog: {},
}

func LoggableOperations() []OperationType {
	list := make([]OperationType, 0, len(loggableOperations))
	for op := range loggableOperations {
		list = append(list, op)
	}
	return list
}

func (t OperationType) IsLoggable() bool {
	_, ok := loggableOperations[t]
	return ok
}

type TargetType string

const (
	TargetUser         TargetType = "user"
	TargetProject      TargetType = "project"
	TargetAssignment   TargetType = "assignment"
	TargetTag          TargetType = "tag"
	TargetSemester     TargetType = "semester"
	TargetOperationLog TargetType = "operation_log"
)
