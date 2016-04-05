This package is deprecated. You should use [go-mackerel-plugin-helper](https://github.com/mackerelio/go-mackerel-plugin-helper).
==========


go-mackerel-plugin [![Build Status](https://travis-ci.org/mackerelio/go-mackerel-plugin.svg?branch=master)](https://travis-ci.org/mackerelio/go-mackerel-plugin)
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

## Adjust Scale Value

Some status values such as `jstat` memory usage are provided as scaled values.
For example, `OGC` value are provided KB scale.

`Scale` of `Metrics` is a multiplier for adjustment of the scale values.

```golang
var graphdef map[string](Graphs) = map[string](Graphs){
    "jvm.old_space": mp.Graphs{
        Label: "JVM Old Space memory",
        Unit:  "float",
        Metrics: [](mp.Metrics){
            mp.Metrics{Name: "OGCMX", Label: "Old max", Diff: false, Scale: 1024},
            mp.Metrics{Name: "OGC", Label: "Old current", Diff: false, Scale: 1024},
            mp.Metrics{Name: "OU", Label: "Old used", Diff: false, Scale: 1024},
        },
    },
}
```
