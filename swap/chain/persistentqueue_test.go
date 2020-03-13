// Copyright 2020 The Swarm Authors
// This file is part of the Swarm library.
//
// The Swarm library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The Swarm library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the Swarm library. If not, see <http://www.gnu.org/licenses/>.
package chain

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ethersphere/swarm/state"
)

// TestNewPersistentQueue adds 200 elements in one routine and waits for them and then deletes them in another
func TestNewPersistentQueue(t *testing.T) {
	store := state.NewInmemoryStore()
	defer store.Close()

	queue := newPersistentQueue(store, "testq")
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	var lock sync.Mutex   // lock for the queue
	var wg sync.WaitGroup // wait group to wait for both routines to terminate
	wg.Add(2)

	count := 200

	var errout error // stores the last error that occurred in one of the routines
	go func() {
		defer wg.Done()
		for i := 0; i < count; i++ {
			func() { // this is a function so we can use defer with the right scope
				var value uint64
				key, err := queue.next(ctx, &value, &lock)
				if err != nil {
					errout = fmt.Errorf("failed to get next item: %v", err)
					return
				}
				defer lock.Unlock()

				if key == "" {
					errout = errors.New("key is empty")
					return
				}

				if value != uint64(i) {
					errout = fmt.Errorf("values don't match: got %v, expected %v", value, i)
					return
				}

				batch := new(state.StoreBatch)
				queue.delete(batch, key)
				err = store.WriteBatch(batch)
				if err != nil {
					errout = fmt.Errorf("could not write batch: %v", err)
					return
				}
			}()
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < count; i++ {
			func() { // this is a function so we can use defer with the right scope
				lock.Lock()
				defer lock.Unlock()

				var value = uint64(i)
				batch := new(state.StoreBatch)
				_, trigger, err := queue.enqueue(batch, value)
				if err != nil {
					errout = fmt.Errorf("failed to queue item: %v", err)
					return
				}
				err = store.WriteBatch(batch)
				if err != nil {
					errout = fmt.Errorf("failed to write batch: %v", err)
					return
				}

				trigger()
			}()
		}
	}()

	wg.Wait()

	if errout != nil {
		t.Fatal(errout)
	}
}