package service

import (
	"context"
	"errors"
	"strings"

	"eworkspace/internal/model"
	"eworkspace/internal/pkg/apperr"
	"eworkspace/internal/pkg/validate"
	"eworkspace/internal/repository"
)

type SemesterDTO struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type TagDTO struct {
	ID   uint          `json:"id"`
	Name string        `json:"name"`
	Type model.TagType `json:"type"`
}

func ToSemesterDTO(s *model.Semester) SemesterDTO {
	return SemesterDTO{ID: s.ID, Name: s.Name}
}

func ToSemesterDTOs(list []model.Semester) []SemesterDTO {
	out := make([]SemesterDTO, 0, len(list))
	for i := range list {
		out = append(out, ToSemesterDTO(&list[i]))
	}
	return out
}

func ToTagDTO(t *model.Tag) TagDTO {
	return TagDTO{ID: t.ID, Name: t.Name, Type: t.Type}
}

func ToTagDTOs(list []model.Tag) []TagDTO {
	out := make([]TagDTO, 0, len(list))
	for i := range list {
		out = append(out, ToTagDTO(&list[i]))
	}
	return out
}

type TagService interface {
	ListSemesters(ctx context.Context) ([]SemesterDTO, error)
	CreateSemester(ctx context.Context, operator *model.User, name string) (*SemesterDTO, error)
	RenameSemester(ctx context.Context, operator *model.User, id uint, name string) (*SemesterDTO, error)
	DeleteSemester(ctx context.Context, operator *model.User, id uint) error

	List(ctx context.Context, tagType *model.TagType) ([]TagDTO, error)
	Create(ctx context.Context, operator *model.User, name string, tagType model.TagType) (*TagDTO, error)
	Rename(ctx context.Context, operator *model.User, id uint, name string) (*TagDTO, error)
	Delete(ctx context.Context, operator *model.User, id uint) error
}

type tagService struct {
	meta  *meta
	repos *repository.Repositories
	log   Logger
}

func NewTagService(m *meta, repos *repository.Repositories, log Logger) TagService {
	return &tagService{meta: m, repos: repos, log: log}
}

func (s *tagService) checker() *validate.Checker {
	return validate.New()
}

func (s *tagService) ListSemesters(ctx context.Context) ([]SemesterDTO, error) {
	list, err := s.repos.Semester.List(ctx)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return ToSemesterDTOs(list), nil
}

func (s *tagService) CreateSemester(ctx context.Context, operator *model.User, name string) (*SemesterDTO, error) {
	if !operator.Role.CanManage() {
		return nil, apperr.ErrForbidden
	}
	c := s.checker()
	name = c.Short("学期名称", name)
	if c.HasError() {
		return nil, apperr.Validation(c.Err())
	}

	exists, err := s.repos.Semester.GetByName(ctx, name)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if exists != nil {
		return nil, apperr.ErrDuplicateName
	}

	semester := &model.Semester{Name: name}
	if err := s.repos.Semester.Create(ctx, semester); err != nil {
		if isDuplicateErr(err) {
			return nil, apperr.ErrDuplicateName
		}
		return nil, apperr.Internal(err)
	}
	dto := ToSemesterDTO(semester)
	return &dto, nil
}

func (s *tagService) RenameSemester(ctx context.Context, operator *model.User, id uint, name string) (*SemesterDTO, error) {
	if !operator.Role.CanManage() {
		return nil, apperr.ErrForbidden
	}
	c := s.checker()
	name = c.Short("学期名称", name)
	if c.HasError() {
		return nil, apperr.Validation(c.Err())
	}

	semester, err := s.repos.Semester.GetByID(ctx, id)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if semester == nil {
		return nil, apperr.NotFound("学期不存在")
	}
	if err := s.repos.Semester.UpdateName(ctx, id, name); err != nil {
		if isDuplicateErr(err) {
			return nil, apperr.ErrDuplicateName
		}
		return nil, apperr.Internal(err)
	}
	semester.Name = name
	dto := ToSemesterDTO(semester)
	return &dto, nil
}

func (s *tagService) DeleteSemester(ctx context.Context, operator *model.User, id uint) error {
	if !operator.Role.CanManage() {
		return apperr.ErrForbidden
	}
	semester, err := s.repos.Semester.GetByID(ctx, id)
	if err != nil {
		return apperr.Internal(err)
	}
	if semester == nil {
		return apperr.NotFound("学期不存在")
	}
	return s.repos.Semester.Delete(ctx, id)
}

func (s *tagService) List(ctx context.Context, tagType *model.TagType) ([]TagDTO, error) {
	if tagType != nil && !tagType.IsValid() {
		return nil, apperr.Validation("标签类型不合法")
	}
	list, err := s.repos.Tag.List(ctx, tagType)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return ToTagDTOs(list), nil
}

func (s *tagService) Create(ctx context.Context, operator *model.User, name string, tagType model.TagType) (*TagDTO, error) {
	if !operator.Role.CanManage() {
		return nil, apperr.ErrForbidden
	}
	if !tagType.IsValid() {
		return nil, apperr.Validation("标签类型不合法")
	}
	c := s.checker()
	name = c.Short("标签名称", name)
	if c.HasError() {
		return nil, apperr.Validation(c.Err())
	}

	exists, err := s.repos.Tag.GetByName(ctx, name, tagType)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if exists != nil {
		return nil, apperr.ErrDuplicateName
	}

	tag := &model.Tag{Name: name, Type: tagType}
	if err := s.repos.Tag.Create(ctx, tag); err != nil {
		if isDuplicateErr(err) {
			return nil, apperr.ErrDuplicateName
		}
		return nil, apperr.Internal(err)
	}
	dto := ToTagDTO(tag)
	return &dto, nil
}

func (s *tagService) Rename(ctx context.Context, operator *model.User, id uint, name string) (*TagDTO, error) {
	if !operator.Role.CanManage() {
		return nil, apperr.ErrForbidden
	}
	c := s.checker()
	name = c.Short("标签名称", name)
	if c.HasError() {
		return nil, apperr.Validation(c.Err())
	}

	tag, err := s.repos.Tag.GetByID(ctx, id)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if tag == nil {
		return nil, apperr.NotFound("标签不存在")
	}

	if err := s.repos.Tag.Rename(ctx, id, name); err != nil {
		if isDuplicateErr(err) {
			return nil, apperr.ErrDuplicateName
		}
		return nil, apperr.Internal(err)
	}
	tag.Name = name
	dto := ToTagDTO(tag)
	return &dto, nil
}

func (s *tagService) Delete(ctx context.Context, operator *model.User, id uint) error {
	if !operator.Role.CanManage() {
		return apperr.ErrForbidden
	}
	tag, err := s.repos.Tag.GetByID(ctx, id)
	if err != nil {
		return apperr.Internal(err)
	}
	if tag == nil {
		return apperr.NotFound("标签不存在")
	}
	return s.repos.Tag.Delete(ctx, id)
}

func isDuplicateErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, repository.ErrDuplicate) {
		return true
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}
