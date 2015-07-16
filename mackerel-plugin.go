package mackerelplugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"strconv"
	"time"
)

type Metrics struct {
	Name    string  `json:"name"`
	Label   string  `json:"label"`
	Diff    bool    `json:"diff"`
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
	FetchMetrics() (map[string]interface{}, error)
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

func (h *MackerelPlugin) printValue(w io.Writer, key string, value interface{}, now time.Time) {
	switch value.(type) {
	case uint32:
		fmt.Fprintf(w, "%s\t%d\t%d\n", key, value.(uint32), now.Unix())
	case uint64:
		fmt.Fprintf(w, "%s\t%d\t%d\n", key, value.(uint64), now.Unix())
	case float64:
		if math.IsNaN(value.(float64)) || math.IsInf(value.(float64), 0) {
			log.Printf("Invalid value: key = %s, value = %f\n", key, value)
		} else {
			fmt.Fprintf(w, "%s\t%f\t%d\n", key, value.(float64), now.Unix())
		}
	}
}

func (h *MackerelPlugin) fetchLastValues() (map[string]interface{}, time.Time, error) {
	lastTime := time.Now()

	f, err := os.Open(h.Tempfilename())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, lastTime, nil
		}
		return nil, lastTime, err
	}
	defer f.Close()

	stat := make(map[string]interface{})
	decoder := json.NewDecoder(f)
	err = decoder.Decode(&stat)
	lastTime = time.Unix(stat["_lastTime"].(int64), 0)
	if err != nil {
		return stat, lastTime, err
	}
	return stat, lastTime, nil
}

func (h *MackerelPlugin) saveValues(values map[string]interface{}, now time.Time) error {
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

func (h *MackerelPlugin) calcDiff(value float64, now time.Time, lastValue float64, lastTime time.Time) (float64, error) {
	diffTime := now.Unix() - lastTime.Unix()
	if diffTime > 600 {
		return 0, errors.New("Too long duration")
	}

	diff := (value - lastValue) * 60 / float64(diffTime)

	return diff, nil
}

func (h *MackerelPlugin) calcDiffUint32(value uint32, now time.Time, lastValue uint32, lastTime time.Time, lastDiff float64) (float64, error) {
	diffTime := now.Unix() - lastTime.Unix()
	if diffTime > 600 {
		return 0, errors.New("Too long duration")
	}

	diff := float64((value-lastValue)*60) / float64(diffTime)

	if lastValue < value || diff < lastDiff*10 {
		return diff, nil
	}
	return 0.0, errors.New("Counter seems to be reseted.")

}

func (h *MackerelPlugin) calcDiffUint64(value uint64, now time.Time, lastValue uint64, lastTime time.Time, lastDiff float64) (float64, error) {
	diffTime := now.Unix() - lastTime.Unix()
	if diffTime > 600 {
		return 0, errors.New("Too long duration")
	}

	diff := float64((value-lastValue)*60) / float64(diffTime)

	if lastValue < value || diff < lastDiff*10 {
		return diff, nil
	}
	return 0.0, errors.New("Counter seems to be reseted.")
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

	for key, graph := range h.GraphDefinition() {
		for _, metric := range graph.Metrics {
			var value interface{}
			value = stat[metric.Name]
			switch value.(type) {
			case string:
				switch metric.Type {
				case "uint32":
					value, _ = strconv.ParseUint(value.(string), 10, 32)
				case "uint64":
					value, _ = strconv.ParseUint(value.(string), 10, 64)
				default:
					value, _ = strconv.ParseFloat(value.(string), 64)
				}
			default:
				continue
			}

			if metric.Diff {
				_, ok := lastStat[metric.Name]
				if ok {
					lastDiff := lastStat[".last_diff."+metric.Name].(float64)
					switch metric.Type {
					case "uint32":
						value, err = h.calcDiffUint32(value.(uint32), now, lastStat[metric.Name].(uint32), lastTime, lastDiff)
						stat[".last_diff."+metric.Name] = value
					case "uint64":
						value, err = h.calcDiffUint64(value.(uint64), now, lastStat[metric.Name].(uint64), lastTime, lastDiff)
						stat[".last_diff."+metric.Name] = value
					default:
						value, err = h.calcDiff(value.(float64), now, lastStat[metric.Name].(float64), lastTime)
					}
					if err != nil {
						log.Println("OutputValues: ", err)
					}
				} else {
					log.Printf("%s is not exist at last fetch\n", metric.Name)
				}
			}

			if metric.Scale != 0 {
				switch metric.Type {
				case "uint32":
					value = value.(uint32) * uint32(metric.Scale)
				case "uint64":
					value = value.(uint64) * uint64(metric.Scale)
				default:
					value = value.(float64) * metric.Scale
				}
			}

			h.printValue(os.Stdout, key+"."+metric.Name, value, now)
		}
	}

	err = h.saveValues(stat, now)
	if err != nil {
		log.Fatalf("saveValues: ", err)
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
