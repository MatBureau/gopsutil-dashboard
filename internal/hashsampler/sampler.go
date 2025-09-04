package hashsampler

import (
	"context"
	"sync"
	"time"

	"github.com/MatBureau/gopsutil-dashboard/internal/system"
)

type Sampler struct {
	mu     sync.RWMutex
	last   *system.HashRandom
	lastAt time.Time
	err    error
}

func Start(ctx context.Context, every time.Duration) *Sampler {
	s := &Sampler{}

	go func() {
		t := time.NewTicker(every)
		defer t.Stop()

		sample := func() {
			val, err := system.CollectHash(ctx)
			s.mu.Lock()
			s.last, s.err, s.lastAt = val, err, time.Now()
			s.mu.Unlock()
		}

		// ➜ premier échantillon tout de suite
		sample()

		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				sample()
			}
		}
	}()

	return s
}

func (s *Sampler) Snapshot() (*system.HashRandom, time.Time, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.last, s.lastAt, s.err
}
