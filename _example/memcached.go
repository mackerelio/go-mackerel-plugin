package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	mph "github.com/mackerelio/go-mackerel-plugin-helper"
	"net"
	"os"
	"strconv"
	"strings"
)

var target string

var graphs [](mph.Graphs) = [](mph.Graphs){
	mph.Graphs{
		Key:   "memcached.connections",
		Label: "Memcached Connections",
		Unit:  "integer",
		Metrics: [](mph.Metrics){
			mph.Metrics{Key: "curr_connections", Label: "Connections", Diff: false},
		},
	},
	mph.Graphs{
		Key:   "memcached.cmd",
		Label: "Memcached Command",
		Unit:  "integer",
		Metrics: [](mph.Metrics){
			mph.Metrics{Key: "cmd_get", Label: "Get", Diff: true},
			mph.Metrics{Key: "cmd_set", Label: "Set", Diff: true},
			mph.Metrics{Key: "cmd_flush", Label: "Flush", Diff: true},
			mph.Metrics{Key: "cmd_touch", Label: "Touch", Diff: true},
		},
	},
	mph.Graphs{
		Key:   "memcached.hitmiss",
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
	mph.Graphs{
		Key:   "memcached.evictions",
		Label: "Memcached Evictions",
		Unit:  "integer",
		Metrics: [](mph.Metrics){
			mph.Metrics{Key: "evictions", Label: "Evictions", Diff: true},
		},
	},
	mph.Graphs{
		Key:   "memcached.unfetched",
		Label: "Memcached Unfetched",
		Unit:  "integer",
		Metrics: [](mph.Metrics){
			mph.Metrics{Key: "expired_unfetched", Label: "Expired unfetched", Diff: true},
			mph.Metrics{Key: "evicted_unfetched", Label: "Evicted unfetched", Diff: true},
		},
	},
	mph.Graphs{
		Key:   "memcached.rusage",
		Label: "Memcached Resouce Usage",
		Unit:  "float",
		Metrics: [](mph.Metrics){
			mph.Metrics{Key: "rusage_user", Label: "User", Diff: true},
			mph.Metrics{Key: "rusage_system", Label: "System", Diff: true},
		},
	},
	mph.Graphs{
		Key:   "memcached.bytes",
		Label: "Memcached Traffics",
		Unit:  "bytes",
		Metrics: [](mph.Metrics){
			mph.Metrics{Key: "bytes_read", Label: "Read", Diff: true},
			mph.Metrics{Key: "bytes_written", Label: "Write", Diff: true},
		},
	},
}

func read_stat() (map[string]float64, error) {
	conn, err := net.Dial("tcp", target)
	if err != nil {
		return nil, err
	}
	fmt.Fprintln(conn, "stats")
	r := bufio.NewReader(conn)
	line, isPrefix, err := r.ReadLine()

	stat := make(map[string]float64)
	for err == nil && !isPrefix {
		s := string(line)
		if s == "END" {
			return stat, nil
		}
		res := strings.Split(s, " ")
		if res[0] == "STAT" {
			stat[res[1]], err = strconv.ParseFloat(res[2], 64)
			if err != nil {
				fmt.Println("read_stat:", err)
			}
		}
		line, isPrefix, err = r.ReadLine()
	}
	if isPrefix {
		return nil, errors.New("buffer size too small")
	}
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func main() {
	opt_host := flag.String("host", "localhost", "Hostname")
	opt_port := flag.String("port", "11211", "Port")
	opt_tempfile := flag.String("tempfile", "", "Temp file name")
	flag.Parse()

	target = fmt.Sprintf("%s:%s", *opt_host, *opt_port)
	var tempfile string
	if *opt_tempfile != "" {
		tempfile = *opt_tempfile
	} else {
		tempfile = fmt.Sprintf("/tmp/mackerel-plugin-memcached-%s-%s", *opt_host, *opt_port)
	}

	helper := mph.MackerelPluginHelper{tempfile, read_stat, graphs}
	fmt.Println(helper)

	if os.Getenv("MACKEREL_AGENT_PLUGIN_META") != "" {
		helper.Output_definitions()
	} else {
		helper.Output_values()
	}
}
