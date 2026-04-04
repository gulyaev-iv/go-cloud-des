package main

import (
	stdrand "math/rand/v2"
	"testing"

	"github.com/gulyaev-iv/go-cloud-des/codegen/container"
	random "github.com/gulyaev-iv/go-cloud-des/codegen/rand"
)

func BenchmarkModelTime(b *testing.B) {
	for b.Loop() {
		BenchFunc(b)
	}

}

func BenchFunc(b *testing.B) {
	eventQueue = container.NewEventQueue(1000)
	clock = 0.0
	stop = false
	rand = random.New(stdrand.NewPCG(stdrand.Uint64(), stdrand.Uint64()))
	nextEntityID = 1

	pool1 = container.NewPool[Entity](100)

	create1 = &create{exitN: 0}
	queue1 = &queue{}
	delay1 = &delay{enterN: 0, exitN: 0, busy: false}
	terminate1 = &terminate{enterN: 0}
	terminate2 = &terminate{enterN: 0}

	Schedule(0.0, blockCreate_1)
	Schedule(3_000_000, func() { stop = true })

	for event := eventQueue.Pop(); event.Time != -1.0 && !stop; event = eventQueue.Pop() {
		clock = event.Time
		event.Handler()

	}
}
