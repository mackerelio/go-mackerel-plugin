package mackerelpluginhelper

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

type Metrics struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Diff  bool   `json:"diff"`
}

type Graphs struct {
	Label   string    `json:"label"`
	Unit    string    `json:"unit"`
	Metrics []Metrics `json:"metrics"`
}

type MackerelPlugin interface {
	FetchData() (map[string]float64, error)
	GetGraphDefinition() map[string]Graphs
	GetTempfilename() string
}

type MackerelPluginHelper struct {
	MackerelPlugin
}

func (h *MackerelPluginHelper) printValue(w io.Writer, key string, value float64, now time.Time) {
	if value == float64(int(value)) {
		fmt.Fprintf(w, "%s\t%d\t%d\n", key, int(value), now.Unix())
	} else {
		fmt.Fprintf(w, "%s\t%f\t%d\n", key, value, now.Unix())
	}
}

func (h *MackerelPluginHelper) fetchLastValues() (map[string]float64, time.Time, error) {
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
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		s := string(line)
		res := strings.Split(s, "\t")
		if len(res) != 3 {
			break
		}
		stat[res[0]], err = strconv.ParseFloat(res[1], 64)
		if err != nil {
			fmt.Fprintln(os.Stderr, "fetchLastValues: ", err)
		}
		timestamp, err := strconv.Atoi(res[2])
		if err != nil {
			fmt.Fprintln(os.Stderr, "fetchLastValues: ", err)
		}
		lastTime = time.Unix(int64(timestamp), 0)
		if err != nil {
			fmt.Fprintln(os.Stderr, "fetchLastValues: ", err)
		}
	}
	if err := scanner.Err(); err != nil {
		return stat, lastTime, err
	}
	return stat, lastTime, nil
}

func (h *MackerelPluginHelper) saveValues(values map[string]float64, now time.Time) error {
	f, err := os.Create(h.GetTempfilename())
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	for key, value := range values {
		h.printValue(w, key, value, now)
	}
	w.Flush()

	return nil
}

func (h *MackerelPluginHelper) calcDiff(value float64, now time.Time, lastValue float64, lastTime time.Time) (float64, error) {
	diffTime := now.Unix() - lastTime.Unix()
	if diffTime > 600 {
		return 0, errors.New("Too long duration")
	}

	diff := (value - lastValue) * 60 / float64(diffTime)
	return diff, nil
}

func (h *MackerelPluginHelper) OutputValues() {
	now := time.Now()
	stat, err := h.FetchData()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}

	lastStat, lastTime, err := h.fetchLastValues()
	if err != nil {
		fmt.Fprintln(os.Stderr, "fetchLastValues (ignore):", err)
	}

	err = h.saveValues(stat, now)
	if err != nil {
		fmt.Fprintln(os.Stderr, "saveValues: ", err)
		return
	}

	for key, graph := range h.GetGraphDefinition() {
		for _, metric := range graph.Metrics {
			if metric.Diff {
				_, ok := lastStat[metric.Key]
				if ok {
					diff, err := h.calcDiff(stat[metric.Key], now, lastStat[metric.Key], lastTime)
					if err != nil {
						fmt.Println(err)
					} else {
						h.printValue(os.Stdout, key+"."+metric.Key, diff, now)
					}
				} else {
					fmt.Fprintf(os.Stderr, "%s is not exist at last fetch\n", metric.Key)
				}
			} else {
				h.printValue(os.Stdout, key+"."+metric.Key, stat[metric.Key], now)
			}
		}
	}
}

func (h *MackerelPluginHelper) OutputDefinitions() {
	fmt.Println("# mackerel-agent-plugin")
	b, err := json.Marshal(h.GetGraphDefinition())
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(string(b))
}
