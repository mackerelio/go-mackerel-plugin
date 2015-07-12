package mackerelplugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"time"
)

type Metrics struct {
	Name    string  `json:"name"`
	Label   string  `json:"label"`
	Diff    bool    `json:"diff"`
	Counter bool    `json:"counter"`
	Type    string  `json:"type"`
	Stacked bool    `json:"stacked"`
	Scale   float64 `json:"scale"`
}

type Graphs struct {
	Label   string    `json:"label"`
	Unit    string    `json:"unit"`
	Metrics []Metrics `json:"metrics"`
}

type Plugin interface {
	FetchMetrics() (map[string]float64, error)
	GraphDefinition() map[string]Graphs
}

type MackerelPlugin struct {
	Plugin
	Tempfile string
}

func NewMackerelPlugin(plugin Plugin) MackerelPlugin {
	mp := MackerelPlugin{plugin, "/tmp/mackerel-plugin-default"}
	return mp
}

func (h *MackerelPlugin) printValue(w io.Writer, key string, value float64, now time.Time) {
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

func (h *MackerelPlugin) fetchLastValues() (map[string]float64, time.Time, error) {
	lastTime := time.Now()

	f, err := os.Open(h.Tempfilename())
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

func (h *MackerelPlugin) saveValues(values map[string]float64, now time.Time) error {
	f, err := os.Create(h.Tempfilename())
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

func (h *MackerelPlugin) calcDiff(value float64, now time.Time, lastValue float64, lastTime time.Time, valType string) (float64, error) {
	diffTime := now.Unix() - lastTime.Unix()
	if diffTime > 600 {
		return 0, errors.New("Too long duration")
	}

	diff := (value - lastValue) * 60 / float64(diffTime)

	// Negative value means counter reset.
	switch valType {
	case "uint32":
		if diff < 0 {
			diff = diff + math.MaxUint32
		}
	}

	return diff, nil
}

func (h *MackerelPlugin) Tempfilename() string {
	return h.Tempfile
}

func (h *MackerelPlugin) OutputValues() {
	now := time.Now()
	stat, err := h.FetchMetrics()
	if err != nil {
		log.Fatalln("OutputValues: ", err)
	}

	lastStat, lastTime, err := h.fetchLastValues()
	if err != nil {
		log.Println("fetchLastValues (ignore):", err)
	}

	err = h.saveValues(stat, now)
	if err != nil {
		log.Fatalf("saveValues: ", err)
	}

	for key, graph := range h.GraphDefinition() {
		for _, metric := range graph.Metrics {
			value := stat[metric.Name]

			if metric.Diff {
				_, ok := lastStat[metric.Name]
				if ok {
					value, err = h.calcDiff(value, now, lastStat[metric.Name], lastTime, metric.Type)
					if err != nil {
						log.Println("OutputValues: ", err)
					}
				} else {
					log.Printf("%s is not exist at last fetch\n", metric.Name)
				}
			}

			if metric.Scale != 0 {
				value *= metric.Scale
			}

			switch metric.Type {
			case "uint32", "uint64":
				if value > 0.0 {
					h.printValue(os.Stdout, key+"."+metric.Name, value, now)
				}
			default:
				h.printValue(os.Stdout, key+"."+metric.Name, value, now)
			}
		}
	}
}

type GraphDef struct {
	Graphs map[string]Graphs `json:"graphs"`
}

func (h *MackerelPlugin) OutputDefinitions() {
	fmt.Println("# mackerel-agent-plugin")
	var graphs GraphDef
	graphs.Graphs = h.GraphDefinition()

	b, err := json.Marshal(graphs)
	if err != nil {
		log.Fatalln("OutputDefinitions: ", err)
	}
	fmt.Println(string(b))
}
