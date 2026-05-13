package errors_test

import (
	"errors"
	"testing"

	pkgerrors "bdd-cli/src/internal/pkg/errors"
)

const (
	testErrCode = "TEST_ERROR"
	testErrMsg  = "test message"
)

func TestAppError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *pkgerrors.AppError
		want string
	}{
		{
			name: "error with cause",
			err: &pkgerrors.AppError{
				Category: pkgerrors.CategoryAI,
				Code:     testErrCode,
				Message:  testErrMsg,
				Cause:    errors.New("underlying error"),
			},
			want: "[ai:TEST_ERROR] test message: underlying error",
		},
		{
			name: "error without cause",
			err: &pkgerrors.AppError{
				Category: pkgerrors.CategoryGitHub,
				Code:     testErrCode,
				Message:  testErrMsg,
				Cause:    nil,
			},
			want: "[github:TEST_ERROR] test message",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if got := testCase.err.Error(); got != testCase.want {
				t.Errorf("AppError.Error() = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestAppError_Unwrap(t *testing.T) {
	cause := errors.New("underlying error")
	err := &pkgerrors.AppError{
		Category: pkgerrors.CategoryAI,
		Code:     testErrCode,
		Message:  testErrMsg,
		Cause:    cause,
	}

	got := err.Unwrap()
	if !errors.Is(got, cause) {
		t.Errorf("AppError.Unwrap() = %v, want %v", got, cause)
	}
}

func TestErrorConstructors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		category pkgerrors.Category
	}{
		{
			name:     "ErrParseTemplateFailed",
			err:      pkgerrors.ErrParseTemplateFailed(errors.New("test")),
			category: pkgerrors.CategoryAI,
		},
		{
			name:     "ErrExecuteTemplateFailed",
			err:      pkgerrors.ErrExecuteTemplateFailed(errors.New("test")),
			category: pkgerrors.CategoryParsing,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			var appErr *pkgerrors.AppError
			if !errors.As(testCase.err, &appErr) {
				t.Errorf("%s should return AppError type", testCase.name)

				return
			}

			if appErr.Category != testCase.category {
				t.Errorf("%s category = %v, want %v", testCase.name, appErr.Category, testCase.category)
			}

			if appErr.Code == "" {
				t.Errorf("%s should have non-empty code", testCase.name)
			}

			if appErr.Message == "" {
				t.Errorf("%s should have non-empty message", testCase.name)
			}
		})
	}
}
