package mackerelplugin

import (
	"math"
	"testing"
	"time"
)

func TestCalcDiff(t *testing.T) {
	var mp MackerelPlugin

	val1 := 10.0
	val2 := 0.0
	now := time.Now()
	last := time.Unix(now.Unix()-10, 0)

	diff, err := mp.calcDiff(val1, now, val2, last, "")
	if diff != 60 {
		t.Errorf("calcDiff: %f should be %f", diff, 60.0)
	}
	if err != nil {
		t.Error("calcDiff causes an error")
	}
}

func TestCalcDiffWithUInt32OverflowWithSigned(t *testing.T) {
	var mp MackerelPlugin

	val := 10.0
	now := time.Now()
	lastval := (math.MaxUint32) - 10.0
	last := time.Unix(now.Unix()-60, 0)

	diff, err := mp.calcDiff(val, now, lastval, last, "int32")
	if diff > 0 {
		t.Errorf("calcDiff: last: %f, now: %f, %f should be negative", val, lastval, diff)
	}
	if err != nil {
		t.Error("calcDiff causes an error")
	}
}

func TestCalcDiffWithUInt32Overflow(t *testing.T) {
	var mp MackerelPlugin

	val := 10.0
	now := time.Now()
	lastval := (math.MaxUint32) - 10.0
	last := time.Unix(now.Unix()-60, 0)

	diff, err := mp.calcDiff(val, now, lastval, last, "uint32")
	if diff != 20 {
		t.Errorf("calcDiff: last: %f, now: %f, %f should be %f", val, lastval, diff, 20.0)
	}
	if err != nil {
		t.Error("calcDiff causes an error")
	}
}
