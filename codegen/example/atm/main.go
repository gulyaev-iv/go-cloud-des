package main

import (
	"errors"
	"fmt"
	stdrand "math/rand/v2"

	"github.com/gulyaev-iv/go-cloud-des/codegen/container"
	random "github.com/gulyaev-iv/go-cloud-des/codegen/rand"
)

var (
	eventQueue   *container.EventQueue = container.NewEventQueue(1000)
	clock        float64
	stop         bool
	rand         *random.Rand
	nextEntityID uint

	pool1 = container.NewPool[Entity](10)

	create1    *create
	queue1     *queue
	delay1     *delay
	terminate1 *terminate
	terminate2 *terminate
)

func main() {

	//eventQueue = heap.NewEventQueue(1000)
	clock = 0.0
	stop = false
	rand = random.New(stdrand.NewPCG(stdrand.Uint64(), stdrand.Uint64()))
	nextEntityID = 1

	//pool1 = heap.NewPool2[Entity](10)

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
	fmt.Printf("----------------CLOCK[%v]----------------\n", clock)
	fmt.Printf("Create: exit[%v]\n", create1.exitN)
	fmt.Printf("Queue: enter[%v], len[%v], exit1[%v], exit2[%v], exit3[%v]\n", queue1.enterN, queue1.len, queue1.exit1N, queue1.exit2N, queue1.exit3N)
	fmt.Printf("Delay: enter[%v], exit[%v]\n", delay1.enterN, delay1.exitN)
	fmt.Printf("Terminate1: enter[%v]\n", terminate1.enterN)
	fmt.Printf("Terminate2: enter[%v]\n", terminate2.enterN)
	fmt.Printf("Entities: cap[%v], len[%v]\n", cap(pool1.Entities), len(pool1.Entities))
	fmt.Printf("Freelist: cap[%v], len[%v]\n", cap(pool1.FreeList), len(pool1.FreeList))
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
	newEntity := pool1.Alloc()
	pool1.Entities[newEntity].id = nextEntityID
	nextEntityID++
	blockQueue_1(newEntity)
	create1.exitN++
	Schedule(rand.Uniform(30, 300), func() {
		blockCreate_1()
	})
}

type queueSlot struct {
	data  uint32
	valid bool
}

type queue struct {
	arr   []queueSlot
	len   int
	total int

	enterN uint
	exit1N uint
	exit2N uint
	exit3N uint
}

func (q *queue) Push(e uint32) int {
	if q.len == 10 {
		return -1
	}
	q.arr = append(q.arr, queueSlot{data: e, valid: true})
	q.len++
	q.total++
	return q.total - 1
}

func (q *queue) Top() uint32 {
	if q.len == 0 {
		return container.NullIdx
	}

	if !q.arr[0].valid {
		q.arr = q.arr[1:]
		return q.Top()
	}
	return q.arr[0].data
}

func (q *queue) Delete() {
	q.arr = q.arr[1:]
	q.len--
}

func (q *queue) Get(idx int) uint32 {
	del := q.total - len(q.arr)
	if idx < del {
		return container.NullIdx
	}
	slot := q.arr[idx-del]
	if !slot.valid {
		return container.NullIdx
	}
	q.arr[idx-del].valid = false
	q.len--
	return slot.data
}

func blockQueue_1(e uint32) {
	queue1.enterN++
	idx := queue1.Push(e)
	if idx == -1 {
		blockTerminate_2(e)
		queue1.exit1N++
		return
	}

	Schedule(rand.Triangular(3*60, 5*60, 15*60), func() {
		if ent := queue1.Get(idx); ent != container.NullIdx {
			queue1.exit2N++
			blockTerminate_2(e)
		}
	})

	ent := queue1.Top()

	if ent == container.NullIdx {
		stop = true
		return
	}
	if err := blockDelay_1(ent); err != nil {
		return
	}
	queue1.Delete()

	queue1.exit3N++
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
	Schedule(rand.Triangular(1*60, 2*60, 10*60), func() {
		blockTerminate_1(e)
		delay1.exitN++

		delay1.busy = false
		ent := queue1.Top()
		if ent == container.NullIdx {
			return
		}
		queue1.Delete()

		queue1.exit3N++

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

func blockTerminate_2(e uint32) {
	terminate2.enterN++
	pool1.Free(e)
}
