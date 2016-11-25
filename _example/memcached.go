package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"

	"github.com/mackerelio/go-mackerel-plugin"
)

var graphdef map[string]mackerelplugin.Graphs = map[string]mackerelplugin.Graphs{
	"memcached.connections": {
		Label: "Memcached Connections",
		Unit:  "integer",
		Metrics: []mackerelplugin.Metrics{
			{Name: "curr_connections", Label: "Connections", Diff: false},
		},
	},
	"memcached.cmd": {
		Label: "Memcached Command",
		Unit:  "integer",
		Metrics: []mackerelplugin.Metrics{
			{Name: "cmd_get", Label: "Get", Diff: true},
			{Name: "cmd_set", Label: "Set", Diff: true},
			{Name: "cmd_flush", Label: "Flush", Diff: true},
			{Name: "cmd_touch", Label: "Touch", Diff: true},
		},
	},
	"memcached.hitmiss": {
		Label: "Memcached Hits/Misses",
		Unit:  "integer",
		Metrics: []mackerelplugin.Metrics{
			{Name: "get_hits", Label: "Get Hits", Diff: true},
			{Name: "get_misses", Label: "Get Misses", Diff: true},
			{Name: "delete_hits", Label: "Delete Hits", Diff: true},
			{Name: "delete_misses", Label: "Delete Misses", Diff: true},
			{Name: "incr_hits", Label: "Incr Hits", Diff: true},
			{Name: "incr_misses", Label: "Incr Misses", Diff: true},
			{Name: "cas_hits", Label: "Cas Hits", Diff: true},
			{Name: "cas_misses", Label: "Cas Misses", Diff: true},
			{Name: "touch_hits", Label: "Touch Hits", Diff: true},
			{Name: "touch_misses", Label: "Touch Misses", Diff: true},
		},
	},
	"memcached.evictions": {
		Label: "Memcached Evictions",
		Unit:  "integer",
		Metrics: []mackerelplugin.Metrics{
			{Name: "evictions", Label: "Evictions", Diff: true},
		},
	},
	"memcached.unfetched": {
		Label: "Memcached Unfetched",
		Unit:  "integer",
		Metrics: []mackerelplugin.Metrics{
			{Name: "expired_unfetched", Label: "Expired unfetched", Diff: true},
			{Name: "evicted_unfetched", Label: "Evicted unfetched", Diff: true},
		},
	},
	"memcached.rusage": {
		Label: "Memcached Resouce Usage",
		Unit:  "float",
		Metrics: []mackerelplugin.Metrics{
			{Name: "rusage_user", Label: "User", Diff: true},
			{Name: "rusage_system", Label: "System", Diff: true},
		},
	},
	"memcached.bytes": {
		Label: "Memcached Traffics",
		Unit:  "bytes",
		Metrics: []mackerelplugin.Metrics{
			{Name: "bytes_read", Label: "Read", Diff: true},
			{Name: "bytes_written", Label: "Write", Diff: true},
		},
	},
}

type MemcachedPlugin struct {
	Target   string
	Tempfile string
}

func (m MemcachedPlugin) FetchMetrics() (map[string]float64, error) {
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
				log.Println("FetchMetrics:", err)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return stat, err
	}
	return nil, nil
}

func (m MemcachedPlugin) GraphDefinition() map[string]mackerelplugin.Graphs {
	return graphdef
}

func main() {
	optHost := flag.String("host", "localhost", "Hostname")
	optPort := flag.String("port", "11211", "Port")
	optTempfile := flag.String("tempfile", "", "Temp file name")
	flag.Parse()

	var memcached MemcachedPlugin

	memcached.Target = fmt.Sprintf("%s:%s", *optHost, *optPort)
	helper := mackerelplugin.NewMackerelPlugin(memcached)
	helper.Tempfile = *optTempfile
	helper.Run()
}
