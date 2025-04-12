package util

import (
	"errors"
	"testing"
)

func TestErrorCode_Error(t *testing.T) {
	tests := []struct {
		name string
		e    ErrorCode
		want string
	}{
		{"TestPathNotFoundCode", ErrCodePathNotFound, "Path not found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.e.Error(); got != tt.want {
				t.Errorf("ErrorCode.Error() = %v, want %v", got, tt.want)
			}
		})
	}

	t.Run("UnknownErrorCode", func(t *testing.T) {
		e := ErrorCode(9999) // An unknown error code
		want := "Unknown error"
		if got := e.Error(); got != want {
			t.Errorf("ErrorCode.Error() = %v, want %v", got, want)
		}
	})
}

func TestErrorCode_NewError(t *testing.T) {
	tests := []struct {
		name string
		e    ErrorCode
		want *Error
	}{
		{"TestNewError", ErrCodePathNotFound, &Error{Code: ErrCodePathNotFound, Message: "Path not found"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewError(tt.e); !errors.Is(got.Code, tt.want.Code) || got.Message != tt.want.Message {
				t.Errorf("ErrorCode.NewError() = %v, want %v", got, tt.want)
			}
		})
	}

	t.Run("TestNewErrorWithAppend", func(t *testing.T) {
		e := NewError(ErrCodePathNotFound, "additional info")
		want := &Error{Code: ErrCodePathNotFound, Message: "Path not found : additional info"}
		if !errors.Is(e.Code, want.Code) || e.Message != want.Message {
			t.Errorf("ErrorCode.NewError() = %v, want %v", e, want)
		}
	})
}

func TestErrorCode_NewUnknownError(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    *Error
	}{
		{"TestNewUnknownError", "Unknown error", &Error{Code: 9999, Message: "Unknown error"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewUnknownError(tt.message); !errors.Is(got.Code, tt.want.Code) || got.Message != tt.want.Message {
				t.Errorf("ErrorCode.NewUnknownError() = %v, want %v", got, tt.want)
			}
		})
	}
}
