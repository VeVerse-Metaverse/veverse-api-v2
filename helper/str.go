package helper

import (
	"regexp"
	"unicode"
)

func HasUppercase(str string) bool {
	for _, r := range str {
		if unicode.IsUpper(r) {
			return true
		}
	}

	return false
}

func HasLowercase(str string) bool {
	for _, r := range str {
		if unicode.IsLower(r) {
			return true
		}
	}

	return false
}

func HasNumber(str string) bool {
	return regexp.MustCompile(`\d`).MatchString(str)
}
