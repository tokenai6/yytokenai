package utils

import (
	"XWFrame/internal/frame/consts"
	"time"

	"github.com/gogf/gf/v2/os/gtime"
)

var dbTimestampLocation = func() *time.Location {
	loc, _ := time.LoadLocation(consts.TimezoneUTC8)
	return loc
}()

// FormatTime 格式化时间
func FormatTime(t time.Time, format string) string {
	return t.Format(format)
}

// FormatDateTime 格式化日期时间
func FormatDateTime(t time.Time) string {
	return t.Format(consts.TimeFormatDateTime)
}

// FormatDate 格式化日期
func FormatDate(t time.Time) string {
	return t.Format(consts.TimeFormatDate)
}

// FormatTimeOnly 格式化时间
func FormatTimeOnly(t time.Time) string {
	return t.Format(consts.TimeFormatTime)
}

// ParseTime 解析时间字符串
func ParseTime(timeStr string, format string) (time.Time, error) {
	return time.Parse(format, timeStr)
}

// ParseDateTime 解析日期时间字符串
func ParseDateTime(timeStr string) (time.Time, error) {
	return time.Parse(consts.TimeFormatDateTime, timeStr)
}

// ParseDate 解析日期字符串
func ParseDate(timeStr string) (time.Time, error) {
	return time.Parse(consts.TimeFormatDate, timeStr)
}

// GetCurrentTime 获取当前时间
func GetCurrentTime() time.Time {
	return time.Now()
}

// GetCurrentTimestamp 获取当前时间戳（秒）
func GetCurrentTimestamp() int64 {
	return time.Now().Unix()
}

// GetCurrentTimestampMs 获取当前时间戳（毫秒）
func GetCurrentTimestampMs() int64 {
	return time.Now().UnixMilli()
}

// TimestampToTime 时间戳转时间
func TimestampToTime(timestamp int64) time.Time {
	return time.Unix(timestamp, 0)
}

// TimeToTimestamp 时间转时间戳
func TimeToTimestamp(t time.Time) int64 {
	return t.Unix()
}

// AddDays 添加天数
func AddDays(t time.Time, days int) time.Time {
	return t.AddDate(0, 0, days)
}

// AddHours 添加小时
func AddHours(t time.Time, hours int) time.Time {
	return t.Add(time.Duration(hours) * time.Hour)
}

// AddMinutes 添加分钟
func AddMinutes(t time.Time, minutes int) time.Time {
	return t.Add(time.Duration(minutes) * time.Minute)
}

// AddSeconds 添加秒数
func AddSeconds(t time.Time, seconds int) time.Time {
	return t.Add(time.Duration(seconds) * time.Second)
}

// IsSameDay 判断是否为同一天
func IsSameDay(t1, t2 time.Time) bool {
	return t1.Year() == t2.Year() && t1.YearDay() == t2.YearDay()
}

// IsSameMonth 判断是否为同一月
func IsSameMonth(t1, t2 time.Time) bool {
	return t1.Year() == t2.Year() && t1.Month() == t2.Month()
}

// IsSameYear 判断是否为同一年
func IsSameYear(t1, t2 time.Time) bool {
	return t1.Year() == t2.Year()
}

// GetDaysBetween 获取两个时间之间的天数
func GetDaysBetween(t1, t2 time.Time) int {
	return int(t2.Sub(t1).Hours() / 24)
}

// GetHoursBetween 获取两个时间之间的小时数
func GetHoursBetween(t1, t2 time.Time) int {
	return int(t2.Sub(t1).Hours())
}

// GetMinutesBetween 获取两个时间之间的分钟数
func GetMinutesBetween(t1, t2 time.Time) int {
	return int(t2.Sub(t1).Minutes())
}

// GetSecondsBetween 获取两个时间之间的秒数
func GetSecondsBetween(t1, t2 time.Time) int {
	return int(t2.Sub(t1).Seconds())
}

// GetDayStartTime 获取某一天的开始时间（00:00:00）
// 参数：gtime对象或time对象
// 返回：该日期的开始时间（时分秒为0）
func GetDayStartTime(date interface{}) time.Time {
	var t time.Time
	switch v := date.(type) {
	case *gtime.Time:
		t = v.Time
	case gtime.Time:
		t = v.Time
	case time.Time:
		t = v
	default:
		t = time.Now()
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// GetDayEndTime 获取某一天的结束时间（23:59:59）
// 参数：gtime对象或time对象
// 返回：该日期的结束时间（23:59:59）
func GetDayEndTime(date interface{}) time.Time {
	var t time.Time
	switch v := date.(type) {
	case *gtime.Time:
		t = v.Time
	case gtime.Time:
		t = v.Time
	case time.Time:
		t = v
	default:
		t = time.Now()
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 999999999, t.Location())
}

// GetYesterdayStartTime 获取昨天的开始时间（00:00:00）
func GetYesterdayStartTime() time.Time {
	yesterday := gtime.Now().AddDate(0, 0, -1)
	return GetDayStartTime(yesterday)
}

// GetYesterdayEndTime 获取昨天的结束时间（23:59:59）
func GetYesterdayEndTime() time.Time {
	yesterday := gtime.Now().AddDate(0, 0, -1)
	return GetDayEndTime(yesterday)
}

// GetDayStartTimeByOffset 获取指定偏移天数的开始时间（00:00:00）
// offset: 天数偏移，负数表示过去，正数表示未来
func GetDayStartTimeByOffset(offset int) time.Time {
	targetDay := gtime.Now().AddDate(0, 0, offset)
	return GetDayStartTime(targetDay)
}

// GetDayEndTimeByOffset 获取指定偏移天数的结束时间（23:59:59）
// offset: 天数偏移，负数表示过去，正数表示未来
func GetDayEndTimeByOffset(offset int) time.Time {
	targetDay := gtime.Now().AddDate(0, 0, offset)
	return GetDayEndTime(targetDay)
}

// DBTimestampToUnix 将数据库 TIMESTAMP (无时区) 转换为 Unix 时间戳
func DBTimestampToUnix(t time.Time) int64 {
	correctTime := time.Date(
		t.Year(), t.Month(), t.Day(),
		t.Hour(), t.Minute(), t.Second(),
		t.Nanosecond(), dbTimestampLocation,
	)
	return correctTime.Unix()
}
