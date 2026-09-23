package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"github.com/gbtreehole/backend/internal/constants"
)

type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Response{Code: constants.CodeOK, Message: "ok", Data: data})
}

func Fail(c *gin.Context, httpStatus, code int, message string) {
	c.JSON(httpStatus, Response{Code: code, Message: message, Data: nil})
}

func BindAndValidate(c *gin.Context, req any) bool {
	if err := c.ShouldBindJSON(req); err != nil {
		Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "invalid request body")
		return false
	}
	if err := validate.Struct(req); err != nil {
		var ve validator.ValidationErrors
		if errors.As(err, &ve) {
			Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "validation failed: "+ve.Error())
			return false
		}
		Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "validation failed")
		return false
	}
	return true
}

func BindQuery(c *gin.Context, req any) bool {
	if err := c.ShouldBindQuery(req); err != nil {
		Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "invalid query params")
		return false
	}
	if err := validate.Struct(req); err != nil {
		Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "invalid query params")
		return false
	}
	return true
}
