package mackerelpluginhelper

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

type FetchFunc func() (map[string]float64, error)

type Metrics struct {
	Key   string
	Label string
	Diff  bool
}

type Graphs struct {
	Label   string
	Key     string
	Unit    string
	Metrics []Metrics
	Ms      []Metrics
}

type MackerelPluginHelper struct {
	Tempfile   string
	Fetch_stat FetchFunc
	Graphs     []Graphs
}

func (h *MackerelPluginHelper) print_value(w io.Writer, key string, value float64, now time.Time) {
	if value == float64(int(value)) {
		fmt.Fprintf(w, "%s\t%d\t%d\n", key, int(value), now.Unix())
	} else {
		fmt.Fprintf(w, "%s\t%f\t%d\n", key, value, now.Unix())
	}
}

func (h *MackerelPluginHelper) fetch_last_values() (map[string]float64, time.Time, error) {
	last_time := time.Now()

	f, err := os.Open(h.Tempfile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, last_time, nil
		}
		return nil, last_time, err
	}
	defer f.Close()

	r := bufio.NewReader(f)
	line, isPrefix, err := r.ReadLine()
	stat := make(map[string]float64)
	for err == nil && !isPrefix {
		s := string(line)
		res := strings.Split(s, "\t")
		if len(res) != 3 {
			break
		}
		stat[res[0]], err = strconv.ParseFloat(res[1], 64)
		if err != nil {
			fmt.Println("fetch_last_values: ", err)
		}
		timestamp, err := strconv.Atoi(res[2])
		if err != nil {
			fmt.Println("fetch_last_values: ", err)
		}
		last_time = time.Unix(int64(timestamp), 0)
		if err != nil {
			fmt.Println("fetch_last_values: ", err)
		}
		line, isPrefix, err = r.ReadLine()
	}
	if isPrefix {
		return nil, last_time, errors.New("buffer size too small")
	}
	if err != nil {
		return stat, last_time, err
	}
	return stat, last_time, nil
}

func (h *MackerelPluginHelper) save_values(values map[string]float64, now time.Time) error {
	f, err := os.Create(h.Tempfile)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)

	for key, value := range values {
		h.print_value(w, key, value, now)
		w.Flush()
	}

	return nil
}

func (h *MackerelPluginHelper) calc_diff(value float64, now time.Time, last_value float64, last_time time.Time) (float64, error) {
	diff_time := now.Unix() - last_time.Unix()
	if diff_time > 600 {
		return 0, errors.New("Too long duration")
	}

	diff := (value - last_value) * 60 / float64(diff_time)
	return diff, nil
}

func (h *MackerelPluginHelper) Output_values() {
	now := time.Now()
	stat, err := h.Fetch_stat()
	if err != nil {
		fmt.Println(err)
		return
	}

	last_stat, last_time, err := h.fetch_last_values()
	if err != nil {
		fmt.Println("fetch_last_values (ignore):", err)
	}

	err = h.save_values(stat, now)
	if err != nil {
		fmt.Println("save_values: ", err)
		return
	}

	for _, graph := range h.Graphs {
		for _, metric := range graph.Metrics {
			if metric.Diff {
				_, ok := last_stat[metric.Key]
				if ok {
					diff, err := h.calc_diff(stat[metric.Key], now, last_stat[metric.Key], last_time)
					if err != nil {
						fmt.Println(err)
					} else {
						h.print_value(os.Stdout, graph.Key+"."+metric.Key, diff, now)
					}
				} else {
					fmt.Printf("%s is not exist at last fetch\n", metric.Key)
				}
			} else {
				h.print_value(os.Stdout, graph.Key+"."+metric.Key, stat[metric.Key], now)
			}
		}
	}
}

func (h *MackerelPluginHelper) Output_definitions() {
	fmt.Print("# mackerel-agent-plugin\n{\n")

	fmt.Print("  \"graphs\": {\n")
	for i, graph := range h.Graphs {
		fmt.Printf("    \"%s\": {\n", graph.Key)
		fmt.Printf("      \"label\": \"%s\",\n", graph.Label)
		fmt.Printf("      \"unit\": \"%s\",\n", graph.Unit)
		fmt.Print("      \"metrics\": [\n")
		for i, metric := range graph.Metrics {
			fmt.Printf("        {\"name\": \"%s\", ", metric.Key)
			if i+1 < len(graph.Metrics) {
				fmt.Printf("\"label\": \"%s\"},\n", metric.Label)
			} else {
				fmt.Printf("\"label\": \"%s\"}\n", metric.Label)
			}
		}
		fmt.Print("      ]\n")
		if i+1 < len(h.Graphs) {
			fmt.Print("    },\n")
		} else {
			fmt.Print("    }\n")
		}
	}
	fmt.Print("  }\n")
	fmt.Print("}\n")
}
