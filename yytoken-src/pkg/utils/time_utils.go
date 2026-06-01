package utils

import (
	"time"

	"github.com/gogf/gf/v2/os/gtime"
)

func GetShanghaiTime() time.Time {
	return gtime.Now().Time
}

func GetShanghaiLocation() *time.Location {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	return loc
}

func GetTodayDateString() string {
	return gtime.Now().Format("Y-m-d")
}

func GetCurrentRound(now time.Time) int {
	hour := now.Hour()
	minute := now.Minute()

	switch {
	case hour < 10 || (hour == 9 && minute < 50):
		return 1
	case hour < 14 || (hour == 13 && minute < 50):
		return 2
	case hour < 18 || (hour == 17 && minute < 50):
		return 3
	case hour < 22 || (hour == 21 && minute < 50):
		return 4
	default:
		return 0
	}
}
