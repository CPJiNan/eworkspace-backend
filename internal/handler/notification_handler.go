package handler

import (
	"github.com/gin-gonic/gin"
)

type NotificationHandler struct {
	base
}

type deleteNotificationsRequest struct {
	IDs []uint `json:"ids"`
}

func (h *NotificationHandler) List(c *gin.Context) {
	actor, err := mustUser(c)
	if err != nil {
		Fail(c, err)
		return
	}
	page, size := pageQuery(c)
	result, err := h.svc.Notification.List(c.Request.Context(), actor, page, size)
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, result)
}

func (h *NotificationHandler) MarkRead(c *gin.Context) {
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
	if err := h.svc.Notification.MarkRead(c.Request.Context(), actor, id); err != nil {
		Fail(c, err)
		return
	}
	OK(c, gin.H{"id": id, "read": true})
}

func (h *NotificationHandler) MarkAllRead(c *gin.Context) {
	actor, err := mustUser(c)
	if err != nil {
		Fail(c, err)
		return
	}
	if err := h.svc.Notification.MarkAllRead(c.Request.Context(), actor); err != nil {
		Fail(c, err)
		return
	}
	OK(c, gin.H{"read": true})
}

func (h *NotificationHandler) Delete(c *gin.Context) {
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
	if err := h.svc.Notification.Delete(c.Request.Context(), actor, id); err != nil {
		Fail(c, err)
		return
	}
	OK(c, gin.H{"id": id, "deleted": true})
}

func (h *NotificationHandler) BatchDelete(c *gin.Context) {
	actor, err := mustUser(c)
	if err != nil {
		Fail(c, err)
		return
	}
	var req deleteNotificationsRequest
	if err := bindJSON(c, &req); err != nil {
		Fail(c, err)
		return
	}
	affected, err := h.svc.Notification.DeleteMany(c.Request.Context(), actor, req.IDs)
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, gin.H{"deleted": affected})
}

func (h *NotificationHandler) Clear(c *gin.Context) {
	actor, err := mustUser(c)
	if err != nil {
		Fail(c, err)
		return
	}
	affected, err := h.svc.Notification.Clear(c.Request.Context(), actor)
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, gin.H{"deleted": affected})
}
