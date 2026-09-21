package handler

import (
	"time"

	"github.com/gin-gonic/gin"

	"eworkspace/internal/model"
	"eworkspace/internal/pkg/apperr"
	"eworkspace/internal/service"
)

type LogHandler struct {
	base
}

type clearLogsRequest struct {
	From *string `json:"from"`
	To   *string `json:"to"`
}

func (h *LogHandler) List(c *gin.Context) {
	actor, err := mustUser(c)
	if err != nil {
		Fail(c, err)
		return
	}

	var opType *model.OperationType
	if raw := c.Query("type"); raw != "" {
		t := model.OperationType(raw)
		opType = &t
	}
	var targetType *model.TargetType
	if raw := c.Query("targetType"); raw != "" {
		t := model.TargetType(raw)
		targetType = &t
	}

	from, err := parseTimeQuery(c, "from", false)
	if err != nil {
		Fail(c, err)
		return
	}
	to, err := parseTimeQuery(c, "to", true)
	if err != nil {
		Fail(c, err)
		return
	}

	page, size := pageQuery(c)
	items, meta, err := h.svc.OperationLog.List(c.Request.Context(), actor, service.ListLogsInput{
		OperatorID: c.Query("operatorId"),
		Type:       opType,
		TargetType: targetType,
		TargetID:   c.Query("targetId"),
		From:       from,
		To:         to,
		Page:       page,
		Size:       size,
	})
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, NewPageResult(items, meta))
}

func (h *LogHandler) BatchDelete(c *gin.Context) {
	actor, err := mustUser(c)
	if err != nil {
		Fail(c, err)
		return
	}
	var req struct {
		IDs []uint `json:"ids"`
	}
	if err := bindJSON(c, &req); err != nil {
		Fail(c, err)
		return
	}

	affected, err := h.svc.OperationLog.DeleteMany(c.Request.Context(), actor, req.IDs)
	if err != nil {
		Fail(c, err)
		return
	}
	h.writeLog(c, actor, model.OpClearOperationLog, model.TargetOperationLog, "", "",
		"按 ID 批量删除 "+formatUint(uint(affected))+" 条操作日志")
	OK(c, gin.H{"deleted": affected})
}

func (h *LogHandler) ClearByRange(c *gin.Context) {
	actor, err := mustUser(c)
	if err != nil {
		Fail(c, err)
		return
	}
	var req clearLogsRequest
	if err := bindJSON(c, &req); err != nil {
		Fail(c, err)
		return
	}

	var from, to *time.Time
	if req.From != nil {
		t, err := parseTime(*req.From, false)
		if err != nil {
			Fail(c, err)
			return
		}
		from = &t
	}
	if req.To != nil {
		t, err := parseTime(*req.To, true)
		if err != nil {
			Fail(c, err)
			return
		}
		to = &t
	}
	if from == nil && to == nil {
		Fail(c, apperr.Validation("请指定要清理的时间范围"))
		return
	}

	affected, err := h.svc.OperationLog.DeleteByRange(c.Request.Context(), actor, from, to)
	if err != nil {
		Fail(c, err)
		return
	}
	h.writeLog(c, actor, model.OpClearOperationLog, model.TargetOperationLog, "", "",
		"按时间范围清理 "+formatUint(uint(affected))+" 条操作日志")
	OK(c, gin.H{"deleted": affected})
}
