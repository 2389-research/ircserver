package server

import (
	"errors"
	"testing"
)

func TestNewError(t *testing.T) {
	tests := []struct {
		name       string
		code       string
		customMsg  string
		ctx        error
		wantCode   string
		wantMsg    string
		wantString string
	}{
		{
			name:       "Standard error no context",
			code:       ErrNoNicknameGiven,
			customMsg:  "",
			ctx:        nil,
			wantCode:   "431",
			wantMsg:    "No nickname given",
			wantString: "431 No nickname given",
		},
		{
			name:       "Custom message",
			code:       ErrNoSuchChannel,
			customMsg:  "Channel #test not found",
			ctx:        nil,
			wantCode:   "403",
			wantMsg:    "Channel #test not found",
			wantString: "403 Channel #test not found",
		},
		{
			name:       "With context error",
			code:       ErrFileError,
			customMsg:  "Failed to open log",
			ctx:        errors.New("permission denied"),
			wantCode:   "424",
			wantMsg:    "Failed to open log",
			wantString: "424 Failed to open log: permission denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewError(tt.code, tt.customMsg, tt.ctx)
			
			if err.Code != tt.wantCode {
				t.Errorf("NewError().Code = %v, want %v", err.Code, tt.wantCode)
			}
			
			if tt.customMsg == "" && err.Message != ErrorMessages[tt.code] {
				t.Errorf("NewError().Message = %v, want %v", err.Message, ErrorMessages[tt.code])
			}
			
			if tt.customMsg != "" && err.Message != tt.customMsg {
				t.Errorf("NewError().Message = %v, want %v", err.Message, tt.customMsg)
			}
			
			if err.Error() != tt.wantString {
				t.Errorf("NewError().Error() = %v, want %v", err.Error(), tt.wantString)
			}
		})
	}
}

func TestFormatError(t *testing.T) {
	tests := []struct {
		name       string
		serverName string
		code       string
		nickname   string
		customMsg  string
		want       string
	}{
		{
			name:       "Standard error message",
			serverName: "irc.test.net",
			code:       ErrNoNicknameGiven,
			nickname:   "user",
			customMsg:  "",
			want:       ":irc.test.net 431 user :No nickname given",
		},
		{
			name:       "Custom error message",
			serverName: "irc.test.net",
			code:       ErrNoSuchChannel,
			nickname:   "user",
			customMsg:  "Channel #test does not exist",
			want:       ":irc.test.net 403 user :Channel #test does not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatError(tt.serverName, tt.code, tt.nickname, tt.customMsg)
			if got != tt.want {
				t.Errorf("FormatError() = %v, want %v", got, tt.want)
			}
		})
	}
}
