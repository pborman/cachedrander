// Package cachedrander provides a reader designed to cache random data for the
// creation of random UUIDs.  Using rand.Reader as the source of random data
// (the default for github.com/google/uuid) requires a mutex operation per newly
// minted version 4 (random) UUID.  This package typically only requires a
// single atomic.AddUint64 per newly minted UUID.
//
// This package works by having two pages of cached random data.  The first page
// is read when the CachedReader is created.  Once that page has been exhausted
// Read calls will block on a mutex while the second page is being loaded.
//
// This package has a theoretical race condition:
//
// Caller A reads the index of its data in the current page and is prempted.
// Prior to resuming a sufficent number of calls to Read are made to exhaust the
// current page and the next loaded page.  It is now possible for caller A to
// return the same data as another caller.
//
// to mitigate this condition the CachedReader should use a sufficiently large
// cache that the probability of this happening is essentially 0.
package cachedrander

import (
	"crypto/rand"
	"io"
	"sync"
	"sync/atomic"
)

// A CachedReader caches chunks of data from a reader and then provides that
// data to calls to its Read method.
//
// The Max value determines the maximum size read that will be honored.  This
// defaults to 16 (the size of a UUID).  Max should only be set prior to the
// first Read of the CachedReader.  Max should be multiple times smaller than
// the size of the cache.
type CachedReader struct {
	Max int

	mu    sync.Mutex
	pages [2][]byte
	size  uint64
	index uint64
	r     io.Reader
}

// NewUUIDReader returns a CachedReader that caches n UUID's worth of data from
// rand.Reader at a time.  The value of n should be sufficiently large to
// prevent the theoretical race conditioned mentioned above (e.g., 100 or 1000)
func NewUUIDReader(n int) (*CachedReader, error) {
	return New(rand.Reader, n*16)
}

// New returns a new CachedReader that caches size bytes from r at a time.  An
// error is returned if filling the initial cache from r returns an error.
func New(r io.Reader, size int) (*CachedReader, error) {
	nr := &CachedReader{
		Max:   16,
		size:  uint64(size),
		pages: [2][]byte{make([]byte, size), make([]byte, size)},
		r:     r,
	}
	// Fill the first cache buffer
	if _, err := io.ReadFull(r, nr.pages[0]); err != nil {
		return nil, err
	}
	return nr, nil
}

// Read fills buf with cached data
func (r *CachedReader) Read(buf []byte) (int, error) {
	if len(buf) > r.Max {
		buf = buf[:r.Max]
	}
	blen := uint64(len(buf))
	for {
		ai := atomic.AddUint64(&r.index, blen)
		page := int(ai >> 32)
		i := ai & 0xffffffff
		if i-blen <= r.size {
			return copy(buf, r.pages[page][i-blen:]), nil
		}
		if err := r.fill(); err != nil {
			return 0, err
		}
	}
}

// fill fills in the cache page we are currently not reading from.
func (r *CachedReader) fill() error {
	r.mu.Lock()
	ai := atomic.LoadUint64(&r.index)
	var err error
	if (ai & 0xffffffff) > r.size {
		page := (ai >> 32) ^ 1
		_, err = io.ReadFull(r.r, r.pages[page])
		atomic.StoreUint64(&r.index, uint64(page)<<32)
	}
	r.mu.Unlock()
	return err
}
