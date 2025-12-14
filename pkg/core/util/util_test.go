package util

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
	"testing"
)

func TestCHU(t *testing.T) {
	t.Log(100035 / 100000)
}

func TestBuilder(t *testing.T) {
	var builder strings.Builder

	builder.WriteString("ni")
	builder.WriteString("hao")
	t.Log(builder.String())
}

func TestFloat(t *testing.T) {
	float := strconv.FormatFloat(120.267812, 'f', 6, 64)
	t.Log(float)
	f := 0.129999999 * 100
	t.Log(int64(f))

}

func TestFmt(t *testing.T) {
	//unix := time.Now().Unix()
	//i := int64(99999999) * 10000000000
	//unix = i + unix
	//t.Log(i)
	//t.Log(unix)
	//int63 := rand.New(rand.NewSource(int64(unix))).Int63()
	//t.Log(strconv.FormatInt(int63, 10)[:4])

	//m := make(map[string]int64)
	//
	//for i := 0; i < 100000000; i++ {
	//	int63 := rand.New(rand.NewSource(int64(i))).Int63()
	//	formatInt := strconv.FormatInt(int63, 10)
	//	s := formatInt[:4]
	//	if m[s] == 1 {
	//		t.Log(i)
	//	}
	//	m[s] = 1
	//}

	sprintf := fmt.Sprintf("%04X", 65535)
	t.Log(sprintf)
	t.Log(fmt.Sprintf("%04X", 100))
}

func TestInt2String(t *testing.T) {
	ids := []int64{1, 2, 3}

	var builder strings.Builder
	for _, id := range ids {
		bs := make([]byte, 8)
		binary.LittleEndian.PutUint64(bs, uint64(id))
		builder.Write(bs)
	}
	t.Log(builder.String())
}
