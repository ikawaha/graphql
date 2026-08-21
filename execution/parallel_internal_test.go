package execution

// The scheduling of concurrent field and entry work, measured on its own.
//
// The executor's bound on concurrency is a mechanism with several plausible
// shapes, and which one is better is a question about goroutines and channels
// rather than about GraphQL. Comparing them here — with no schema, no
// document and no resolvers in the way — is what makes the difference visible
// at all: against a resolver that waits on a database it would be lost in the
// noise, and against one that answers from memory it is most of the cost.

import (
	"sync"
	"sync/atomic"
	"testing"
)

// spawnPerItem is what the executor does today: one goroutine per piece of
// work, and a buffered channel as the bound. A goroutine is started only once
// it has taken a slot, so a wide selection set cannot spawn one per field.
func spawnPerItem(limit, n int, run func(int)) {
	if limit > n {
		limit = n
	}
	slots := make(chan struct{}, limit)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		slots <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-slots }()
			run(i)
		}()
	}
	wg.Wait()
}

// workersOverChannel is the textbook pool: as many goroutines as the bound
// allows, each taking indices off a channel until it is closed.
func workersOverChannel(limit, n int, run func(int)) {
	if limit > n {
		limit = n
	}
	indices := make(chan int)
	var wg sync.WaitGroup
	wg.Add(limit)
	for range limit {
		go func() {
			defer wg.Done()
			for i := range indices {
				run(i)
			}
		}()
	}
	for i := range n {
		indices <- i
	}
	close(indices)
	wg.Wait()
}

// workersOverCounter is the same pool with the channel replaced by an atomic
// counter: the work is a range of integers, so there is nothing to send.
func workersOverCounter(limit, n int, run func(int)) {
	if limit > n {
		limit = n
	}
	var next atomic.Int64
	var wg sync.WaitGroup
	wg.Add(limit)
	for range limit {
		go func() {
			defer wg.Done()
			for {
				i := int(next.Add(1)) - 1
				if i >= n {
					return
				}
				run(i)
			}
		}()
	}
	wg.Wait()
}

// workersWithCaller is workersOverCounter with the caller taking a share
// instead of standing and waiting, which is one goroutine fewer and, where the
// bound is one, none at all.
func workersWithCaller(limit, n int, run func(int)) {
	if limit > n {
		limit = n
	}
	var next atomic.Int64
	var wg sync.WaitGroup
	take := func() {
		for {
			i := int(next.Add(1)) - 1
			if i >= n {
				return
			}
			run(i)
		}
	}
	wg.Add(limit - 1)
	for range limit - 1 {
		go func() {
			defer wg.Done()
			take()
		}()
	}
	take()
	wg.Wait()
}

// unbounded is the comparison the bound exists to rule out: a goroutine per
// piece of work and nothing holding them back.
func unbounded(_, n int, run func(int)) {
	var wg sync.WaitGroup
	wg.Add(n)
	for i := range n {
		go func() {
			defer wg.Done()
			run(i)
		}()
	}
	wg.Wait()
}

// asShipped is what the executor does, reached through an executor so that
// the measurement is of the code that runs and not of a copy of it.
func asShipped(limit, n int, run func(int)) {
	e := &executor{concurrency: limit}
	e.inParallel(n, run)
}

var parallelSchedulers = []struct {
	name string
	run  func(limit, n int, run func(int))
}{
	{"as shipped", asShipped},
	{"spawn per item, channel bound", spawnPerItem},
	{"workers over a channel", workersOverChannel},
	{"workers over a counter", workersOverCounter},
	{"workers, caller takes a share", workersWithCaller},
	{"unbounded", unbounded},
}

// sink keeps the compiler from removing the work.
var sink atomic.Int64

// The three shapes of work a field can be: nothing at all, which measures the
// scheduling on its own; a little arithmetic, which is what a resolver reading
// from memory costs; and a great deal, which is a resolver that computes.
var parallelWork = []struct {
	name  string
	spins int
}{
	{"no work", 0},
	{"100ns of work", 40},
	{"2µs of work", 900},
}

func spin(n int) {
	total := 0
	for i := range n {
		total += i * i
	}
	sink.Add(int64(total & 1))
}

func BenchmarkParallelScheduling(b *testing.B) {
	for _, work := range parallelWork {
		b.Run(work.name, func(b *testing.B) {
			run := func(int) { spin(work.spins) }
			for _, n := range []int{4, 16, 200} {
				for _, limit := range []int{4, 12} {
					b.Run(schedulingCase(n, limit), func(b *testing.B) {
						for _, s := range parallelSchedulers {
							b.Run(s.name, func(b *testing.B) {
								b.ReportAllocs()
								for b.Loop() {
									s.run(limit, n, run)
								}
							})
						}
					})
				}
			}
		})
	}
}

func schedulingCase(n, limit int) string {
	return "n=" + itoa(n) + ",limit=" + itoa(limit)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
