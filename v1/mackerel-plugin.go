package v1

import (
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/mackerelio/golib/pluginutil"
)

// Metric units
const (
	UnitFloat          = "float"
	UnitInteger        = "integer"
	UnitPercentage     = "percentage"
	UnitSeconds        = "seconds"
	UnitMilliseconds   = "milliseconds"
	UnitBytes          = "bytes"
	UnitBytesPerSecond = "bytes/sec"
	UnitBitsPerSecond  = "bits/sec"
	UnitIOPS           = "iops"
)

// Metrics represents definition of a metric
type Metrics struct {
	Name    string  `json:"name"`
	Label   string  `json:"label"`
	Diff    bool    `json:"-"`
	Stacked bool    `json:"stacked"`
	Scale   float64 `json:"-"`
}

// Graphs represents definition of a graph
type Graphs struct {
	Label   string    `json:"label"`
	Unit    string    `json:"unit"`
	Metrics []Metrics `json:"metrics"`
}

// Plugin is old interface of mackerel-plugin
type Plugin interface {
	FetchMetrics() (map[string]float64, error)
	GraphDefinition() map[string]Graphs
}

// PluginWithPrefix is recommended interface
type PluginWithPrefix interface {
	Plugin
	MetricKeyPrefix() string
}

// MackerelPlugin is for mackerel-agent-plugins
type MackerelPlugin struct {
	Plugin
	Tempfile string
	diff     *bool
	writer   io.Writer
}

// NewMackerelPlugin returns new MackrelPlugin
func NewMackerelPlugin(plugin Plugin) *MackerelPlugin {
	return &MackerelPlugin{Plugin: plugin}
}

func (mp *MackerelPlugin) getWriter() io.Writer {
	if mp.writer == nil {
		mp.writer = os.Stdout
	}
	return mp.writer
}

func (mp *MackerelPlugin) hasDiff() bool {
	if mp.diff == nil {
		diff := false
		mp.diff = &diff
	DiffCheck:
		for _, graph := range mp.GraphDefinition() {
			for _, metric := range graph.Metrics {
				if metric.Diff {
					*mp.diff = true
					break DiffCheck
				}
			}
		}
	}
	return *mp.diff
}

func (mp *MackerelPlugin) printValue(w io.Writer, key string, value float64, now time.Time) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		log.Printf("Invalid value: key = %s, value = %f\n", key, value)
		return
	}

	if value == float64(int(value)) {
		fmt.Fprintf(w, "%s\t%d\t%d\n", key, int(value), now.Unix())
	} else {
		fmt.Fprintf(w, "%s\t%f\t%d\n", key, value, now.Unix())
	}
}

var errStateRecentlyUpdated = errors.New("state was recently updated")

const oldEnoughDuration = time.Second

func (mp *MackerelPlugin) fetchLastValues(now time.Time) (map[string]float64, time.Time, error) {
	if !mp.hasDiff() {
		return nil, time.Time{}, nil
	}

	f, err := os.Open(mp.tempfilename())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, time.Time{}, nil
		}
		return nil, time.Time{}, err
	}
	defer f.Close()

	stat := make(map[string]float64)
	decoder := json.NewDecoder(f)
	err = decoder.Decode(&stat)
	if err != nil {
		return stat, time.Time{}, err
	}
	lastTime := time.Unix(int64(stat["_lastTime"]), 0)
	if now.Sub(lastTime) < oldEnoughDuration {
		return stat, time.Time{}, errStateRecentlyUpdated
	}
	return stat, lastTime, nil
}

func (mp *MackerelPlugin) saveValues(values map[string]float64, now time.Time) error {
	if !mp.hasDiff() {
		return nil
	}
	f, err := os.Create(mp.tempfilename())
	if err != nil {
		return err
	}
	defer f.Close()

	// Since Go 1.15 strconv.ParseFloat returns +Inf if it couldn't parse a string.
	// But JSON does not accept invalid numbers, such as +Inf, -Inf or NaN.
	// We perhaps have some plugins that is affected above change,
	// so saveState should clear invalid numbers in the values before saving it.
	for k, v := range values {
		if math.IsInf(v, 0) || math.IsNaN(v) {
			delete(values, k)
		}
	}

	values["_lastTime"] = float64(now.Unix())
	encoder := json.NewEncoder(f)
	err = encoder.Encode(values)
	if err != nil {
		return err
	}

	return nil
}

func (mp *MackerelPlugin) calcDiff(value float64, now time.Time, lastValue float64, lastTime time.Time) (float64, error) {
	diffTime := now.Unix() - lastTime.Unix()
	if diffTime > 600 {
		return 0, errors.New("too long duration")
	}

	diff := (value - lastValue) * 60 / float64(diffTime)
	if diff < 0 {
		return 0, errors.New("counter seems to be reset")
	}
	return diff, nil
}

func (mp *MackerelPlugin) tempfilename() string {
	if mp.Tempfile == "" {
		mp.Tempfile = mp.generateTempfilePath(os.Args)
	}
	return mp.Tempfile
}

var tempfileSanitizeReg = regexp.MustCompile(`[^-_.A-Za-z0-9]`)

// SetTempfileByBasename sets Tempfile under proper directory with specified basename.
func (mp *MackerelPlugin) SetTempfileByBasename(base string) {
	mp.Tempfile = filepath.Join(pluginutil.PluginWorkDir(), base)
}

func (mp *MackerelPlugin) generateTempfilePath(args []string) string {
	commandPath := args[0]
	var prefix string
	if p, ok := mp.Plugin.(PluginWithPrefix); ok {
		prefix = p.MetricKeyPrefix()
	} else {
		name := filepath.Base(commandPath)
		prefix = strings.TrimPrefix(tempfileSanitizeReg.ReplaceAllString(name, "_"), "mackerel-plugin-")
	}
	filename := fmt.Sprintf(
		"mackerel-plugin-%s-%x",
		prefix,
		// When command-line options are different, mostly different metrics.
		// e.g. `-host` and `-port` options for mackerel-plugin-mysql
		sha1.Sum([]byte(strings.Join(args[1:], " "))),
	)
	return filepath.Join(pluginutil.PluginWorkDir(), filename)
}

// OutputValues output the metrics
func (mp *MackerelPlugin) OutputValues() {
	now := time.Now()
	stat, err := mp.FetchMetrics()
	if err != nil {
		log.Fatalln("OutputValues: ", err)
	}

	lastStat, lastTime, err := mp.fetchLastValues(now)
	if err != nil {
		if err == errStateRecentlyUpdated {
			log.Println("OutputValues:", err)
			return
		}
		log.Println("fetchLastValues (ignore):", err)
	}

	for key, graph := range mp.GraphDefinition() {
		for _, metric := range graph.Metrics {
			if strings.ContainsAny(key+metric.Name, "*#") {
				mp.formatValuesWithWildcard(key, metric, stat, lastStat, now, lastTime)
			} else {
				mp.formatValues(key, metric, stat, lastStat, now, lastTime)
			}
		}
	}

	err = mp.saveValues(stat, now)
	if err != nil {
		log.Fatalf("saveValues: %s", err)
	}
}

func (mp *MackerelPlugin) formatValuesWithWildcard(prefix string, metric Metrics, stat map[string]float64, lastStat map[string]float64, now time.Time, lastTime time.Time) {
	regexpStr := `\A` + prefix + "." + metric.Name
	regexpStr = strings.Replace(regexpStr, ".", `\.`, -1)
	regexpStr = strings.Replace(regexpStr, "*", `[-a-zA-Z0-9_]+`, -1)
	regexpStr = strings.Replace(regexpStr, "#", `[-a-zA-Z0-9_]+`, -1)
	re, err := regexp.Compile(regexpStr)
	if err != nil {
		log.Fatalln("Failed to compile regexp: ", err)
	}
	for k := range stat {
		if re.MatchString(k) {
			metricEach := metric
			metricEach.Name = k
			mp.formatValues("", metricEach, stat, lastStat, now, lastTime)
		}
	}
}

func (mp *MackerelPlugin) formatValues(prefix string, metric Metrics, stat map[string]float64, lastStat map[string]float64, now time.Time, lastTime time.Time) {
	name := metric.Name
	if prefix != "" {
		name = prefix + "." + name
	}
	value, ok := stat[name]
	if !ok {
		return
	}
	if metric.Diff {
		lastValue, ok := lastStat[name]
		if ok {
			var err error
			value, err = mp.calcDiff(value, now, lastValue, lastTime)
			if err != nil {
				log.Println("OutputValues: ", err)
			}
		} else {
			log.Printf("%s does not exist at last fetch\n", metric.Name)
			return
		}
	}

	if metric.Scale != 0 {
		value *= metric.Scale
	}

	metricNames := []string{}
	if p, ok := mp.Plugin.(PluginWithPrefix); ok {
		metricNames = append(metricNames, p.MetricKeyPrefix())
	}
	if prefix != "" {
		metricNames = append(metricNames, prefix)
	}
	metricNames = append(metricNames, metric.Name)
	mp.printValue(mp.getWriter(), strings.Join(metricNames, "."), value, now)
}

// GraphDef is graph definitions
type GraphDef struct {
	Graphs map[string]Graphs `json:"graphs"`
}

func title(s string) string {
	r := strings.NewReplacer(".", " ", "_", " ", "*", "", "#", "")
	return strings.TrimSpace(cases.Title(language.Und, cases.NoLower).String(r.Replace(s)))
}

// OutputDefinitions outputs graph definitions
func (mp *MackerelPlugin) OutputDefinitions() {
	fmt.Fprintln(mp.getWriter(), "# mackerel-agent-plugin")
	graphs := make(map[string]Graphs)
	for key, graph := range mp.GraphDefinition() {
		g := graph
		k := key
		if p, ok := mp.Plugin.(PluginWithPrefix); ok {
			prefix := p.MetricKeyPrefix()
			if k == "" {
				k = prefix
			} else {
				k = prefix + "." + k
			}
		}
		if g.Label == "" {
			g.Label = title(k)
		}
		metrics := []Metrics{}
		for _, v := range g.Metrics {
			if v.Label == "" {
				v.Label = title(v.Name)
			}
			metrics = append(metrics, v)
		}
		g.Metrics = metrics
		graphs[k] = g
	}
	var graphdef GraphDef
	graphdef.Graphs = graphs
	b, err := json.Marshal(graphdef)
	if err != nil {
		log.Fatalln("OutputDefinitions: ", err)
	}
	fmt.Fprintln(mp.getWriter(), string(b))
}

// Run the plugin
func (mp *MackerelPlugin) Run() {
	if os.Getenv("MACKEREL_AGENT_PLUGIN_META") != "" {
		mp.OutputDefinitions()
	} else {
		mp.OutputValues()
	}
}
