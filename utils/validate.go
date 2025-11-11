package utils


import (
	"regexp"
)

func ValidatePhone(phone string) bool {
	matched, _ := regexp.MatchString(`^1[3-9]\d{9}$`, phone)
	return matched
}