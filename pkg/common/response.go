package common

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response 统一响应结构
type Response struct {
	Code int         `json:"code"`
	Data interface{} `json:"data"`
	Msg  string      `json:"msg"`
}

// ========== 响应构建函数 ==========

// Success 构建成功响应
func Success(data interface{}, msg string) Response {
	if data == nil {
		data = struct{}{}
	}
	if msg == "" {
		msg = "操作成功"
	}
	return Response{Code: 0, Data: data, Msg: msg}
}

// SuccessWithData 构建成功响应（使用默认消息）
func SuccessWithData(data interface{}) Response {
	return Success(data, "操作成功")
}

// SuccessWithMessage 构建成功响应（无数据）
func SuccessWithMessage(msg string) Response {
	return Success(struct{}{}, msg)
}

// SuccessEmpty 构建空成功响应
func SuccessEmpty() Response {
	return Success(struct{}{}, "操作成功")
}

// Error 构建错误响应
func Error(code int, msg string) Response {
	if msg == "" {
		msg = GetDefaultMessage(code)
	}
	return Response{Code: code, Data: struct{}{}, Msg: msg}
}

// ErrorWithData 构建带数据的错误响应
func ErrorWithData(code int, data interface{}, msg string) Response {
	if data == nil {
		data = struct{}{}
	}
	if msg == "" {
		msg = GetDefaultMessage(code)
	}
	return Response{Code: code, Data: data, Msg: msg}
}

// ========== Gin 快捷方法 ==========

// JSON 统一 JSON 响应
func JSON(c *gin.Context, code int, data interface{}, msg string) {
	c.JSON(http.StatusOK, Response{
		Code: code,
		Data: data,
		Msg:  msg,
	})
}

// SuccessResp 成功响应快捷方法
func SuccessResp(c *gin.Context, data interface{}) {
	JSON(c, 0, data, "操作成功")
}

// SuccessRespWithMsg 成功响应快捷方法（自定义消息）
func SuccessRespWithMsg(c *gin.Context, data interface{}, msg string) {
	JSON(c, 0, data, msg)
}

// ErrorResp 错误响应快捷方法
func ErrorResp(c *gin.Context, code int, msg string) {
	JSON(c, code, struct{}{}, msg)
}

// ErrorRespWithData 错误响应快捷方法（带数据）
func ErrorRespWithData(c *gin.Context, code int, data interface{}, msg string) {
	JSON(c, code, data, msg)
}

// BindAndValidate 绑定并验证请求参数
func BindAndValidate(c *gin.Context, obj interface{}) bool {
	if err := c.ShouldBindJSON(obj); err != nil {
		ErrorResp(c, CodeInvalidParams, err.Error())
		return false
	}
	return true
}

// BindQueryAndValidate 绑定并验证 Query 参数
func BindQueryAndValidate(c *gin.Context, obj interface{}) bool {
	if err := c.ShouldBindQuery(obj); err != nil {
		ErrorResp(c, CodeInvalidParams, err.Error())
		return false
	}
	return true
}

// HandleError 处理 error 并返回响应
func HandleError(c *gin.Context, err error, defaultCode int, defaultMsg string) {
	if err != nil {
		ErrorResp(c, defaultCode, defaultMsg)
	}
}
