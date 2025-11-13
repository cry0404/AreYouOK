package utils

import (
	"regexp"
)

func ValidatePhone(phone string) bool {
	matched, err := regexp.MatchString(`^1[3-9]\d{9}$`, phone)
	if err != nil {
		return false
	}
	return matched
}
