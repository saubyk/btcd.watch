package chain

import (
	"testing"

	"github.com/btcsuite/btcd/chaincfg/v2"
)

func TestParseBlockHeight(t *testing.T) {
	cases := []struct {
		in     string
		height int64
		ok     bool
	}{
		{"842317", 842317, true},
		{"842,317", 842317, true},
		{" 0 ", 0, true},
		{"", 0, false},
		{"abc", 0, false},
		{"12a4", 0, false},
		{"-5", 0, false},
		// 64 all-digit hex strings and other overlong numbers are not
		// plausible heights.
		{"1111111111111111111111111111111111111111111111111111111111111111", 0, false},
		{"12345678901", 0, false},
	}
	for _, c := range cases {
		height, ok := ParseBlockHeight(c.in)
		if height != c.height || ok != c.ok {
			t.Errorf("ParseBlockHeight(%q) = %d,%v want %d,%v",
				c.in, height, ok, c.height, c.ok)
		}
	}
}

func TestBlockSubsidy(t *testing.T) {
	params := &chaincfg.RegressionNetParams // halving interval 150
	cases := []struct {
		height int64
		want   int64
	}{
		{0, 50_0000_0000},
		{149, 50_0000_0000},
		{150, 25_0000_0000},
		{300, 12_5000_0000},
		{150 * 64, 0},
	}
	for _, c := range cases {
		if got := BlockSubsidy(c.height, params); got != c.want {
			t.Errorf("BlockSubsidy(%d) = %d, want %d", c.height, got, c.want)
		}
	}
}
