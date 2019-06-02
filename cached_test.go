package cachedrander

import (
	"io"
	"testing"

	"github.com/google/uuid"
)

type gen struct {
	size       int
	head, tail int
	fills      int
}

func (g *gen) Read(buf []byte) (int, error) {
	if len(buf) > g.size {
		buf = buf[:g.size]
	}
	if g.head == g.tail {
		g.fills++
		g.tail += g.size
	}
	for j := range buf {
		if g.head == g.tail {
			return j, nil
		}
		buf[j] = byte(g.head)
		g.head++
	}
	return len(buf), nil
}

func TestReader(t *testing.T) {
	g := &gen{size: 17}
	r, err := New(g, 1024)
	r.Max = 8
	if err != nil {
		t.Fatal(err)
	}
	var buf [64]byte

	next := 0
	for i := 1; i < len(buf); i++ {
		n, err := io.ReadFull(r, buf[:i])
		if err != nil {
			t.Fatal(err)
		}
		for _, b := range buf[:n] {
			if b != byte(next) {
				t.Fatalf("byte %d: got %d, want %d", next, b, byte(next))
			}
			next++
		}
	}
}

func BenchmarkNormal(b *testing.B) {
	b.StopTimer()
	uuid.SetRand(nil)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 10000; j++ {
			uuid.New()
		}
	}
}

func BenchmarkCached(b *testing.B) {
	b.StopTimer()
	r, err := NewUUIDReader(1000)
	if err != nil {
		b.Fatal(err)
	}
	uuid.SetRand(r)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 10000; j++ {
			uuid.New()
		}
	}
}
