package server

import (
	"strings"
	"unicode"
)

func isValidChannelName(name string) bool {
	// RFC 1459: Channel names are strings (beginning with a '&' or '#' character) of
	// length up to 200 characters. Apart from the requirement that the first character
	// being either '&' or '#', the only restriction on a channel name is that it may
	// not contain any spaces (' '), a control G (^G or ASCII 7), or a comma (',' which
	// is used as a list item separator by the protocol).
	if len(name) < 2 || len(name) > 200 {
		return false
	}
	
	if !strings.HasPrefix(name, "#") && !strings.HasPrefix(name, "&") {
		return false
	}

	// Check for spaces and other invalid characters
	for _, r := range name[1:] {
		if r == ' ' || r == ',' || r == '\a' {
			return false
		}
	}
	return true
}

func isValidNick(nick string) bool {
	// RFC 1459: nicknames must be max 9 chars
	if len(nick) > 9 {
		return false
	}
	
	// Must start with a letter
	if len(nick) == 0 || !unicode.IsLetter(rune(nick[0])) {
		return false
	}

	// Can only contain letters, numbers, - and _
	for _, r := range nick[1:] {
		if !unicode.IsLetter(r) && !unicode.IsNumber(r) && r != '-' && r != '_' {
			return false
		}
	}
	return true
}
