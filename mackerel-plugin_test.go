package mackerelplugin

import (
	"testing"
	"time"
)

func TestCalcDiff(t *testing.T) {
	var mp MackerelPlugin

	val1 := 10.0
	val2 := 0.0
	now := time.Now()
	last := time.Unix(now.Unix()-10, 0)

	diff, err := mp.calcDiff(val1, now, val2, last)
	if diff != 60 {
		t.Errorf("calcDiff: %f should be %f", diff, 60)
	}
	if err != nil {
		t.Error("calcDiff causes an error")
	}
}
