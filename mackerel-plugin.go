package mackerelplugin

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
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

func (mp *MackerelPlugin) fetchLastValues() (map[string]float64, time.Time, error) {
	if !mp.hasDiff() {
		return nil, time.Unix(0, 0), nil
	}
	lastTime := time.Now()

	f, err := os.Open(mp.tempfilename())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, lastTime, nil
		}
		return nil, lastTime, err
	}
	defer f.Close()

	stat := make(map[string]float64)
	decoder := json.NewDecoder(f)
	err = decoder.Decode(&stat)
	lastTime = time.Unix(int64(stat["_lastTime"]), 0)
	if err != nil {
		return stat, lastTime, err
	}
	return stat, lastTime, nil
}

func (mp *MackerelPlugin) saveValues(values map[string]float64, now time.Time) error {
	f, err := os.Create(mp.tempfilename())
	if err != nil {
		return err
	}
	defer f.Close()

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
		return 0, fmt.Errorf("Too long duration")
	}

	diff := (value - lastValue) * 60 / float64(diffTime)
	if diff < 0 {
		return 0, fmt.Errorf("Counter seems to be reset.")
	}
	return diff, nil
}

func (mp *MackerelPlugin) tempfilename() string {
	if mp.Tempfile == "" {
		mp.Tempfile = mp.generateTempfilePath(os.Args[0])
	}
	return mp.Tempfile
}

var tempfileSanitizeReg = regexp.MustCompile(`[^-_.A-Za-z0-9]`)

func (mp *MackerelPlugin) generateTempfilePath(path string) string {
	var prefix string
	if p, ok := mp.Plugin.(PluginWithPrefix); ok {
		prefix = p.MetricKeyPrefix()
	} else {
		name := filepath.Base(path)
		prefix = strings.TrimPrefix(tempfileSanitizeReg.ReplaceAllString(name, "_"), "mackerel-plugin-")
	}
	filename := fmt.Sprintf("mackerel-plugin-%s", prefix)
	dir := os.Getenv("MACKEREL_PLUGIN_WORKDIR")
	if dir == "" {
		dir = os.TempDir()
	}
	return filepath.Join(dir, filename)
}

// OutputValues output the metrics
func (mp *MackerelPlugin) OutputValues() {
	now := time.Now()
	stat, err := mp.FetchMetrics()
	if err != nil {
		log.Fatalln("OutputValues: ", err)
	}

	lastStat, lastTime, err := mp.fetchLastValues()
	if err != nil {
		log.Println("fetchLastValues (ignore):", err)
	}

	for key, graph := range mp.GraphDefinition() {
		for _, metric := range graph.Metrics {
			mp.formatValues(key, metric, stat, lastStat, now, lastTime)
		}
	}

	err = mp.saveValues(stat, now)
	if err != nil {
		log.Fatalf("saveValues: %s", err)
	}
}

func (mp *MackerelPlugin) formatValues(prefix string, metric Metrics, stat map[string]float64, lastStat map[string]float64, now time.Time, lastTime time.Time) {
	name := metric.Name
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
		}
	}

	if metric.Scale != 0 {
		value *= metric.Scale
	}
	mp.printValue(mp.getWriter(), prefix+"."+metric.Name, value, now)
}

// GraphDef is graph definitions
type GraphDef struct {
	Graphs map[string]Graphs `json:"graphs"`
}

// OutputDefinitions outputs graph definitions
func (mp *MackerelPlugin) OutputDefinitions() {
	fmt.Println("# mackerel-agent-plugin")
	var graphs GraphDef
	graphs.Graphs = mp.GraphDefinition()

	b, err := json.Marshal(graphs)
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
