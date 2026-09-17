package expense

import "testing"

func TestCentsToYuan(t *testing.T) {
	cases := []struct {
		cents int64
		want  string
	}{
		{0, "0.00"},
		{5, "0.05"},
		{105, "1.05"},
		{12000, "120.00"},
		{12050, "120.50"},
		{99, "0.99"},
		{-50, "-0.50"},
		{-12000, "-120.00"},
	}
	for _, tc := range cases {
		if got := centsToYuan(tc.cents); got != tc.want {
			t.Errorf("centsToYuan(%d) = %q, want %q", tc.cents, got, tc.want)
		}
	}
}

func TestCentsMapToYuan(t *testing.T) {
	m := map[string]int64{"食材": 12000, "日用": 305}
	out := centsMapToYuan(m)
	if out["食材"] != "120.00" || out["日用"] != "3.05" {
		t.Fatalf("map 转换错误: %v", out)
	}
}
