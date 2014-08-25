package main

import (
	"bufio"
	"flag"
	"fmt"
	mph "github.com/mackerelio/go-mackerel-plugin-helper"
	"net"
	"os"
	"strconv"
	"strings"
)

var graphdef map[string](mph.Graphs) = map[string](mph.Graphs){
	"memcached.connections": mph.Graphs{
		Label: "Memcached Connections",
		Unit:  "integer",
		Metrics: [](mph.Metrics){
			mph.Metrics{Key: "curr_connections", Label: "Connections", Diff: false},
		},
	},
	"memcached.cmd": mph.Graphs{
		Label: "Memcached Command",
		Unit:  "integer",
		Metrics: [](mph.Metrics){
			mph.Metrics{Key: "cmd_get", Label: "Get", Diff: true},
			mph.Metrics{Key: "cmd_set", Label: "Set", Diff: true},
			mph.Metrics{Key: "cmd_flush", Label: "Flush", Diff: true},
			mph.Metrics{Key: "cmd_touch", Label: "Touch", Diff: true},
		},
	},
	"memcached.hitmiss": mph.Graphs{
		Label: "Memcached Hits/Misses",
		Unit:  "integer",
		Metrics: [](mph.Metrics){
			mph.Metrics{Key: "get_hits", Label: "Get Hits", Diff: true},
			mph.Metrics{Key: "get_misses", Label: "Get Misses", Diff: true},
			mph.Metrics{Key: "delete_hits", Label: "Delete Hits", Diff: true},
			mph.Metrics{Key: "delete_misses", Label: "Delete Misses", Diff: true},
			mph.Metrics{Key: "incr_hits", Label: "Incr Hits", Diff: true},
			mph.Metrics{Key: "incr_misses", Label: "Incr Misses", Diff: true},
			mph.Metrics{Key: "cas_hits", Label: "Cas Hits", Diff: true},
			mph.Metrics{Key: "cas_misses", Label: "Cas Misses", Diff: true},
			mph.Metrics{Key: "touch_hits", Label: "Touch Hits", Diff: true},
			mph.Metrics{Key: "touch_misses", Label: "Touch Misses", Diff: true},
		},
	},
	"memcached.evictions": mph.Graphs{
		Label: "Memcached Evictions",
		Unit:  "integer",
		Metrics: [](mph.Metrics){
			mph.Metrics{Key: "evictions", Label: "Evictions", Diff: true},
		},
	},
	"memcached.unfetched": mph.Graphs{
		Label: "Memcached Unfetched",
		Unit:  "integer",
		Metrics: [](mph.Metrics){
			mph.Metrics{Key: "expired_unfetched", Label: "Expired unfetched", Diff: true},
			mph.Metrics{Key: "evicted_unfetched", Label: "Evicted unfetched", Diff: true},
		},
	},
	"memcached.rusage": mph.Graphs{
		Label: "Memcached Resouce Usage",
		Unit:  "float",
		Metrics: [](mph.Metrics){
			mph.Metrics{Key: "rusage_user", Label: "User", Diff: true},
			mph.Metrics{Key: "rusage_system", Label: "System", Diff: true},
		},
	},
	"memcached.bytes": mph.Graphs{
		Label: "Memcached Traffics",
		Unit:  "bytes",
		Metrics: [](mph.Metrics){
			mph.Metrics{Key: "bytes_read", Label: "Read", Diff: true},
			mph.Metrics{Key: "bytes_written", Label: "Write", Diff: true},
		},
	},
}

type MemcachedPlugin struct {
	Target   string
	Tempfile string
}

func (m MemcachedPlugin) FetchData() (map[string]float64, error) {
	conn, err := net.Dial("tcp", m.Target)
	if err != nil {
		return nil, err
	}
	fmt.Fprintln(conn, "stats")
	scanner := bufio.NewScanner(conn)
	stat := make(map[string]float64)

	for scanner.Scan() {
		line := scanner.Text()
		s := string(line)
		if s == "END" {
			return stat, nil
		}

		res := strings.Split(s, " ")
		if res[0] == "STAT" {
			stat[res[1]], err = strconv.ParseFloat(res[2], 64)
			if err != nil {
				fmt.Fprintln(os.Stderr, "readStat:", err)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return stat, err
	}
	return nil, nil
}

func (m MemcachedPlugin) GetGraphDefinition() map[string](mph.Graphs) {
	return graphdef
}

func (m MemcachedPlugin) GetTempfilename() string {
	return m.Tempfile
}

func main() {
	optHost := flag.String("host", "localhost", "Hostname")
	optPort := flag.String("port", "11211", "Port")
	optTempfile := flag.String("tempfile", "", "Temp file name")
	flag.Parse()

	var memcached MemcachedPlugin

	memcached.Target = fmt.Sprintf("%s:%s", *optHost, *optPort)
	if *optTempfile != "" {
		memcached.Tempfile = *optTempfile
	} else {
		memcached.Tempfile = fmt.Sprintf("/tmp/mackerel-plugin-memcached-%s-%s", *optHost, *optPort)
	}

	helper := mph.MackerelPluginHelper{memcached}

	if os.Getenv("MACKEREL_AGENT_PLUGIN_META") != "" {
		helper.OutputDefinitions()
	} else {
		helper.OutputValues()
	}
}
