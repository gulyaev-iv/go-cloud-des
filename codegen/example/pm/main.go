package main

import (
	"fmt"
	stdrand "math/rand/v2"
	"runtime"
	"time"

	"github.com/gulyaev-iv/go-cloud-des/codegen/container"
	random "github.com/gulyaev-iv/go-cloud-des/codegen/rand"
)

var (
	eventQueue *container.EventQueue = container.NewEventQueue(1000)
	clock      float64               = 0.0

	stop       bool = false
	reasonStop string

	rand         *random.Rand
	nextEntityID uint = 1

	poolJobProto = container.NewPool[JobProto](10)

	create1 = &create{exitN: 0}

	queueA  = &queue{}
	queueB  = &queue{}
	queueQC = &queue{}

	delayA  = &delay{enterN: 0, exitN: 0, busy: false}
	delayB  = &delay{enterN: 0, exitN: 0, busy: false}
	delayQC = &delay{enterN: 0, exitN: 0, busy: false}

	terminateSink_Success = &terminate{enterN: 0}
	terminateSink_Scrap   = &terminate{enterN: 0}
)

const (
	totalEntities = 10_000_000

	arrivalRate   = 1.0
	serviceARate  = 1.0 / 1.2
	serviceBRate  = 1.0 / 1.8
	serviceQCRate = 1.0 / 0.5
)

func main() {
	runtime.GOMAXPROCS(1)
	rand = random.New(stdrand.NewPCG(stdrand.Uint64(), stdrand.Uint64()))

	Schedule(0.0, blockGenerateJobs)

	startTime := time.Now()

	for event := eventQueue.Pop(); event.Time != -1.0 && !stop; event = eventQueue.Pop() {
		clock = event.Time
		event.Handler()
		if terminateSink_Success.enterN+terminateSink_Scrap.enterN >= totalEntities {
			stop = true
			reasonStop = "Выполнено условие остановки"
		}
	}

	wallClockTime := time.Since(startTime).Seconds()

	fmt.Println("Симуляция остановлена. Причина: " + reasonStop)
	fmt.Printf("Реальное время (Wall-clock): %.4f секунд \n", wallClockTime)
	fmt.Printf("Модельное время: %.2f\n", clock)

	fmt.Printf("Sink_Success: %v\n", terminateSink_Success.enterN)
	fmt.Printf("Sink_Scrap: %v\n", terminateSink_Scrap.enterN)
}

func Schedule(time float64, handler func()) {
	eventQueue.Push(container.Event{Time: clock + time, Handler: handler})
}

type JobProto struct {
	id          uint
	JobType     int
	ReworkCount int
}

type create struct {
	exitN uint
}

func blockGenerateJobs() {
	Schedule(rand.Exponential(arrivalRate), func() {
		newEntity := poolJobProto.Alloc()
		poolJobProto.Entities[newEntity].id = nextEntityID
		nextEntityID++

		poolJobProto.Entities[newEntity].ReworkCount = 0
		if rand.Uniform(0.0, 1.0) < 0.6 {
			poolJobProto.Entities[newEntity].JobType = 1
		} else {
			poolJobProto.Entities[newEntity].JobType = 2
		}

		if blockBranch_ByType(newEntity) {
			create1.exitN++
			blockGenerateJobs()
		} else if stop {
			return
		} else {
			stop = true
			reasonStop = "GenerateJobs: заявка не смогла покинуть блок"
		}
	})
}

func blockBranch_ByType(e uint32) bool {
	switch poolJobProto.Entities[e].JobType {
	case 1:
		blockQueue_A(e)
	case 2:
		blockQueue_B(e)
	default:
		return false
	}
	return true
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

func blockQueue_A(e uint32) {
	queueA.enterN++
	queueA.Push(e)

	ent := queueA.Top()

	if ent == container.NullIdx {
		stop = true
		reasonStop = "Queue_A: необрабатываемая ошибка"
		return
	}

	if !blockDelay_A(ent) {
		return
	}

	queueA.Delete()

	queueA.exit1N++
}

func blockQueue_B(e uint32) {
	queueB.enterN++
	queueB.Push(e)

	ent := queueB.Top()

	if ent == container.NullIdx {
		stop = true
		reasonStop = "Queue_B: необрабатываемая ошибка"
		return
	}

	if !blockDelay_B(ent) {
		return
	}

	queueB.Delete()

	queueB.exit1N++
}

type delay struct {
	enterN uint
	exitN  uint
	busy   bool
}

func blockDelay_A(e uint32) bool {
	if delayA.busy {
		return false
	}

	delayA.enterN++
	delayA.busy = true
	Schedule(rand.Exponential(serviceARate), func() {
		blockQueue_QC(e)
		delayA.exitN++

		delayA.busy = false
		ent := queueA.Top()
		if ent == container.NullIdx {
			return
		}
		queueA.Delete()

		queueA.exit1N++

		blockDelay_A(ent)
	})
	return true
}

func blockDelay_B(e uint32) bool {
	if delayB.busy {
		return false
	}

	delayB.enterN++
	delayB.busy = true
	Schedule(rand.Exponential(serviceBRate), func() {
		blockQueue_QC(e)
		delayB.exitN++

		delayB.busy = false
		ent := queueB.Top()
		if ent == container.NullIdx {
			return
		}
		queueB.Delete()

		queueB.exit1N++

		blockDelay_B(ent)
	})
	return true
}

func blockQueue_QC(e uint32) {
	queueQC.enterN++
	queueQC.Push(e)

	ent := queueQC.Top()

	if ent == container.NullIdx {
		stop = true
		reasonStop = "Queue_QC: необрабатываемая ошибка"
		return
	}

	if !blockDelay_QC(ent) {
		return
	}

	queueQC.Delete()

	queueQC.exit1N++
}

func blockDelay_QC(e uint32) bool {
	if delayQC.busy {
		return false
	}

	delayQC.enterN++
	delayQC.busy = true
	Schedule(rand.Exponential(serviceQCRate), func() {
		blockBranch_QCResult(e)
		delayQC.exitN++

		delayQC.busy = false
		ent := queueQC.Top()
		if ent == container.NullIdx {
			return
		}
		queueQC.Delete()

		queueQC.exit1N++

		blockDelay_QC(ent)
	})
	return true
}

func blockBranch_QCResult(e uint32) {
	if rand.Uniform(0.0, 1.0) < 0.9 {
		blockSink_Success(e)
	} else {
		blockBranch_ReworkDecision(e)
	}
}

func blockBranch_ReworkDecision(e uint32) {
	poolJobProto.Entities[e].ReworkCount++

	if poolJobProto.Entities[e].ReworkCount <= 3 {
		blockBranch_ByType(e)
	} else {
		blockSink_Scrap(e)
	}
}

type terminate struct {
	enterN uint
}

func blockSink_Success(e uint32) {
	terminateSink_Success.enterN++
	poolJobProto.Free(e)
}

func blockSink_Scrap(e uint32) {
	terminateSink_Scrap.enterN++
	poolJobProto.Free(e)
}
