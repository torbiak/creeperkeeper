package creeperkeeper

import (
	"log"
	"sync"
)

// parallel runs jobs concurrently.
func parallel(jobs []interface{}, f func(interface{}) error, atOnce int) (nerr int) {
	nerrors := &syncCounter{}

	// Producer
	jobq := make(chan interface{})
	go func() {
		for _, job := range jobs {
			jobq <- job
		}
	}()

	// Consumers
	wg := &sync.WaitGroup{}
	wg.Add(len(jobs))
	for i := 0; i < atOnce; i++ {
		go func() {
			for job := range jobq {
				err := f(job)
				if err != nil {
					log.Println(err)
					nerrors.Add(1)
				}
				wg.Done()
			}
		}()
	}
	wg.Wait()
	close(jobq)
	return nerrors.val
}

type syncCounter struct {
	val  int
	lock sync.Mutex
}

func (s *syncCounter) Add(n int) int {
	s.lock.Lock()
	s.val += n
	cur := s.val
	s.lock.Unlock()
	return cur
}
