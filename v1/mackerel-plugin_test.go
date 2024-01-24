package v1

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
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
	expect := filepath.Join(os.TempDir(), fmt.Sprintf(
		"mackerel-plugin-%s-%x",
		filename,
		sha1.Sum([]byte(strings.Join(os.Args[1:], " "))),
	))
	if mp.tempfilename() != expect {
		t.Errorf("mp.tempfilename() should be %s, but: %s", expect, mp.tempfilename())
	}

	pPrefix := NewMackerelPlugin(testP{})
	expectForPrefix := filepath.Join(os.TempDir(), fmt.Sprintf(
		"mackerel-plugin-testP-%x",
		sha1.Sum([]byte(strings.Join(os.Args[1:], " "))),
	))
	if pPrefix.tempfilename() != expectForPrefix {
		t.Errorf("pPrefix.tempfilename() should be %s, but: %s", expectForPrefix, pPrefix.tempfilename())
	}
}

func TestTempfilenameFromExecutableFilePath(t *testing.T) {
	mp := &MackerelPlugin{}

	wd, _ := os.Getwd()
	// not PluginWithPrefix, regular filename
	expect1 := filepath.Join(os.TempDir(), "mackerel-plugin-foobar-da39a3ee5e6b4b0d3255bfef95601890afd80709")
	filename1 := mp.generateTempfilePath([]string{filepath.Join(wd, "foobar")})
	if filename1 != expect1 {
		t.Errorf("p.generateTempfilePath() should be %s, but: %s", expect1, filename1)
	}

	// not PluginWithPrefix, contains some characters to be sanitized
	expect2 := filepath.Join(os.TempDir(), "mackerel-plugin-some_sanitized_name_1.2-da39a3ee5e6b4b0d3255bfef95601890afd80709")
	filename2 := mp.generateTempfilePath([]string{filepath.Join(wd, "some sanitized:name+1.2")})
	if filename2 != expect2 {
		t.Errorf("p.generateTempfilePath() should be %s, but: %s", expect2, filename2)
	}

	// not PluginWithPrefix, begins with "mackerel-plugin-"
	expect3 := filepath.Join(os.TempDir(), "mackerel-plugin-trimmed-da39a3ee5e6b4b0d3255bfef95601890afd80709")
	filename3 := mp.generateTempfilePath([]string{filepath.Join(wd, "mackerel-plugin-trimmed")})
	if filename3 != expect3 {
		t.Errorf("p.generateTempfilePath() should be %s, but: %s", expect3, filename3)
	}

	// PluginWithPrefix ignores current filename
	pPrefix := NewMackerelPlugin(testP{})
	expectForPrefix := filepath.Join(os.TempDir(), "mackerel-plugin-testP-da39a3ee5e6b4b0d3255bfef95601890afd80709")
	filenameForPrefix := pPrefix.generateTempfilePath([]string{filepath.Join(wd, "foo")})
	if filenameForPrefix != expectForPrefix {
		t.Errorf("pPrefix.generateTempfilePath() should be %s, but: %s", expectForPrefix, filenameForPrefix)
	}

	// Generate sha1 using command-line options, and use it for filename
	expect5 := filepath.Join(os.TempDir(), "mackerel-plugin-mysql-9045504f8fadd7ddcc8962ec1d9fc70e3f7ba627")
	filename5 := mp.generateTempfilePath([]string{filepath.Join(wd, "mackerel-plugin-mysql"), "-host", "hostname1", "-port", "3306"})
	if filename5 != expect5 {
		t.Errorf("p.generateTempfilePath() should be %s, but: %s", expect5, filename5)
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
	if sortLines(got) != sortLines(expect) {
		t.Errorf("result of OutputValues is invalid :%s", got)
	}
}

type testPHasDiff struct{}

func (t testPHasDiff) FetchMetrics() (map[string]float64, error) {
	return nil, nil
}

func (t testPHasDiff) GraphDefinition() map[string]Graphs {
	return map[string]Graphs{
		"hoge": {
			Metrics: []Metrics{
				{Name: "hoge1", Label: "hoge1", Diff: true},
			},
		},
	}
}

type testPHasntDiff struct{}

func (t testPHasntDiff) FetchMetrics() (map[string]float64, error) {
	return nil, nil
}

func (t testPHasntDiff) GraphDefinition() map[string]Graphs {
	return map[string]Graphs{
		"hoge": {
			Metrics: []Metrics{
				{Name: "hoge1", Label: "hoge1"},
			},
		},
	}
}

func TestPluginHasDiff(t *testing.T) {
	pHasDiff := NewMackerelPlugin(testPHasDiff{})
	if !pHasDiff.hasDiff() {
		t.Errorf("something went wrong")
	}

	pHasntDiff := NewMackerelPlugin(testPHasntDiff{})
	if pHasntDiff.hasDiff() {
		t.Errorf("something went wrong")
	}
}

func TestFormatValuesWithWildcard(t *testing.T) {
	wtr := &bytes.Buffer{}
	mp := &MackerelPlugin{writer: wtr}
	prefix := "foo.#"
	metric := Metrics{Name: "bar", Label: "Get", Diff: true}
	stat := map[string]float64{"foo.1.bar": 1000.0, "foo.2.bar": 2000.0}
	lastStat := map[string]float64{"foo.1.bar": 500.0, ".last_diff.foo.1.bar": 2.0}
	now := time.Unix(1437227240, 0)
	lastTime := now.Add(time.Second * (-60))
	mp.formatValuesWithWildcard(prefix, metric, stat, lastStat, now, lastTime)

	expect := "foo.1.bar	500	1437227240\n"
	got := wtr.String()
	if got != expect {
		t.Errorf("something went wrong: %s", got)
	}
}

func TestFormatValuesWithWildcardAndNoDiff(t *testing.T) {
	wtr := &bytes.Buffer{}
	mp := &MackerelPlugin{writer: wtr}
	prefix := "foo.#"
	metric := Metrics{Name: "bar", Label: "Get", Diff: false}
	stat := map[string]float64{"foo.1.bar": 1000.0}
	lastStat := map[string]float64{"foo.1.bar": 500.0, ".last_diff.foo.1.bar": 2.0}
	now := time.Unix(1437227240, 0)
	lastTime := now.Add(time.Second * (-60))
	mp.formatValuesWithWildcard(prefix, metric, stat, lastStat, now, lastTime)

	expect := "foo.1.bar	1000	1437227240\n"
	got := wtr.String()
	if got != expect {
		t.Errorf("something went wrong: %s", got)
	}
}

func TestFormatValuesWithWildcardAstarisk(t *testing.T) {
	wtr := &bytes.Buffer{}
	mp := &MackerelPlugin{writer: wtr}
	prefix := "foo"
	metric := Metrics{Name: "*", Label: "Get", Diff: true}
	stat := map[string]float64{"foo.1": 1000.0, "foo.2": 2000.0}
	lastStat := map[string]float64{"foo.1": 500.0, ".last_diff.foo.1": 2.0}
	now := time.Unix(1437227240, 0)
	lastTime := now.Add(time.Second * (-60))
	mp.formatValuesWithWildcard(prefix, metric, stat, lastStat, now, lastTime)

	expect := "foo.1	500	1437227240\n"
	got := wtr.String()
	if got != expect {
		t.Errorf("something went wrong: %s", got)
	}
}

type testPWithWildcard struct{}

func (t testPWithWildcard) FetchMetrics() (map[string]float64, error) {
	return map[string]float64{
		"piyo.1.bar": 11,
		"piyo.2.bar": 12,
		"piyo.3.bar": 13,
		"baz":        18.0,
	}, nil
}

func (t testPWithWildcard) GraphDefinition() map[string]Graphs {
	return map[string]Graphs{
		"piyo.#": {
			Metrics: []Metrics{
				{Name: "bar"},
			},
		},
		"fuga": {
			Metrics: []Metrics{
				{Name: "baz"},
			},
		},
	}
}

func (t testPWithWildcard) MetricKeyPrefix() string {
	return "testPWithWildcard"
}

func TestPluginOutputDefinitionsWithPrefixAndWildcard(t *testing.T) {
	mp := NewMackerelPlugin(testPWithWildcard{})
	wtr := &bytes.Buffer{}
	mp.writer = wtr
	os.Setenv("MACKEREL_AGENT_PLUGIN_META", "1")
	defer os.Setenv("MACKEREL_AGENT_PLUGIN_META", "")
	mp.Run()
	expect := `# mackerel-agent-plugin
{"graphs":{"testPWithWildcard.fuga":{"label":"TestPWithWildcard Fuga","unit":"","metrics":[{"name":"baz","label":"Baz","stacked":false}]},"testPWithWildcard.piyo.#":{"label":"TestPWithWildcard Piyo","unit":"","metrics":[{"name":"bar","label":"Bar","stacked":false}]}}}
`
	got := wtr.String()
	if got != expect {
		t.Errorf("result of OutputDefinitions is invalid: %s", got)
	}
}

func TestOutputValuesWithPrefixAndWildcard(t *testing.T) {
	mp := NewMackerelPlugin(testPWithWildcard{})
	wtr := &bytes.Buffer{}
	mp.writer = wtr
	mp.Run()
	epoch := time.Now().Unix()
	expect := fmt.Sprintf("testPWithWildcard.piyo.1.bar\t11\t%[1]d\n"+
		"testPWithWildcard.piyo.2.bar\t12\t%[1]d\n"+
		"testPWithWildcard.piyo.3.bar\t13\t%[1]d\n"+
		"testPWithWildcard.fuga.baz\t18\t%[1]d\n", epoch)
	got := wtr.String()
	if sortLines(got) != sortLines(expect) {
		t.Errorf("result of OutputValues is invalid :%s", got)
	}
}

func TestSetTempfileWithBasename(t *testing.T) {
	var p MackerelPlugin

	expect1 := filepath.Join(os.TempDir(), "my-super-tempfile")
	p.SetTempfileByBasename("my-super-tempfile")
	if p.Tempfile != expect1 {
		t.Errorf("p.SetTempfileByBasename() should set %s, but: %s", expect1, p.Tempfile)
	}

	origDir := os.Getenv("MACKEREL_PLUGIN_WORKDIR")
	os.Setenv("MACKEREL_PLUGIN_WORKDIR", "/tmp/somewhere")
	defer os.Setenv("MACKEREL_PLUGIN_WORKDIR", origDir)

	expect2 := filepath.FromSlash("/tmp/somewhere/my-great-tempfile")
	p.SetTempfileByBasename("my-great-tempfile")
	if p.Tempfile != expect2 {
		t.Errorf("p.SetTempfileByBasename() should set %s, but: %s", expect2, p.Tempfile)
	}
}

func sortLines(s string) string {
	xs := strings.Split(s, "\n")
	sort.Strings(xs)
	return strings.Join(xs, "\n")
}

func TestFetchLastValuesIfNotExist(t *testing.T) {
	p := NewMackerelPlugin(testPHasDiff{})
	p.Tempfile = "state_file_should_not_exist.json"
	_, last, err := p.fetchLastValues(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !last.IsZero() {
		t.Errorf("Timestamp = %v; want 0001-01-01", last)
	}
}

func TestFetchLastValuesIfFileIsBroken(t *testing.T) {
	p := NewMackerelPlugin(testPHasDiff{})
	f := createTempState(t)
	defer f.Close()
	p.Tempfile = f.Name()

	if _, err := f.Write([]byte("{{{-0")); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	_, last, err := p.fetchLastValues(time.Now())
	if err == nil {
		t.Errorf("fetchLastValues should return an error; state is broken")
	}
	if !last.IsZero() {
		t.Errorf("Timestamp = %v; want 0001-01-01", last)
	}
}

func TestFetchLastValuesReadStateSameTime(t *testing.T) {
	p := NewMackerelPlugin(testPHasDiff{})
	f := createTempState(t)
	defer f.Close()
	p.Tempfile = f.Name()
	now := time.Now()
	stats := make(map[string]float64)
	if err := p.saveValues(stats, now); err != nil {
		t.Fatal(err)
	}

	_, last, err := p.fetchLastValues(now)
	if err != errStateRecentlyUpdated {
		t.Errorf("fetchLastValues: %v; want %v", err, errStateRecentlyUpdated)
	}
	if !last.IsZero() {
		t.Errorf("fetchLastValues: last should be zero; but %v", last)
	}
}

func TestSaveStateIfContainsInvalidNumbers(t *testing.T) {
	p := NewMackerelPlugin(testPHasDiff{})
	f := createTempState(t)
	defer f.Close()
	p.Tempfile = f.Name()

	stats := map[string]float64{
		"key1": 3.0,
		"key2": math.Inf(1),
		"key3": math.Inf(-1),
		"key4": math.NaN(),
	}
	const lastTime = 1624848982

	now := time.Unix(lastTime, 0)
	if err := p.saveValues(stats, now); err != nil {
		t.Errorf("saveValues: %v", err)
	}
	stats, _, err := p.fetchLastValues(now.Add(time.Second))
	if err != nil {
		t.Fatal("fetchLastValues:", err)
	}
	want := map[string]float64{
		"_lastTime": lastTime,
		"key1":      3.0,
	}
	if !reflect.DeepEqual(stats, want) {
		t.Errorf("saveValues stores only valid numbers: got %v; want %v", stats, want)
	}
}

func createTempState(t testing.TB) *os.File {
	t.Helper()
	f, err := ioutil.TempFile("", "mackerel-plugin.")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Remove(f.Name()); err != nil {
			t.Fatal(err)
		}
	})
	return f
}
