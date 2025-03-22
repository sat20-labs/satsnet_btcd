satoshinet
====

[![Build Status](https://github.com/sat20-labs/satoshinet/workflows/Build%20and%20Test/badge.svg)](https://github.com/sat20-labs/satoshinet/actions)
[![ISC License](https://img.shields.io/badge/license-ISC-blue.svg)](http://copyfree.org)
[![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg)](https://pkg.go.dev/github.com/sat20-labs/satoshinet)

satoshinet is an alternative full node SatoshiNet implementation written in Go (golang), forks from btcd.

This project is currently under active development and is in a Beta state. 


## Requirements

[Go](http://golang.org) 1.22 or newer.

## Installation

https://github.com/sat20-labs/satoshinet/releases

#### Linux/BSD/MacOSX/POSIX - Build from Source

- Install Go according to the installation instructions here:
  http://golang.org/doc/install

- Ensure Go was installed properly and is a supported version:

```bash
$ go version
$ go env GOROOT GOPATH
```

NOTE: The `GOROOT` and `GOPATH` above must not be the same path.  It is
recommended that `GOPATH` is set to a directory in your home directory such as
`~/goprojects` to avoid write permission issues.  It is also recommended to add
`$GOPATH/bin` to your `PATH` at this point.

- Run the following commands to obtain satoshinet, all dependencies, and install it:

```bash
$ cd $GOPATH/src/github.com/sat20-labs/satoshinet
$ go install -v . ./cmd/...
```

- satoshinet (and utilities) will now be installed in ```$GOPATH/bin```.  If you did
  not already add the bin directory to your system path during Go installation,
  we recommend you do so now.

## Updating

#### Linux/BSD/MacOSX/POSIX - Build from Source

- Run the following commands to update satoshinet, all dependencies, and install it:

```bash
$ cd $GOPATH/src/github.com/sat20-labs/satoshinet
$ git pull
$ go install -v . ./cmd/...
```

## Getting Started

satoshinet has several configuration options available to tweak how it runs, but all
of the basic operations described in the intro section work with zero
configuration.

#### Linux/BSD/POSIX/Source

```bash
$ ./satoshinet
```

## Issue Tracker

The [integrated github issue tracker](https://github.com/sat20-labs/satoshinet/issues)
is used for this project.

## Documentation

The documentation is a work-in-progress.  It is located in the [docs](https://github.com/sat20-labs/satoshinet/tree/master/docs) folder.

## Release Verification

Please see our [documentation on the current build/verification
process](https://github.com/sat20-labs/satoshinet/tree/master/release) for all our
releases for information on how to verify the integrity of published releases
using our reproducible build system.

## License

satoshinet is licensed under the [copyfree](http://copyfree.org) ISC License.
