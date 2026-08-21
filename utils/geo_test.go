package utils

import (
	"math"
	"testing"
)

// geoEpsilon 单点转换的比较容差。1e-9 度约合 0.1 毫米，足以锁死算法行为，
// 又不会被不同平台/Go 版本的浮点末位差异误伤（实测与公认基准差约 1e-14）。
const geoEpsilon = 1e-9

// geoRoundTripEpsilon 往返转换的比较容差。
//
// BD09ToGCJ02 是 GCJ02ToBD09 的近似逆而非精确逆：两者的正弦/余弦修正项都用
// 各自的输入坐标计算，往返时这两个修正量并不严格抵消。实测残差约 4e-7 度
// （约 4 厘米），这是该算法族的固有精度，不是实现缺陷。
const geoRoundTripEpsilon = 1e-6

// assertClose 按容差比较两个坐标分量，失败时打印实际差值便于定位。
func assertClose(t *testing.T, label string, got, want, eps float64) {
	t.Helper()
	if diff := math.Abs(got - want); diff > eps {
		t.Errorf("%s = %.14f, want %.14f（差值 %.3g 超出容差 %.3g）", label, got, want, diff, eps)
	}
}

// 注意参数顺序是 (经度, 纬度)。中国境内经度约 73~135、纬度约 3~54，
// 两者数值范围不重叠——若某个用例的「经度」看起来像纬度，多半是写反了。
// 本测试此前的用例就把上海的经纬度对调了，且期望值是 IDE 生成的占位 0。
var gcj02ToBD09Cases = []struct {
	name   string
	gcjLon float64
	gcjLat float64
	bdLon  float64
	bdLat  float64
}{
	{
		// 天安门，坐标系转换领域被广泛引用的基准点
		name:   "天安门",
		gcjLon: 116.404, gcjLat: 39.915,
		bdLon: 116.41036949371031, bdLat: 39.92133699351022,
	},
	{
		name:   "上海",
		gcjLon: 121.34895713, gcjLat: 31.40778628,
		bdLon: 121.35543275545923, bdLat: 31.41380117104878,
	},
}

func TestGCJ02ToBD09(t *testing.T) {
	for _, tt := range gcj02ToBD09Cases {
		t.Run(tt.name, func(t *testing.T) {
			gotLon, gotLat := GCJ02ToBD09(tt.gcjLon, tt.gcjLat)
			assertClose(t, "bdLon", gotLon, tt.bdLon, geoEpsilon)
			assertClose(t, "bdLat", gotLat, tt.bdLat, geoEpsilon)
		})
	}
}

func TestBD09ToGCJ02(t *testing.T) {
	for _, tt := range gcj02ToBD09Cases {
		t.Run(tt.name, func(t *testing.T) {
			gotLon, gotLat := BD09ToGCJ02(tt.bdLon, tt.bdLat)
			// 用往返容差而非单点容差：这里是拿正向的输出反推输入，
			// 精度上限由近似逆本身决定。
			assertClose(t, "gcjLon", gotLon, tt.gcjLon, geoRoundTripEpsilon)
			assertClose(t, "gcjLat", gotLat, tt.gcjLat, geoRoundTripEpsilon)
		})
	}
}

// 往返一致性：两个方向互为近似逆，残差必须稳定在厘米级。
// 若某次改动让残差显著变大，说明有一侧的修正项被改坏了。
func TestGeoConversionRoundTrip(t *testing.T) {
	for _, tt := range gcj02ToBD09Cases {
		t.Run(tt.name, func(t *testing.T) {
			bdLon, bdLat := GCJ02ToBD09(tt.gcjLon, tt.gcjLat)
			backLon, backLat := BD09ToGCJ02(bdLon, bdLat)
			assertClose(t, "往返经度", backLon, tt.gcjLon, geoRoundTripEpsilon)
			assertClose(t, "往返纬度", backLat, tt.gcjLat, geoRoundTripEpsilon)
		})
	}
}

// BD-09 相对 GCJ-02 的偏移方向是固定的（+0.0065, +0.006 再叠加旋转缩放），
// 固化这一事实，防止有人把偏移量的符号改反——那种错误在容差比较里不明显，
// 但会让所有坐标系统性偏移约 700 米。
func TestGCJ02ToBD09ShiftsNortheast(t *testing.T) {
	for _, tt := range gcj02ToBD09Cases {
		t.Run(tt.name, func(t *testing.T) {
			bdLon, bdLat := GCJ02ToBD09(tt.gcjLon, tt.gcjLat)
			if bdLon <= tt.gcjLon {
				t.Errorf("BD-09 经度 %.14f 未大于 GCJ-02 经度 %.14f，偏移方向可能被改反", bdLon, tt.gcjLon)
			}
			if bdLat <= tt.gcjLat {
				t.Errorf("BD-09 纬度 %.14f 未大于 GCJ-02 纬度 %.14f，偏移方向可能被改反", bdLat, tt.gcjLat)
			}
		})
	}
}
