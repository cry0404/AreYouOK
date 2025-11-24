package utils

import (
	"time"
)

// parseTime 解析时间字符串（格式：HH:MM:SS）并应用到指定日期
func ParseTime(timeStr string, date time.Time) (time.Time, error) {
	if timeStr == "" {
		return date, nil
	}

	parsedTime, err := time.Parse("15:04:05", timeStr)
	if err != nil {
		return date, err
	}

	return time.Date(
		date.Year(),
		date.Month(),
		date.Day(),
		parsedTime.Hour(),
		parsedTime.Minute(),
		parsedTime.Second(),
		0,
		date.Location(),
	), nil
}
