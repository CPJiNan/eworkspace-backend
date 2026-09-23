package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"eworkspace/internal/pkg/apperr"
	"eworkspace/internal/service"
)

type Response struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Response{Code: "OK", Message: "成功", Data: data})
}

func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, Response{Code: "OK", Message: "创建成功", Data: data})
}

func Fail(c *gin.Context, err error) {
	e := apperr.From(err)
	if e == nil {
		e = apperr.ErrInternal
	}
	c.AbortWithStatusJSON(e.Status, Response{Code: e.Code, Message: e.Message})
}

type PageResult[T any] struct {
	Items []T              `json:"items"`
	Page  service.PageMeta `json:"page"`
}

func NewPageResult[T any](items []T, page service.PageMeta) PageResult[T] {
	if items == nil {
		items = make([]T, 0)
	}
	return PageResult[T]{Items: items, Page: page}
}

func bindJSON(c *gin.Context, dst any) error {
	if c.Request == nil || c.Request.Body == nil {
		return nil
	}
	if err := c.ShouldBindJSON(dst); err != nil {
		return apperr.Validation("请求体格式不正确：" + err.Error())
	}
	return nil
}

func uintParam(c *gin.Context, name string) (uint, error) {
	raw := c.Param(name)
	v, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || v == 0 {
		return 0, apperr.Validation("路径参数 " + name + " 不合法")
	}
	return uint(v), nil
}

func pageQuery(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", strconv.Itoa(service.DefaultPageSize)))
	return service.NormalizePage(page, size)
}

func boolQuery(c *gin.Context, name string) bool {
	raw := strings.ToLower(strings.TrimSpace(c.Query(name)))
	return raw == "1" || raw == "true" || raw == "yes"
}

func intQueryOptional(c *gin.Context, name string) (*int, error) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return nil, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return nil, apperr.Validation("参数 " + name + " 不合法")
	}
	return &v, nil
}

func uintQueryOptional(c *gin.Context, name string) (*uint, error) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return nil, nil
	}
	v, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || v == 0 {
		return nil, apperr.Validation("参数 " + name + " 不合法")
	}
	result := uint(v)
	return &result, nil
}

func uintList(c *gin.Context, name string) ([]uint, error) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	out := make([]uint, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		v, err := strconv.ParseUint(part, 10, 64)
		if err != nil || v == 0 {
			return nil, apperr.Validation("参数 " + name + " 不合法")
		}
		out = append(out, uint(v))
	}
	return out, nil
}
