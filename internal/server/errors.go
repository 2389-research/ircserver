package server

import (
	"fmt"
)

// IRC error codes as per RFC 2812
const (
	ErrNoSuchNick          = "401"
	ErrNoSuchChannel       = "403"
	ErrCannotSendToChan    = "404"
	ErrTooManyChannels     = "405"
	ErrNoOrigin            = "409"
	ErrNoRecipient        = "411"
	ErrNoTextToSend       = "412"
	ErrUnknownCommand     = "421"
	ErrNoNicknameGiven    = "431"
	ErrNicknameInUse      = "433"
	ErrNotOnChannel       = "442"
	ErrNeedMoreParams     = "461"
)

// IRCError represents a standard IRC error
type IRCError struct {
	Code    string
	Message string
	Context error
}

func (e *IRCError) Error() string {
	if e.Context != nil {
		return fmt.Sprintf("%s %s: %v", e.Code, e.Message, e.Context)
	}
	return fmt.Sprintf("%s %s", e.Code, e.Message)
}

// NewError creates a new IRC error
func NewError(code string, message string, ctx error) *IRCError {
	return &IRCError{
		Code:    code,
		Message: message,
		Context: ctx,
	}
}
