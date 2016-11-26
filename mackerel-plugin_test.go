package mackerelplugin

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
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

// an example implementation
type testMemcachedPlugin struct {
}

func (m testMemcachedPlugin) GraphDefinition() map[string]Graphs {
	return map[string]Graphs{
		"memcached.cmd": {
			Label: "Memcached Command",
			Unit:  "integer",
			Metrics: []Metrics{
				{Name: "cmd_get", Label: "Get"},
			},
		},
	}
}

func (m testMemcachedPlugin) FetchMetrics() (map[string]float64, error) {
	return map[string]float64{
		"cmd_get": 11.0,
		"cmd_set": 8.0,
	}, nil
}

func TestOutputDefinitions(t *testing.T) {
	var m testMemcachedPlugin
	mp := NewMackerelPlugin(m)
	wtr := &bytes.Buffer{}
	mp.writer = wtr
	mp.OutputDefinitions()

	expect := `# mackerel-agent-plugin
{"graphs":{"memcached.cmd":{"label":"Memcached Command","unit":"integer","metrics":[{"name":"cmd_get","label":"Get","stacked":false}]}}}
`
	got := wtr.String()
	if got != expect {
		t.Errorf("result of OutputDefinitions is invalid :%s", got)
	}
}

func TestOutputValues(t *testing.T) {
	var m testMemcachedPlugin
	mp := NewMackerelPlugin(m)
	wtr := &bytes.Buffer{}
	mp.writer = wtr
	mp.OutputValues()
	epoch := time.Now().Unix()
	expect := fmt.Sprintf("memcached.cmd.cmd_get\t%d\t%d\n", 11, epoch)
	got := wtr.String()
	if got != expect {
		t.Errorf("result of OutputValues is invalid :%s", got)
	}
}

type testP struct{}

func (t testP) FetchMetrics() (map[string]float64, error) {
	return map[string]float64{
		"bar": 15.0,
		"baz": 18.0,
	}, nil
}

func (t testP) GraphDefinition() map[string]Graphs {
	return map[string]Graphs{
		"": {
			Unit: "integer",
			Metrics: []Metrics{
				{Name: "bar"},
			},
		},
		"fuga": {
			Unit: "float",
			Metrics: []Metrics{
				{Name: "baz"},
			},
		},
	}
}

func (t testP) MetricKeyPrefix() string {
	return "testP"
}

func TestDefaultTempfile(t *testing.T) {
	mp := &MackerelPlugin{}
	filename := filepath.Base(os.Args[0])
	expect := filepath.Join(os.TempDir(), fmt.Sprintf("mackerel-plugin-%s", filename))
	if mp.tempfilename() != expect {
		t.Errorf("mp.tempfilename() should be %s, but: %s", expect, mp.tempfilename())
	}

	pPrefix := NewMackerelPlugin(testP{})
	expectForPrefix := filepath.Join(os.TempDir(), "mackerel-plugin-testP")
	if pPrefix.tempfilename() != expectForPrefix {
		t.Errorf("pPrefix.tempfilename() should be %s, but: %s", expectForPrefix, pPrefix.tempfilename())
	}
}

func TestTempfilenameFromExecutableFilePath(t *testing.T) {
	mp := &MackerelPlugin{}

	wd, _ := os.Getwd()
	// not PluginWithPrefix, regular filename
	expect1 := filepath.Join(os.TempDir(), "mackerel-plugin-foobar")
	filename1 := mp.generateTempfilePath(filepath.Join(wd, "foobar"))
	if filename1 != expect1 {
		t.Errorf("p.generateTempfilePath() should be %s, but: %s", expect1, filename1)
	}

	// not PluginWithPrefix, contains some characters to be sanitized
	expect2 := filepath.Join(os.TempDir(), "mackerel-plugin-some_sanitized_name_1.2")
	filename2 := mp.generateTempfilePath(filepath.Join(wd, "some sanitized:name+1.2"))
	if filename2 != expect2 {
		t.Errorf("p.generateTempfilePath() should be %s, but: %s", expect2, filename2)
	}

	// not PluginWithPrefix, begins with "mackerel-plugin-"
	expect3 := filepath.Join(os.TempDir(), "mackerel-plugin-trimmed")
	filename3 := mp.generateTempfilePath(filepath.Join(wd, "mackerel-plugin-trimmed"))
	if filename3 != expect3 {
		t.Errorf("p.generateTempfilePath() should be %s, but: %s", expect3, filename3)
	}

	// PluginWithPrefix ignores current filename
	pPrefix := NewMackerelPlugin(testP{})
	expectForPrefix := filepath.Join(os.TempDir(), "mackerel-plugin-testP")
	filenameForPrefix := pPrefix.generateTempfilePath(filepath.Join(wd, "foo"))
	if filenameForPrefix != expectForPrefix {
		t.Errorf("pPrefix.generateTempfilePath() should be %s, but: %s", expectForPrefix, filenameForPrefix)
	}
}

func TestPluginOutputDefinitionsWithPrefix(t *testing.T) {
	mp := NewMackerelPlugin(testP{})
	wtr := &bytes.Buffer{}
	mp.writer = wtr
	mp.OutputDefinitions()
	expect := `# mackerel-agent-plugin
{"graphs":{"testP":{"label":"TestP","unit":"integer","metrics":[{"name":"bar","label":"Bar","stacked":false}]},"testP.fuga":{"label":"TestP Fuga","unit":"float","metrics":[{"name":"baz","label":"Baz","stacked":false}]}}}
`
	got := wtr.String()
	if got != expect {
		t.Errorf("result of OutputDefinitions is invalid: %s", got)
	}
}

func TestOutputValuesWithPrefix(t *testing.T) {
	mp := NewMackerelPlugin(testP{})
	wtr := &bytes.Buffer{}
	mp.writer = wtr
	mp.OutputValues()
	epoch := time.Now().Unix()
	expect := fmt.Sprintf("testP.bar\t15\t%[1]d\ntestP.fuga.baz\t18\t%[1]d\n", epoch)
	got := wtr.String()
	if got != expect {
		t.Errorf("result of OutputValues is invalid :%s", got)
	}
}
