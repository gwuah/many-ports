package bpfproxy

import (
	"sync/atomic"
)

type loadbalancer struct {
	targets []string
	next    int32
}

func (lb *loadbalancer) nextInLine() int {
	n := atomic.AddInt32(&lb.next, 1)
	return int(n-1) % len(lb.targets)
}

func (lb *loadbalancer) GetNextTartget() string {
	return lb.targets[lb.nextInLine()]
}
