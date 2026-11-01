// Package util 提供统一响应、JWT 与密码工具。
package util

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Body 统一响应结构。
type Body struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// OK 返回成功响应。
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Body{Code: 0, Message: "ok", Data: data})
}

// OKMsg 返回带提示的成功响应。
func OKMsg(c *gin.Context, message string, data any) {
	c.JSON(http.StatusOK, Body{Code: 0, Message: message, Data: data})
}

// Fail 返回业务错误，HTTP 状态与 code 保持一致便于前端统一处理。
func Fail(c *gin.Context, status int, message string) {
	c.AbortWithStatusJSON(status, Body{Code: status, Message: message})
}

// Page 分页响应。
type Page struct {
	Items    any   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
}
