package utils

import (
	"time"
)

// 常用时间格式
const (
	DateFormat     = "2006-01-02"
	TimeFormat     = "15:04:05"
	DateTimeFormat = "2006-01-02 15:04:05"
	RFC3339Format  = time.RFC3339
	UnixFormat     = "Mon Jan _2 15:04:05 MST 2006"
)

// Now 获取当前时间
func Now() time.Time {
	return time.Now()
}

// NowUnix 获取当前Unix时间戳
func NowUnix() int64 {
	return time.Now().Unix()
}

// NowUnixMilli 获取当前Unix毫秒时间戳
func NowUnixMilli() int64 {
	return time.Now().UnixMilli()
}

// NowUnixNano 获取当前Unix纳秒时间戳
func NowUnixNano() int64 {
	return time.Now().UnixNano()
}

// FormatTime 格式化时间
func FormatTime(t time.Time, layout string) string {
	return t.Format(layout)
}

// FormatNow 格式化当前时间
func FormatNow(layout string) string {
	return time.Now().Format(layout)
}

// ParseTime 解析时间字符串
func ParseTime(layout, value string) (time.Time, error) {
	return time.Parse(layout, value)
}

// ParseTimeInLocation 在指定时区解析时间字符串
func ParseTimeInLocation(layout, value string, loc *time.Location) (time.Time, error) {
	return time.ParseInLocation(layout, value, loc)
}

// UnixToTime Unix时间戳转Time
func UnixToTime(sec int64) time.Time {
	return time.Unix(sec, 0)
}

// UnixMilliToTime Unix毫秒时间戳转Time
func UnixMilliToTime(msec int64) time.Time {
	return time.UnixMilli(msec)
}

// UnixNanoToTime Unix纳秒时间戳转Time
func UnixNanoToTime(nsec int64) time.Time {
	return time.UnixMicro(nsec / 1000)
}

// AddDays 增加天数
func AddDays(t time.Time, days int) time.Time {
	return t.AddDate(0, 0, days)
}

// AddMonths 增加月数
func AddMonths(t time.Time, months int) time.Time {
	return t.AddDate(0, months, 0)
}

// AddYears 增加年数
func AddYears(t time.Time, years int) time.Time {
	return t.AddDate(years, 0, 0)
}

// BeginOfDay 一天的开始时间
func BeginOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// EndOfDay 一天的结束时间
func EndOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 999999999, t.Location())
}

// BeginOfWeek 一周的开始时间（周一）
func BeginOfWeek(t time.Time) time.Time {
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return BeginOfDay(t.AddDate(0, 0, 1-weekday))
}

// EndOfWeek 一周的结束时间（周日）
func EndOfWeek(t time.Time) time.Time {
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return EndOfDay(t.AddDate(0, 0, 7-weekday))
}

// BeginOfMonth 一月的开始时间
func BeginOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}

// EndOfMonth 一月的结束时间
func EndOfMonth(t time.Time) time.Time {
	return BeginOfMonth(t).AddDate(0, 1, 0).Add(-time.Nanosecond)
}

// BeginOfYear 一年的开始时间
func BeginOfYear(t time.Time) time.Time {
	return time.Date(t.Year(), 1, 1, 0, 0, 0, 0, t.Location())
}

// EndOfYear 一年的结束时间
func EndOfYear(t time.Time) time.Time {
	return time.Date(t.Year(), 12, 31, 23, 59, 59, 999999999, t.Location())
}

// DurationString 将时间间隔转换为可读字符串
func DurationString(d time.Duration) string {
	if d < time.Minute {
		return d.Truncate(time.Second).String()
	}
	if d < time.Hour {
		return d.Truncate(time.Minute).String()
	}
	if d < 24*time.Hour {
		return d.Truncate(time.Hour).String()
	}
	days := d / (24 * time.Hour)
	remaining := d % (24 * time.Hour)
	return (time.Duration(days*24)*time.Hour + remaining.Truncate(time.Hour)).String()
}

// IsLeapYear 判断是否为闰年
func IsLeapYear(year int) bool {
	return year%4 == 0 && (year%100 != 0 || year%400 == 0)
}

// DaysInMonth 获取指定月份的天数
func DaysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// Age 计算年龄
func Age(birthday time.Time) int {
	now := time.Now()
	age := now.Year() - birthday.Year()
	if now.Month() < birthday.Month() || (now.Month() == birthday.Month() && now.Day() < birthday.Day()) {
		age--
	}
	return age
}

// IsSameDay 判断是否为同一天
func IsSameDay(t1, t2 time.Time) bool {
	y1, m1, d1 := t1.Date()
	y2, m2, d2 := t2.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

// IsBefore 判断时间是否在另一个时间之前
func IsBefore(t1, t2 time.Time) bool {
	return t1.Before(t2)
}

// IsAfter 判断时间是否在另一个时间之后
func IsAfter(t1, t2 time.Time) bool {
	return t1.After(t2)
}

// TimeZone 时区相关函数

// GetLocation 获取时区
func GetLocation(name string) (*time.Location, error) {
	return time.LoadLocation(name)
}

// ToUTC 转换为UTC时间
func ToUTC(t time.Time) time.Time {
	return t.UTC()
}

// ToLocal 转换为本地时间
func ToLocal(t time.Time) time.Time {
	return t.Local()
}

// ToLocation 转换到指定时区
func ToLocation(t time.Time, loc *time.Location) time.Time {
	return t.In(loc)
}
