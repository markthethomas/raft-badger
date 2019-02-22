# raft-badger

[![CircleCI](https://circleci.com/gh/markthethomas/raft-badger/tree/master.svg?style=svg)](https://circleci.com/gh/markthethomas/raft-badger/tree/master) [![Go Report Card](https://goreportcard.com/badge/github.com/markthethomas/raft-badger)](https://goreportcard.com/report/github.com/markthethomas/raft-badger) [![Maintainability](https://api.codeclimate.com/v1/badges/2aef013ae290d9233ac5/maintainability)](https://codeclimate.com/github/markthethomas/raft-badger/maintainability) [![codecov.io Code Coverage](https://img.shields.io/codecov/c/github/markthethomas/raft-badger.svg?maxAge=2592000)](https://codecov.io/github/markthethomas/raft-badger?branch=master) [![HitCount](http://hits.dwyl.io/markthethomas/github.com/markthethomas/raft-badger.svg)](http://hits.dwyl.io/markthethomas/github.com/markthethomas/raft-badger) [![GoDoc](https://godoc.org/github.com/markthethomas/raft-badger?status.png)](https://godoc.org/github.com/markthethomas/raft-badger)

![Raft + Badger backend plugin](https://cdn.ifelse.io/images/raft-badger.png)

This repository provides a storage backend for the excellent [raft package](https://github.com/hashicorp/raft) from Hashicorp. Raft is a [distributed consensus](https://en.wikipedia.org/wiki/Consensus_(computer_science)) protocol. Distributed consensus has *many* practical applications, ranging from fault-tolerant databases to clock synchronization to things like Google's PageRank.

This package exports the `BadgerStore`, which is an implementation of both a `LogStore` and `StableStore` (interfaces used by the raft package for reading/writing logs as part of its consensus protocol).

- [raft-badger](#raft-badger)
  - [installation](#installation)
  - [usage](#usage)
  - [developing](#developing)
  - [motivation](#motivation)
  - [misc.](#misc)
  - [todo](#todo)

## installation

```bash
go get -u github.com/markthethomas/raft-badger
```

## usage

Create a new BadgerStore and pass it to Raft when setting up.

```go
//...
var logStore raft.LogStore
var stableStore raft.StableStore
myPath  := filepath.Join(s.RaftDir) // replace this with what you're actually using
badgerDB, err := raftbadgerdb.NewBadgerStore(myPath)
if err != nil {
  return fmt.Errorf("error creating new badger store: %s", err)
}
logStore = badgerDB
stableStore = badgerDB

r, err := raft.NewRaft(config, (*fsm)(s), logStore, stableStore, snapshots, transport)
//...
```

## developing

To run tests, run:

```bash
go test -cover -coverprofile=coverage.out .
```

To view coverage, run:

```bash
go tool cover -html=coverage.out
```

To run the benchmark, run:

```bash
go test -race -bench .
```

## motivation

This package is meant to be used with the [raft package](https://github.com/hashicorp/raft) from Hashicorb. This package borrows heavily from the excellent [raft-boltdb](https://github.com/hashicorp/raft-boltdb) package, also from Hashicorp. I wanted to learn about Badger and similar tools and needed to use Raft + a durable backend.

## misc.

-   raft-badger uses prefix keys to "bucket" logs and config, avoiding the need for multiple badger database files for each type of k/v raft sets
-   encodes/decodes the raft [Log](https://godoc.org/github.com/hashicorp/raft#Log) types using Go's [gob](https://golang.org/pkg/encoding/gob/) for efficient encoding/decoding of keys See more at https://blog.golang.org/gobs-of-data.
-   images used are from the [raft website](https://raft.github.io) and [the badger repository](https://github.com/dgraph-io/badger), respectively
-   thanks to the authors of the excellent [raft-boltdb](https://github.com/hashicorp/raft-boltdb) package for providing patterns to follow in satisfying the requisite raft interfaces 🙌
-   curious to learn more about the raft protocol? check out [the raft website](https://raft.github.io). There's also a beginner's guide at [Free Code Camp](https://medium.freecodecamp.org/in-search-of-an-understandable-consensus-algorithm-a-summary-4bc294c97e0d)

## todo

-   support custom badger options
-   explore other encodings besides `gob`
-   add more examples of use with raft
