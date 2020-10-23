# go-patchutils [![GoDoc](https://godoc.org/github.com/google/go-patchutils?status.svg)](https://godoc.org/github.com/google/go-patchutils) [![Go Report Card](https://goreportcard.com/badge/github.com/google/go-patchutils)](https://goreportcard.com/report/github.com/google/go-patchutils)
Package patchutils provides tools to compute the diff between source and diff files.

Works with diff files in [unified format](http://gnu.org/software/diffutils/manual/html_node/Unified-Format.html).

### Table of contents 
* [Design](https://github.com/google/go-patchutils#design) 
    * [Interdiff mode](https://github.com/google/go-patchutils#interdiff-mode) 
    * [Mixed mode](https://github.com/google/go-patchutils#mixed-mode) 
* [Installation](https://github.com/google/go-patchutils#installation) 
* [Usage](https://github.com/google/go-patchutils#usage) 
    * [API](https://github.com/google/go-patchutils#api)
    * [CLI tool](https://github.com/google/go-patchutils#cli-tool)

## Design

### Interdiff mode
<img src="https://github.com/google/go-patchutils/blob/main/docs/interdiff_mode.png" width="300">

**InterDiff** computes the diff of a source file patched with **oldDiff**
and the same source file patched with **newDiff**, without access to the source.

<details>    
<summary><b>Example</b></summary>

<table>
   <tr>
      <th>oldDiff</th>
      <th>newDiff</th>
   </tr>
<tr>
<td>

```diff
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
```
</td>
<td>

```diff
@@ -2,9 +2,13 @@
 We’ll find a pot of gold
 just sitting where the rainbow’s ending.
 
+There’s a voice that keeps on calling me.
+Who’s intellectual close friends get to call,
+providing it’s with dignity.
+The indisputable leader of the gang.

 Time — we’ll fight against the time,
 and we’ll fly on the white wings of the wind.
 
-80 days around the world,
 no we won’t say a word before
 the ship is really back.
```
</td>
</tr>
<tr>
    <th colspan="2">result</th>
</tr>
<tr>
<td colspan="2">

```diff
@@ -1,8 +1,8 @@
 80 days around the world.
+We’ll find a pot of gold
-You’ll find a pot of gold
 just sitting where the rainbow’s ending.
 
-Top Cat! The most effectual Top Cat!
+There’s a voice that keeps on calling me.
 Who’s intellectual close friends get to call,
 providing it’s with dignity.
 The indisputable leader of the gang.
 
 Time — we’ll fight against the time,
 and we’ll fly on the white wings of the wind.
 
+no we won’t say a word before
 the ship is really back.
```
</td>
</tr>
</table>

</details>

### Mixed mode
<img src="https://github.com/google/go-patchutils/blob/main/docs/mixed_mode.png" width="300">

**MixedMode** computes the diff of an **oldSource** patched with **oldDiff** and
**newSource** patched with **newDiff**.

<details>
<summary><b>Example</b></summary>
   
<table>
   <tr>
      <th>oldSource</th>
      <th>newSource</th>
   </tr>
<tr>
<td>
         
```
80 days around the world.
You’ll find a pot of gold
just sitting where the rainbow’s ending.

Top Cat! The most effectual Top Cat!
Who’s intellectual close friends get to call,
providing it’s with dignity.
The indisputable leader of the gang.

Time — we’ll fight against the time,
and we’ll fly on the white wings of the wind.

the ship is really back.
```
</td>
<td>

```
80 days around the world.
We’ll find a pot of gold
just sitting where the rainbow’s ending.

There’s a voice that keeps on calling me.
Who’s intellectual close friends get to call,
providing it’s with dignity.
The indisputable leader of the gang.

Time — we’ll fight against the time,
and we’ll fly on the white wings of the wind.

no we won’t say a word before
the ship is really back.
```
</td>
</tr>
   <tr>
      <th>oldDiff</th>
      <th>newDiff</th>  
   </tr>
<tr>
<td>

```diff
@@ -4,6 +4,7 @@
 
 Top Cat! The most effectual Top Cat!
 Who’s intellectual close friends get to call,
+Round, round, all around the world.
 providing it’s with dignity.
 The indisputable leader of the gang.
```
</td>
<td>

```diff
@@ -5,7 +5,6 @@
 There’s a voice that keeps on calling me.
 Who’s intellectual close friends get to call,
 providing it’s with dignity.
-The indisputable leader of the gang.
 
 Time — we’ll fight against the time,
 and we’ll fly on the white wings of the wind.
```
</td>
</tr>
<tr>
    <th><em>oldSource + oldDiff</em></th>
    <th><em>newSource + newDiff</em></th> 
</tr>
<tr>
<td>

```diff
80 days around the world.
You’ll find a pot of gold
just sitting where the rainbow’s ending.

Top Cat! The most effectual Top Cat!
Who’s intellectual close friends get to call,
Round, round, all around the world.
providing it’s with dignity.
The indisputable leader of the gang.

Time — we’ll fight against the time,
and we’ll fly on the white wings of the wind.

the ship is really back.
```
</td>
<td>
   
```
80 days around the world.
We’ll find a pot of gold
just sitting where the rainbow’s ending.

There’s a voice that keeps on calling me.
Who’s intellectual close friends get to call,
providing it’s with dignity.

Time — we’ll fight against the time,
and we’ll fly on the white wings of the wind.

no we won’t say a word before
the ship is really back.
```
</td>
</tr>
<tr>
   <th colspan="2">result</th>
</tr>
<tr>
<td colspan="2">

```diff
@@ -1,14 +1,13 @@
 80 days around the world.
-You’ll find a pot of gold
+We’ll find a pot of gold
 just sitting where the rainbow’s ending.
 
-Top Cat! The most effectual Top Cat!
+There’s a voice that keeps on calling me.
 Who’s intellectual close friends get to call,
-Round, round, all around the world.
 providing it’s with dignity.
-The indisputable leader of the gang.
 
 Time — we’ll fight against the time,
 and we’ll fly on the white wings of the wind.
 
+no we won’t say a word before
 the ship is really back.
```
</td>
</tr>
</table>
</details>

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

