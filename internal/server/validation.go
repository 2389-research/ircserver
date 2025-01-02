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
	for _, r := range nick {
		if !unicode.IsLetter(r) && !unicode.IsNumber(r) && r != '-' && r != '_' {
			return false
		}
	}
	return true
}
