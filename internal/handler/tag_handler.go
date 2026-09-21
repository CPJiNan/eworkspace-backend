package handler

import (
	"github.com/gin-gonic/gin"

	"eworkspace/internal/model"
	"eworkspace/internal/pkg/apperr"
)

type TagHandler struct {
	base
}

type tagRequest struct {
	Name string        `json:"name"`
	Type model.TagType `json:"type"`
}

func (h *TagHandler) ListSemesters(c *gin.Context) {
	if _, err := mustUser(c); err != nil {
		Fail(c, err)
		return
	}
	items, err := h.svc.Tag.ListSemesters(c.Request.Context())
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, gin.H{"items": items})
}

func (h *TagHandler) CreateSemester(c *gin.Context) {
	actor, err := mustUser(c)
	if err != nil {
		Fail(c, err)
		return
	}
	var req tagRequest
	if err := bindJSON(c, &req); err != nil {
		Fail(c, err)
		return
	}

	dto, err := h.svc.Tag.CreateSemester(c.Request.Context(), actor, req.Name)
	if err != nil {
		Fail(c, err)
		return
	}
	h.writeLog(c, actor, model.OpCreateSemester, model.TargetSemester, formatUint(dto.ID), dto.Name, "")
	Created(c, dto)
}

func (h *TagHandler) RenameSemester(c *gin.Context) {
	actor, err := mustUser(c)
	if err != nil {
		Fail(c, err)
		return
	}
	id, err := uintParam(c, "id")
	if err != nil {
		Fail(c, err)
		return
	}
	var req tagRequest
	if err := bindJSON(c, &req); err != nil {
		Fail(c, err)
		return
	}

	dto, err := h.svc.Tag.RenameSemester(c.Request.Context(), actor, id, req.Name)
	if err != nil {
		Fail(c, err)
		return
	}
	h.writeLog(c, actor, model.OpRenameSemester, model.TargetSemester, formatUint(id), dto.Name, "重命名为 "+dto.Name)
	OK(c, dto)
}

func (h *TagHandler) DeleteSemester(c *gin.Context) {
	actor, err := mustUser(c)
	if err != nil {
		Fail(c, err)
		return
	}
	id, err := uintParam(c, "id")
	if err != nil {
		Fail(c, err)
		return
	}
	if err := h.svc.Tag.DeleteSemester(c.Request.Context(), actor, id); err != nil {
		Fail(c, err)
		return
	}
	h.writeLog(c, actor, model.OpDeleteSemester, model.TargetSemester, formatUint(id), "", "")
	OK(c, gin.H{"id": id, "deleted": true})
}

func (h *TagHandler) ListTags(c *gin.Context) {
	if _, err := mustUser(c); err != nil {
		Fail(c, err)
		return
	}

	var tagType *model.TagType
	if raw := c.Query("type"); raw != "" {
		t := model.TagType(raw)
		if !t.IsValid() {
			Fail(c, apperr.Validation("标签类型不合法"))
			return
		}
		tagType = &t
	}

	items, err := h.svc.Tag.List(c.Request.Context(), tagType)
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, gin.H{"items": items})
}

func (h *TagHandler) CreateTag(c *gin.Context) {
	actor, err := mustUser(c)
	if err != nil {
		Fail(c, err)
		return
	}
	var req tagRequest
	if err := bindJSON(c, &req); err != nil {
		Fail(c, err)
		return
	}

	dto, err := h.svc.Tag.Create(c.Request.Context(), actor, req.Name, req.Type)
	if err != nil {
		Fail(c, err)
		return
	}
	h.writeLog(c, actor, model.OpCreateTag, model.TargetTag, formatUint(dto.ID), dto.Name, dto.Type.String()+"标签")
	Created(c, dto)
}

func (h *TagHandler) RenameTag(c *gin.Context) {
	actor, err := mustUser(c)
	if err != nil {
		Fail(c, err)
		return
	}
	id, err := uintParam(c, "id")
	if err != nil {
		Fail(c, err)
		return
	}
	var req tagRequest
	if err := bindJSON(c, &req); err != nil {
		Fail(c, err)
		return
	}

	dto, err := h.svc.Tag.Rename(c.Request.Context(), actor, id, req.Name)
	if err != nil {
		Fail(c, err)
		return
	}
	h.writeLog(c, actor, model.OpRenameTag, model.TargetTag, formatUint(id), dto.Name, "重命名为 "+dto.Name)
	OK(c, dto)
}

func (h *TagHandler) DeleteTag(c *gin.Context) {
	actor, err := mustUser(c)
	if err != nil {
		Fail(c, err)
		return
	}
	id, err := uintParam(c, "id")
	if err != nil {
		Fail(c, err)
		return
	}
	if err := h.svc.Tag.Delete(c.Request.Context(), actor, id); err != nil {
		Fail(c, err)
		return
	}
	h.writeLog(c, actor, model.OpDeleteTag, model.TargetTag, formatUint(id), "", "")
	OK(c, gin.H{"id": id, "deleted": true})
}
