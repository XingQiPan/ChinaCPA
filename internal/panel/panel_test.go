package panel

import (
	"testing"

	"chinacpa/internal/pubmodel"
)

func TestParseCreditsFactor(t *testing.T) {
	cases := []struct {
		in   string
		want float64
		ok   bool
	}{
		{"0.1x", 0.1, true},
		{"3x", 3, true},
		{"免费", 0, true},
		{"Free", 0, true},
		{"1.8x", 1.8, true},
		{"x0.11", 0.11, true},
		{"x0.06", 0.06, true},
		{"", 0, false},
		{"abc", 0, false},
	}
	for _, c := range cases {
		got, ok := parseCreditsFactor(c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("parseCreditsFactor(%q) = (%v,%v) 期望 (%v,%v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestMillionCreditBase(t *testing.T) {
	// 无单价：默认 输入1 输出4 缓存0.1
	im, om, cm := millionCreditBase(nil)
	if im != 1 || om != 4 || cm != 0.1 {
		t.Errorf("无单价 base = (%v,%v,%v) 期望 (1,4,0.1)", im, om, cm)
	}
	// 有单价 input=1 output=4 cache=0.1 → 归一后 (1, 4, 0.1)
	p := &pubmodel.Pricing{Input: 1, Output: 4, Cache: 0.1}
	im, om, cm = millionCreditBase(p)
	if im != 1 || om != 4 || cm != 0.1 {
		t.Errorf("单价 base = (%v,%v,%v) 期望 (1,4,0.1)", im, om, cm)
	}
	// 有单价 input=2 output=8 cache=0.2（同比例）→ (1,4,0.1)
	p2 := &pubmodel.Pricing{Input: 2, Output: 8, Cache: 0.2}
	im, om, cm = millionCreditBase(p2)
	if im != 1 || om != 4 || cm != 0.1 {
		t.Errorf("缩放单价 base = (%v,%v,%v) 期望 (1,4,0.1)", im, om, cm)
	}
	// 只设输入 input=7 → 输出4 缓存0.1
	p3 := &pubmodel.Pricing{Input: 7}
	im, om, cm = millionCreditBase(p3)
	if im != 1 || om != 4 || cm != 0.1 {
		t.Errorf("仅输入 base = (%v,%v,%v) 期望 (1,4,0.1)", im, om, cm)
	}
}

func TestRound2(t *testing.T) {
	if got := round2(0.025); got != 0.03 {
		t.Errorf("round2(0.025)=%v 期望 0.03", got)
	}
	if got := round2(0.0015); got != 0.0 {
		t.Errorf("round2(0.0015)=%v 期望 0.0", got)
	}
	if got := round2(3.14159); got != 3.14 {
		t.Errorf("round2(3.14159)=%v 期望 3.14", got)
	}
}