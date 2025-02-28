package apiErrors

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5"
)

type APIError struct {
	Code   int    `json:"code"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
}

type ErrorMeta struct {
	RequestDateTime string `json:"requestDateTime"`
	TotalPages      int    `json:"totalPages"`
	TotalRecords    int    `json:"totalRecords"`
}

type ApiErrorResponse struct {
	Errors []APIError `json:"errors"`
	Meta   ErrorMeta  `json:"meta"`
}

func (e APIError) Error() string {
	return e.Detail
}

func buildErrorResponse(errors []APIError) ApiErrorResponse {
	meta := ErrorMeta{
		RequestDateTime: time.Now().Format(time.RFC3339),
		TotalPages:      1,
		TotalRecords:    len(errors),
	}

	return ApiErrorResponse{
		Errors: errors,
		Meta:   meta,
	}
}

func newAPIError(statusCode int, title, detail string) APIError {
	cleanTitle := strings.ToUpper(string(title[0])) + title[1:]
	cleanDetail := strings.ToUpper(string(detail[0])) + detail[1:]

	return APIError{
		Code:   statusCode,
		Title:  cleanTitle,
		Detail: cleanDetail,
	}
}

func InternalServerError(detail string) APIError {
	return newAPIError(http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", detail)
}

func BadRequestError(message string) APIError {
	return newAPIError(http.StatusBadRequest, "BAD_REQUEST", message)
}

func NotFoundError(message string) APIError {
	return newAPIError(http.StatusNotFound, "NOT_FOUND", message)
}

func UnauthorizedError(message string) APIError {
	return newAPIError(http.StatusUnauthorized, "UNAUTHORIZED", message)
}

func ForbiddenError(message string) APIError {
	return newAPIError(http.StatusForbidden, "FORBIDDEN", message)
}

func ConflictError(detail string) APIError {
	return newAPIError(http.StatusConflict, "CONFLICT", detail)
}

func HandleError(c *gin.Context, err error) {
	c.Errors = append(c.Errors, &gin.Error{Err: err})
	switch {
	case errors.Is(err, pgx.ErrTooManyRows):
		apiErr := NotFoundError("Query returned too many results")
		c.AbortWithStatusJSON(apiErr.Code, buildErrorResponse([]APIError{apiErr}))
	case errors.Is(err, pgx.ErrNoRows):
		apiErr := NotFoundError("Resource not found")
		c.AbortWithStatusJSON(apiErr.Code, buildErrorResponse([]APIError{apiErr}))
	case errors.As(err, &validator.ValidationErrors{}):
		apiErr := ValidationError(err)
		response := buildErrorResponse([]APIError{apiErr})
		c.AbortWithStatusJSON(apiErr.Code, response)
	case errors.As(err, &APIError{}):
		response := buildErrorResponse([]APIError{err.(APIError)})
		c.AbortWithStatusJSON(err.(APIError).Code, response)
	default:
		apiErr := InternalServerError(err.Error())
		c.AbortWithStatusJSON(apiErr.Code, buildErrorResponse([]APIError{apiErr}))
	}
}

func WrapError(err error, message string) error {
	return fmt.Errorf("%s: %w", message, err)
}

func ValidationError(err error) APIError {
	validationErrors, ok := err.(validator.ValidationErrors)
	if !ok {
		slog.Error("Failed to parse validation error", "error", err)
		return BadRequestError("Validation error, please check the request body")
	}

	var errorMessages []string
	for _, err := range validationErrors {
		field := err.Field()
		if len(field) > 2 {
			field = strings.ToLower(string(field[0])) + field[1:]
		} else {
			field = strings.ToLower(field)
		}

		switch err.Tag() {
		case "required":
			errorMessages = append(errorMessages, fmt.Sprintf("'%s' is required", field))
		case "email":
			errorMessages = append(errorMessages, fmt.Sprintf("'%s' must be a valid email", field))
		case "len":
			errorMessages = append(errorMessages, fmt.Sprintf("'%s' must be exactly %s characters long", field, err.Param()))
		case "min":
			errorMessages = append(errorMessages, fmt.Sprintf("'%s' must be at least %s characters long", field, err.Param()))
		case "max":
			errorMessages = append(errorMessages, fmt.Sprintf("'%s' must be at most %s characters long", field, err.Param()))
		case "datauri":
			errorMessages = append(errorMessages, fmt.Sprintf("'%s' must be a valid URI", field))
		default:
			errorMessages = append(errorMessages, fmt.Sprintf("'%s' failed validation for '%s'", field, err.Tag()))
		}
	}

	return BadRequestError(strings.Join(errorMessages, "; "))
}
