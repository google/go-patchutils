# go-patchutils [![GoDoc](https://godoc.org/github.com/google/go-patchutils?status.svg)](https://godoc.org/github.com/google/go-patchutils) [![Go Report Card](https://goreportcard.com/badge/github.com/google/go-patchutils)](https://goreportcard.com/report/github.com/google/go-patchutils)
Package patchutils provides tools to compute the diff between source and diff files.

Works with diff files in [unified format](http://gnu.org/software/diffutils/manual/html_node/Unified-Format.html).

// TODO: add links
### Agenda 
* Design 
    * Interdiff mode 
    * Mixed mode 
* Installation 
* Usage 

## Design

### Interdiff mode
// TODO: add scheme <br />
InterDiff computes the diff of a source file patched with oldDiff
and the same source file patched with newDiff, without access to the source.

### Mixed mode
// TODO: add scheme <br />
MixedModeFile computes the diff of an oldSource patched with oldDiff and
newSource patched with newDiff.

## Installation

```shell
go get -u github.com/google/go-patchutils
```

## Usage

### API
[Godoc](https://godoc.org/github.com/google/go-patchutils) is available.

### CLI tool

Build CLI tool
```shell
cd cli
go build
```

**Interdiff mode**
```shell
./cli interdiff -olddiff=<path_to_old_diff> -newdiff=<path_to_new_diff>
```

**Mixed mode**
```shell
./cli mixed -oldsource=<path_to_old_source> -olddiff=<path_to_old_diff> 
-newsource=<path_to_new_source> -newdiff=<path_to_new_diff>
```

