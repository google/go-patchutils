# go-patchutils [![GoDoc](https://godoc.org/github.com/google/go-patchutils?status.svg)](https://godoc.org/github.com/google/go-patchutils) [![Go Report Card](https://goreportcard.com/badge/github.com/google/go-patchutils)](https://goreportcard.com/report/github.com/google/go-patchutils)
Package patchutils provides tools to compute the diff between source and diff files.

Works with diff files in [unified format](http://gnu.org/software/diffutils/manual/html_node/Unified-Format.html).

### Agenda 
* [Design](https://github.com/google/go-patchutils#design) 
    * [Interdiff mode](https://github.com/google/go-patchutils#interdiff-mode) 
    * [Mixed mode](https://github.com/google/go-patchutils#mixed-mode) 
* [Installation](https://github.com/google/go-patchutils#installation) 
* [Usage](https://github.com/google/go-patchutils#usage) 
    * [API](https://github.com/google/go-patchutils#api)
    * [CLI tool](https://github.com/google/go-patchutils#cli-tool)

## Design

### Interdiff mode
![Interdiff mode](https://github.com/google/go-patchutils/docs/interdiff_mode.png)
InterDiff computes the diff of a source file patched with oldDiff
and the same source file patched with newDiff, without access to the source.

**Example**

<table>
    <tr>
        <th>oldDiff</th>
        <th>newDiff</th>
        <th>result</th>
    </tr>
    <tr>
        <pre lang="diff">
        @@ -1,10 +1,13 @@
         80 days around the world.
        -We’ll find a pot of gold
        +You’ll find a pot of gold
         just sitting where the rainbow’s ending.
        +Top Cat! The most effectual Top Cat!
        +Who’s intellectual close friends get to call,
        +providing it’s with dignity.
        +The indisputable leader of the gang.
         Time — we’ll fight against the time,
         and we’ll fly on the white wings of the wind.
        -80 days around the world,
        -no we won’t say a word before
         the ship is really back.
        </pre>
        <pre lang="diff">
        @@ -2,9 +2,13 @@
         We’ll find a pot of gold
         just sitting where the rainbow’s ending.
        +There’s a voice that keeps on calling me.
        +Who’s intellectual close friends get to call him T.C.,
        +providing it’s with dignity.
        +The indisputable leader of the gang.
         Time — we’ll fight against the time,
         and we’ll fly on the white wings of the wind.
        -80 days around the world,
         no we won’t say a word before
         the ship is really back.
         </pre>
         <pre lang="diff">
        @@ -1,8 +1,8 @@
         80 days around the world.
        +We’ll find a pot of gold
        -You’ll find a pot of gold
         just sitting where the rainbow’s ending.
        -Top Cat! The most effectual Top Cat!
        +There’s a voice that keeps on calling me.
         Who’s intellectual close friends get to call him T.C.,
         providing it’s with dignity.
         The indisputable leader of the gang.
         Time — we’ll fight against the time,
         and we’ll fly on the white wings of the wind.
        +no we won’t say a word before
         the ship is really back.
         </pre>
    </tr>
</table>

### Mixed mode
![Mixed mode](https://github.com/google/go-patchutils/docs/mixed_mode.png)
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

