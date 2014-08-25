go-mackerel-plugin
==================

This package provides helper methods to create mackerel agent plugin easily.


How to use
==========

## Graph Definition

Example of graph definition.
`Graphs` is a type represents one graph and `Metrics` is a type represents each line.

```golang
var graphdef map[string](Graphs) = map[string](Graphs){
	"memcached.connections": mp.Graphs{
		Label: "Memcached Connections",
		Unit:  "integer",
		Metrics: [](Metrics){
			Metrics{Key: "curr_connections", Label: "Connections", Diff: false},
		},
	},
}
```

## Method
- FetchData
  - fetch status data
- GetGraphDefinition
  - output graph definition.
- GetTempfilename
  - output temporally filename.

## Calculate Differential of Counter

Many status values of popular middlewares are provided as counter.
But current Mackerel API can accept only absolute values, so differential values must be caculated beside agent plugins.

`Diff` of `Metrics` is a flag wheather values must be treated as counter or not.
If this flag is set, this package calculate differential values automatically with current values and previous values, which are saved to a temporally file.

