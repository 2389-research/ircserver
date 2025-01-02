package server

import (
	"fmt"
)

// IRC error codes as per RFC 1459
const (
	// Connection Registration Errors
	ErrNoNicknameGiven  = "431" // ERR_NONICKNAMEGIVEN
	ErrErroneusNickname = "432" // ERR_ERRONEUSNICKNAME
	ErrNicknameInUse    = "433" // ERR_NICKNAMEINUSE
	ErrNeedMoreParams   = "461" // ERR_NEEDMOREPARAMS
	
	// Channel Operation Errors
	ErrNoSuchChannel    = "403" // ERR_NOSUCHCHANNEL
	ErrCannotSendToChan = "404" // ERR_CANNOTSENDTOCHAN
	ErrTooManyChannels  = "405" // ERR_TOOMANYCHANNELS
	ErrNoRecipient      = "411" // ERR_NORECIPIENT
	ErrNoTextToSend     = "412" // ERR_NOTEXTTOSEND
	ErrNotOnChannel     = "442" // ERR_NOTONCHANNEL
	ErrNotInChannel     = "442" // ERR_NOTONCHANNEL (alias)
	ErrUserOnChannel    = "443" // ERR_USERONCHANNEL
	ErrNicknameInUse    = "433" // ERR_NICKNAMEINUSE
	
	// Server Errors
	ErrNoSuchNick      = "401" // ERR_NOSUCHNICK
	ErrNoOrigin        = "409" // ERR_NOORIGIN
	ErrUnknownCommand  = "421" // ERR_UNKNOWNCOMMAND
	ErrFileError       = "424" // ERR_FILEERROR
	ErrNoAdminInfo     = "423" // ERR_NOADMININFO
	
	// Misc Errors
	ErrUsersDontMatch = "502" // ERR_USERSDONTMATCH
)

// Standard error messages as per RFC 1459
var ErrorMessages = map[string]string{
	ErrNoNicknameGiven:  "No nickname given",
	ErrErroneusNickname: "Erroneous nickname",
	ErrNicknameInUse:    "Nickname is already in use",
	ErrNeedMoreParams:   "Not enough parameters",
	ErrNoSuchChannel:    "No such channel",
	ErrCannotSendToChan: "Cannot send to channel",
	ErrTooManyChannels:  "You have joined too many channels",
	ErrNoRecipient:      "No recipient given",
	ErrNoTextToSend:     "No text to send",
	ErrNotOnChannel:     "You're not on that channel",
	ErrUserOnChannel:    "is already on channel",
	ErrNoSuchNick:      "No such nick/channel",
	ErrNoOrigin:        "No origin specified",
	ErrUnknownCommand:  "Unknown command",
	ErrFileError:       "File error doing operation",
	ErrNoAdminInfo:     "No administrative info available",
	ErrUsersDontMatch:  "Cant change mode for other users",
}

// IRCError represents a standard IRC error.
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

// NewError creates a new IRC error with standard message
func NewError(code string, customMsg string, ctx error) *IRCError {
	msg := customMsg
	if msg == "" {
		// Use standard message if available
		if stdMsg, ok := ErrorMessages[code]; ok {
			msg = stdMsg
		}
	}
	return &IRCError{
		Code:    code,
		Message: msg,
		Context: ctx,
	}
}

// FormatError formats an IRC error message for sending to client
func FormatError(serverName string, code string, nickname string, customMsg string) string {
	msg := customMsg
	if msg == "" {
		if stdMsg, ok := ErrorMessages[code]; ok {
			msg = stdMsg
		}
	}
	return fmt.Sprintf(":%s %s %s :%s", serverName, code, nickname, msg)
}
