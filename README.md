go-mackerel-plugin [![Build Status](https://travis-ci.org/mackerelio/go-mackerel-plugin.svg?branch=master)](https://travis-ci.org/mackerelio/go-mackerel-plugin)
==================

This package provides helper methods to create mackerel agent plugin easily.


How to use
==========

## Graph Definition
A plugin can specify `Graphs` and `Metrics`.
`Graphs` represents one graph and includes some `Metrics`s which represent each line.

`Graphs` includes followings:

- `Label`: Label for the graph
- `Unit`: Unit for lines, `float`, `integer`, `percentage`, `bytes`, `bytes/sec`, `iops` can be specified.
- `Metrics`: Array of `Metrics` which represents each line.

`Metics` includes followings:

- `Name`: Key of the line
- `Label`: Label of the line
- `Diff`: If `Diff` is true, differential is used as value.
- `Stacked`: If `Stacked` is true, the line is stacked.
- `Scale`: Each value is multiplied by `Scale`.

Example of graph definition.
```golang
var graphdef = map[string]mackerelplugin.Graphs{
	"memcached.connections": {
		Label: "Memcached Connections",
		Unit:  "integer",
		Metrics: []mackerelplugin.Metrics{
			{Key: "curr_connections", Label: "Connections", Diff: false},
		},
	},
}
```

## Method

A plugin must implement this interface and the `main` method.

```go
type PluginWithPrefix interface {
	FetchMetrics() (map[string]interface{}, error)
	GraphDefinition() map[string]Graphs
	MetricKeyPrefix() string
}
```

```go
func main() {
	optHost := flag.String("host", "localhost", "Hostname")
	optPort := flag.String("port", "11211", "Port")
	optTempfile := flag.String("tempfile", "", "Temp file name")
	optMetricKeyPrefix := flag.String("metric-key-prefix", "memcached", "Metric Key Prefix")
	flag.Parse()

	var memcached MemcachedPlugin

	memcached.Target = fmt.Sprintf("%s:%s", *optHost, *optPort)
	memcached.prefix = *optMetricKeyPrefix
	helper := mackerelplugin.NewMackerelPlugin(memcached)
	helper.Tempfile = *optTempfile

	helper.Run()
}
```

You can find an example implementation in _example/ directory.

## Calculate Differential of Counter

Many status values of popular middle-wares are provided as counter.
But current Mackerel API can accept only absolute values, so differential values must be calculated beside agent plugins.

`Diff` of `Metrics` is a flag whether values must be treated as counter or not.
If this flag is set, this package calculate differential values automatically with current values and previous values, which are saved to a temporally file.

## Adjust Scale Value

Some status values such as `jstat` memory usage are provided as scaled values.
For example, `OGC` value are provided KB scale.

`Scale` of `Metrics` is a multiplier for adjustment of the scale values.

```golang
var graphdef = map[string]mackerelplugin.Graphs{
	"jvm.old_space": {
		Label: "JVM Old Space memory",
		Unit:  "float",
		Metrics: []mackerelplugin.Metrics{
			{Name: "OGCMX", Label: "Old max", Diff: false, Scale: 1024},
			{Name: "OGC", Label: "Old current", Diff: false, Scale: 1024},
			{Name: "OU", Label: "Old used", Diff: false, Scale: 1024},
		},
	},
}
```

## Tempfile

`MackerelPlugin` interface has `Tempfile` field. The Tempfile is used to calculate differences in metrics with `Diff: true`.
If this field is omitted, the filename of the temporaty file is automatically generated from plugin filename.

### Default value of Tempfile

mackerel-agent's plugins should place its Tempfile under `os.Getenv("MACKEREL_PLUGIN_WORKDIR")` unless specified explicitly.
Since this helper handles the environmental value, it's recommended not to set default Tempfile path.
But if a plugin wants to set default Tempfile filename by itself, use `MackerelPlugin.SetTempfileByBasename()`, which sets Tempfile path considering the environmental value.

```go
  helper.Tempfile = *optTempfile
  if optTempfile == nil {
    helper.SetTempfileByBasename("YOUR_DEFAULT_FILENAME")
  }
```
