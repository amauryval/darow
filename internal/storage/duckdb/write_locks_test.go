package duckdb

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"darow/internal/storage"
)

func Test_writeLocks_acquire(t *testing.T) {
	cases := []struct {
		name           string
		first          storage.WriteScope
		second         storage.WriteScope
		wantConcurrent bool
	}{
		{
			name:           "same scope serializes",
			first:          storage.TableWriteScope("a"),
			second:         storage.TableWriteScope("a"),
			wantConcurrent: false,
		},
		{
			name:           "different tables run in parallel",
			first:          storage.TableWriteScope("a"),
			second:         storage.TableWriteScope("b"),
			wantConcurrent: true,
		},
		{
			name:           "table and global run in parallel",
			first:          storage.TableWriteScope("a"),
			second:         storage.GlobalWriteScope,
			wantConcurrent: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var locks writeLocks
			release := locks.acquire(c.first)

			var acquired atomic.Bool
			done := make(chan struct{})
			go func() {
				defer close(done)
				secondRelease := locks.acquire(c.second)
				acquired.Store(true)
				secondRelease()
			}()

			assert.Eventually(t, func() bool {
				return acquired.Load() == c.wantConcurrent
			}, time.Second, time.Millisecond)

			release()
			<-done
			assert.True(t, acquired.Load())
		})
	}
}

func Test_writeLocks_acquire_releasesEntries(t *testing.T) {
	var locks writeLocks

	release := locks.acquire(storage.TableWriteScope("a"))
	require.Len(t, locks.entries, 1)
	release()
	assert.Empty(t, locks.entries)
}

func Test_writeLocks_acquire_serializesConcurrentWriters(t *testing.T) {
	var locks writeLocks
	var wg sync.WaitGroup
	var inFlight, maxInFlight atomic.Int32

	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release := locks.acquire(storage.TableWriteScope("a"))
			defer release()

			current := inFlight.Add(1)
			for {
				peak := maxInFlight.Load()
				if current <= peak || maxInFlight.CompareAndSwap(peak, current) {
					break
				}
			}
			time.Sleep(time.Millisecond)
			inFlight.Add(-1)
		}()
	}
	wg.Wait()

	assert.Equal(t, int32(1), maxInFlight.Load())
	assert.Empty(t, locks.entries)
}
