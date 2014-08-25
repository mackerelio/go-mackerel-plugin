package mackerelplugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"time"
)

type Metrics struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Diff    bool   `json:"diff"`
	Stacked bool   `json:"stacked"`
}

type Graphs struct {
	Label   string    `json:"label"`
	Unit    string    `json:"unit"`
	Metrics []Metrics `json:"metrics"`
}

type Plugin interface {
	FetchData() (map[string]float64, error)
	GetGraphDefinition() map[string]Graphs
	GetTempfilename() string
}

type MackerelPlugin struct {
	Plugin
}

func (h *MackerelPlugin) printValue(w io.Writer, key string, value float64, now time.Time) {
	if value == float64(int(value)) {
		fmt.Fprintf(w, "%s\t%d\t%d\n", key, int(value), now.Unix())
	} else {
		fmt.Fprintf(w, "%s\t%f\t%d\n", key, value, now.Unix())
	}
}

func (h *MackerelPlugin) fetchLastValues() (map[string]float64, time.Time, error) {
	lastTime := time.Now()

	f, err := os.Open(h.GetTempfilename())
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
	f, err := os.Create(h.GetTempfilename())
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

func (h *MackerelPlugin) OutputValues() {
	now := time.Now()
	stat, err := h.FetchData()
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

	for key, graph := range h.GetGraphDefinition() {
		for _, metric := range graph.Metrics {
			if metric.Diff {
				_, ok := lastStat[metric.Key]
				if ok {
					diff, err := h.calcDiff(stat[metric.Key], now, lastStat[metric.Key], lastTime)
					if err != nil {
						log.Println("OutputValues: ", err)
					} else {
						h.printValue(os.Stdout, key+"."+metric.Key, diff, now)
					}
				} else {
					log.Printf("%s is not exist at last fetch\n", metric.Key)
				}
			} else {
				h.printValue(os.Stdout, key+"."+metric.Key, stat[metric.Key], now)
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
	graphs.Graphs = h.GetGraphDefinition()

	b, err := json.Marshal(graphs)
	if err != nil {
		log.Fatalln("OutputDefinitions: ", err)
	}
	fmt.Println(string(b))
}
