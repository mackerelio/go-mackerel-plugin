package mackerelplugin

import (
	"bytes"
	"testing"
	"time"
)

func TestCalcDiff(t *testing.T) {
	var mp *MackerelPlugin

	val1 := 10.0
	val2 := 0.0
	now := time.Now()
	last := time.Unix(now.Unix()-10, 0)

	diff, err := mp.calcDiff(val1, now, val2, last)
	if diff != 60.0 {
		t.Errorf("calcDiff: %f should be %f", diff, 60.0)
	}
	if err != nil {
		t.Error("calcDiff causes an error")
	}
}

func TestCalcDiffWithReset(t *testing.T) {
	var mp *MackerelPlugin

	val := 10.0
	lastval := 12345.0
	now := time.Now()
	last := time.Unix(now.Unix()-60, 0)

	diff, err := mp.calcDiff(val, now, lastval, last)
	if err == nil {
		t.Errorf("calcDiff with counter reset should cause an error: %f", diff)
	}
}

func TestFormatValues(t *testing.T) {
	wtr := &bytes.Buffer{}
	mp := &MackerelPlugin{writer: wtr}

	prefix := "foo"
	metric := Metrics{Name: "cmd_get", Label: "Get", Diff: true}
	stat := map[string]float64{"cmd_get": 1000.0}
	lastStat := map[string]float64{"cmd_get": 500.0, ".last_diff.cmd_get": 300.0}
	now := time.Unix(1437227240, 0)
	lastTime := now.Add(time.Second * (-60))
	mp.formatValues(prefix, metric, stat, lastStat, now, lastTime)

	got := wtr.String()
	expect := "foo.cmd_get	500	1437227240\n"
	if got != expect {
		t.Errorf("result of formatValues is not expected one: %s", got)
	}
}
