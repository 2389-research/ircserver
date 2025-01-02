package server

import (
	"strings"
	"unicode"
)

func isValidChannelName(name string) bool {
	if len(name) < 2 || !strings.HasPrefix(name, "#") {
		return false
	}
	// Check for spaces and other invalid characters
	for _, r := range name[1:] {
		if !unicode.IsLetter(r) && !unicode.IsNumber(r) && r != '-' && r != '_' {
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
