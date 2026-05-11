package main

import (
	"errors"
	"fmt"
	stdrand "math/rand/v2"
	"runtime"
	"time"

	"github.com/gulyaev-iv/go-cloud-des/codegen/container"
	random "github.com/gulyaev-iv/go-cloud-des/codegen/rand"
)

var (
	eventQueue   *container.EventQueue = container.NewEventQueue(1000)
	clock        float64               = 0.0
	stop         bool                  = false
	rand         *random.Rand
	nextEntityID uint = 1

	pool1 = container.NewPool[Entity](10)

	create1    *create    = &create{exitN: 0}
	queue1     *queue     = &queue{}
	delay1     *delay     = &delay{enterN: 0, exitN: 0, busy: false}
	terminate1 *terminate = &terminate{enterN: 0}
)

const (
	totalEntities = 10_000_000

	arrivalRate = 1.0
	serviceRate = 1.25
)

func main() {
	runtime.GOMAXPROCS(1)

	rand = random.New(stdrand.NewPCG(stdrand.Uint64(), stdrand.Uint64()))

	Schedule(0.0, blockCreate_1)

	startTime := time.Now()

	for event := eventQueue.Pop(); event.Time != -1.0 && !stop; event = eventQueue.Pop() {
		clock = event.Time
		event.Handler()
		if terminate1.enterN >= totalEntities {
			stop = true
		}
	}
	endTime := time.Now()

	wallClockTime := float64(endTime.Sub(startTime).Nanoseconds()) / 1000 / 1000 / 1000
	eventPerSec := int(totalEntities / wallClockTime)

	fmt.Println("Готово")
	fmt.Printf("Реальное время (Wall-clock): %.4f секунд \n", wallClockTime)
	fmt.Printf("Пропускная способность: %v заявок/сек\n", eventPerSec)
	fmt.Printf("Модельное время: %.2f\n", clock)
}

func Schedule(time float64, handler func()) {
	eventQueue.Push(container.Event{Time: clock + time, Handler: handler})
}

type Entity struct {
	id uint
}

type create struct {
	exitN uint
}

func blockCreate_1() {
	Schedule(rand.Exponential(arrivalRate), func() {
		newEntity := pool1.Alloc()
		pool1.Entities[newEntity].id = nextEntityID
		nextEntityID++
		blockQueue_1(newEntity)
		create1.exitN++
		blockCreate_1()
	})
}

type queue struct {
	arr []uint32

	enterN uint
	exit1N uint
}

func (q *queue) Push(e uint32) {
	q.arr = append(q.arr, e)
}

func (q *queue) Top() uint32 {
	if len(q.arr) == 0 {
		return container.NullIdx
	}

	return q.arr[0]
}

func (q *queue) Delete() {
	q.arr = q.arr[1:]
}

func blockQueue_1(e uint32) {
	queue1.enterN++
	queue1.Push(e)

	ent := queue1.Top()

	if ent == container.NullIdx {
		stop = true
		return
	}

	if err := blockDelay_1(ent); err != nil {
		return
	}

	queue1.Delete()

	queue1.exit1N++
}

type delay struct {
	enterN uint
	exitN  uint
	busy   bool
}

func blockDelay_1(e uint32) error {
	if delay1.busy {
		return errors.New("")
	}

	delay1.enterN++
	delay1.busy = true
	Schedule(rand.Exponential(serviceRate), func() {
		blockTerminate_1(e)
		delay1.exitN++

		delay1.busy = false
		ent := queue1.Top()
		if ent == container.NullIdx {
			return
		}
		queue1.Delete()

		queue1.exit1N++

		blockDelay_1(ent)
	})
	return nil
}

type terminate struct {
	enterN uint
}

func blockTerminate_1(e uint32) {
	terminate1.enterN++
	pool1.Free(e)
}
