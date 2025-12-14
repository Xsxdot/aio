package utils

import (
	"math"
)

const (
	// 定义坐标系转换的常量
	xPi = math.Pi * 3000.0 / 180.0
)

// GCJ02ToBD09 高德坐标系(GCJ-02)转百度坐标系(BD-09)
// lon: 经度
// lat: 纬度
// 返回转换后的经度和纬度
func GCJ02ToBD09(lon, lat float64) (float64, float64) {
	z := math.Sqrt(lon*lon+lat*lat) + 0.00002*math.Sin(lat*xPi)
	theta := math.Atan2(lat, lon) + 0.000003*math.Cos(lon*xPi)
	bdLon := z*math.Cos(theta) + 0.0065
	bdLat := z*math.Sin(theta) + 0.006
	return bdLon, bdLat
}

// BD09ToGCJ02 百度坐标系(BD-09)转高德坐标系(GCJ-02)
// lon: 经度
// lat: 纬度
// 返回转换后的经度和纬度
func BD09ToGCJ02(lon, lat float64) (float64, float64) {
	x := lon - 0.0065
	y := lat - 0.006
	z := math.Sqrt(x*x+y*y) - 0.00002*math.Sin(y*xPi)
	theta := math.Atan2(y, x) - 0.000003*math.Cos(x*xPi)
	gcjLon := z * math.Cos(theta)
	gcjLat := z * math.Sin(theta)
	return gcjLon, gcjLat
}
