# skiplist

<p>
    <a href="https://github.com/garrettladley/skiplist/releases"><img src="https://img.shields.io/github/release/garrettladley/skiplist" alt="Latest Release"></a>
    <a href="https://github.com/garrettladley/skiplist/actions"><img src="https://github.com/garrettladley/skiplist/actions/workflows/ci.yml/badge.svg" alt="Build Status"></a>
</p>

`skiplist` is a generic ordered map backed by a probabilistic skip list.

It stores keys in sorted order and provides expected `O(log n)` `Get`, `Set`,
and `Delete` operations.

## Install

```sh
go get github.com/garrettladley/skiplist
```

Requires Go 1.23 or newer.

## Usage

```go
package main

import (
	"fmt"

	"github.com/garrettladley/skiplist"
)

func main() {
	var l skiplist.List[int, string]

	l.Set(3, "three")
	l.Set(1, "one")
	l.Set(2, "two")

	value, ok := l.Get(2)
	fmt.Println(value, ok)

	for key, value := range l.All() {
		fmt.Println(key, value)
	}
}
```

## API

`List[K, V]` accepts any `cmp.Ordered` key type.

- `Get` returns the value for a key.
- `Set` inserts or overwrites a key.
- `Delete` removes a key.
- `Len` returns the number of keys.
- `All` iterates key/value pairs in ascending key order.
- `Min` and `Max` return the smallest and largest key/value pairs.

The zero value is ready to use. Use `New` when passing options such as
`WithMaxLevel`, `WithProbability`, or `WithRand`.

## Concurrency

`List` is not safe for concurrent use by multiple goroutines. If a list is
shared across goroutines, serialize access externally.
