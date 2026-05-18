package main

import (
	"bufio"
	"fmt"
	stdrand "math/rand/v2"
	"os"
	"strconv"
	"strings"

	"github.com/gulyaev-iv/go-cloud-des/codegen/container"
	random "github.com/gulyaev-iv/go-cloud-des/codegen/rand"
)

const modelHash = "6f155949352b29d96564d08110c12682626a309f0058d80404a035f1b42f8b5d"

const nullEntityType uint8 = 0

type EntityRef struct {
	Type uint8
	Idx  uint32
}

type queueSlot struct {
	ref   EntityRef
	valid bool
}

var (
	eventQueue *container.EventQueue
	clock      float64 = 0.0
	stop       bool    = false
	reasonStop string
	rng        *random.Rand
)

type metricFunc struct {
	name  string
	value func() string
}

var (
	protoOut      *bufio.Writer
	metrics       []metricFunc
	metricStep    float64
	hasMetricStep bool
	nextMetricAt  float64
	stopRule      func() bool
)

const entityType_Jobs uint8 = 1

type entity_Jobs struct {
	field_JobType     int64
	field_ReworkCount int64
}

var pool_entity_Jobs *container.Pool[entity_Jobs]

var var_totalEntitiesEnd uint64 = 10000000
var var_totalEntitiesSink uint64 = 0
var var_arrivalRate float64 = 1.0
var var_serviceARate float64 = 0.83333333
var var_serviceBRate float64 = 0.55555556
var var_serviceQCRate float64 = 2.0

type struct_block_CREATE_Source struct {
	createdN uint64
	exitN    uint64
	dropN    uint64
}

var block_CREATE_Source *struct_block_CREATE_Source

type struct_block_BRANCH_BranchJobType struct {
	enterN uint64
	exitN  uint64
}

var block_BRANCH_BranchJobType *struct_block_BRANCH_BranchJobType

type struct_block_ASSIGN_AssignJobTypeOne struct {
	enterN uint64
	exitN  uint64
}

var block_ASSIGN_AssignJobTypeOne *struct_block_ASSIGN_AssignJobTypeOne

type struct_block_ASSIGN_AssignJobTypeTwo struct {
	enterN uint64
	exitN  uint64
}

var block_ASSIGN_AssignJobTypeTwo *struct_block_ASSIGN_AssignJobTypeTwo

type struct_block_BRANCH_BranchByType struct {
	enterN uint64
	exitN  uint64
}

var block_BRANCH_BranchByType *struct_block_BRANCH_BranchByType

type struct_block_QUEUE_QueueA struct {
	arr      []queueSlot
	total    uint64
	enterN   uint64
	exitN    uint64
	dropN    uint64
	timeoutN uint64
	len      uint64
	maxLen   uint64
}

var block_QUEUE_QueueA *struct_block_QUEUE_QueueA

type struct_block_DELAY_DelayA struct {
	enterN  uint64
	exitN   uint64
	busy    uint64
	maxBusy uint64
}

var block_DELAY_DelayA *struct_block_DELAY_DelayA

type struct_block_QUEUE_QueueB struct {
	arr      []queueSlot
	total    uint64
	enterN   uint64
	exitN    uint64
	dropN    uint64
	timeoutN uint64
	len      uint64
	maxLen   uint64
}

var block_QUEUE_QueueB *struct_block_QUEUE_QueueB

type struct_block_DELAY_DelayB struct {
	enterN  uint64
	exitN   uint64
	busy    uint64
	maxBusy uint64
}

var block_DELAY_DelayB *struct_block_DELAY_DelayB

type struct_block_QUEUE_QueueQC struct {
	arr      []queueSlot
	total    uint64
	enterN   uint64
	exitN    uint64
	dropN    uint64
	timeoutN uint64
	len      uint64
	maxLen   uint64
}

var block_QUEUE_QueueQC *struct_block_QUEUE_QueueQC

type struct_block_DELAY_DelayQC struct {
	enterN  uint64
	exitN   uint64
	busy    uint64
	maxBusy uint64
}

var block_DELAY_DelayQC *struct_block_DELAY_DelayQC

type struct_block_BRANCH_QCResult struct {
	enterN uint64
	exitN  uint64
}

var block_BRANCH_QCResult *struct_block_BRANCH_QCResult

type struct_block_ASSIGN_SinkSuccessCount struct {
	enterN uint64
	exitN  uint64
}

var block_ASSIGN_SinkSuccessCount *struct_block_ASSIGN_SinkSuccessCount

type struct_block_TERMINATE_SinkSuccess struct {
	enterN uint64
}

var block_TERMINATE_SinkSuccess *struct_block_TERMINATE_SinkSuccess

type struct_block_ASSIGN_ReworkCountUp struct {
	enterN uint64
	exitN  uint64
}

var block_ASSIGN_ReworkCountUp *struct_block_ASSIGN_ReworkCountUp

type struct_block_BRANCH_ReworkDecision struct {
	enterN uint64
	exitN  uint64
}

var block_BRANCH_ReworkDecision *struct_block_BRANCH_ReworkDecision

type struct_block_ASSIGN_SinkScrapCount struct {
	enterN uint64
	exitN  uint64
}

var block_ASSIGN_SinkScrapCount *struct_block_ASSIGN_SinkScrapCount

type struct_block_TERMINATE_SinkScrap struct {
	enterN uint64
}

var block_TERMINATE_SinkScrap *struct_block_TERMINATE_SinkScrap

func initModel() {
	eventQueue = container.NewEventQueue(1000)
	clock = 0.0
	stop = false
	reasonStop = ""
	rng = random.New(stdrand.NewPCG(stdrand.Uint64(), stdrand.Uint64()))
	pool_entity_Jobs = container.NewPool[entity_Jobs](10)
	block_CREATE_Source = &struct_block_CREATE_Source{}
	block_BRANCH_BranchJobType = &struct_block_BRANCH_BranchJobType{}
	block_ASSIGN_AssignJobTypeOne = &struct_block_ASSIGN_AssignJobTypeOne{}
	block_ASSIGN_AssignJobTypeTwo = &struct_block_ASSIGN_AssignJobTypeTwo{}
	block_BRANCH_BranchByType = &struct_block_BRANCH_BranchByType{}
	block_QUEUE_QueueA = &struct_block_QUEUE_QueueA{}
	block_DELAY_DelayA = &struct_block_DELAY_DelayA{}
	block_QUEUE_QueueB = &struct_block_QUEUE_QueueB{}
	block_DELAY_DelayB = &struct_block_DELAY_DelayB{}
	block_QUEUE_QueueQC = &struct_block_QUEUE_QueueQC{}
	block_DELAY_DelayQC = &struct_block_DELAY_DelayQC{}
	block_BRANCH_QCResult = &struct_block_BRANCH_QCResult{}
	block_ASSIGN_SinkSuccessCount = &struct_block_ASSIGN_SinkSuccessCount{}
	block_TERMINATE_SinkSuccess = &struct_block_TERMINATE_SinkSuccess{}
	block_ASSIGN_ReworkCountUp = &struct_block_ASSIGN_ReworkCountUp{}
	block_BRANCH_ReworkDecision = &struct_block_BRANCH_ReworkDecision{}
	block_ASSIGN_SinkScrapCount = &struct_block_ASSIGN_SinkScrapCount{}
	block_TERMINATE_SinkScrap = &struct_block_TERMINATE_SinkScrap{}
}

func Schedule(time float64, handler func()) {
	eventQueue.Push(container.Event{Time: clock + time, Handler: handler})
}

func freeEntity(ref EntityRef) {
	switch ref.Type {
	case entityType_Jobs:
		pool_entity_Jobs.Free(ref.Idx)
	default:
		stop = true
		reasonStop = "unknown entity type"
	}
}

func func_block_CREATE_Source() {
	Schedule(float64(rng.Exponential(var_arrivalRate)), func() {
		batch := uint64(1)
		for i := uint64(0); i < batch; i++ {
			newEntity := pool_entity_Jobs.Alloc()
			pool_entity_Jobs.Entities[newEntity].field_JobType = 0
			pool_entity_Jobs.Entities[newEntity].field_ReworkCount = 0
			ref := EntityRef{Type: entityType_Jobs, Idx: newEntity}
			block_CREATE_Source.createdN++
			if func_block_BRANCH_BranchJobType(ref) {
				block_CREATE_Source.exitN++
			} else if stop {
				return
			} else {
				block_CREATE_Source.dropN++
				freeEntity(ref)
			}
		}
		if !stop {
			func_block_CREATE_Source()
		}
	})
}

func func_block_BRANCH_BranchJobType(ref EntityRef) bool {
	if rng.Uniform(0.0, 1.0) < 0.6 {
		if func_block_ASSIGN_AssignJobTypeOne(ref) {
			block_BRANCH_BranchJobType.enterN++
			block_BRANCH_BranchJobType.exitN++
			return true
		}
		return false
	} else {
		if func_block_ASSIGN_AssignJobTypeTwo(ref) {
			block_BRANCH_BranchJobType.enterN++
			block_BRANCH_BranchJobType.exitN++
			return true
		}
		return false
	}
}

func func_block_ASSIGN_AssignJobTypeOne(ref EntityRef) bool {
	var assignOld0 int64
	assignChanged0 := false
	if ref.Type == entityType_Jobs {
		assignOld0 = pool_entity_Jobs.Entities[ref.Idx].field_JobType
		assignChanged0 = true
		pool_entity_Jobs.Entities[ref.Idx].field_JobType = 1
	}
	if func_block_BRANCH_BranchByType(ref) {
		block_ASSIGN_AssignJobTypeOne.enterN++
		block_ASSIGN_AssignJobTypeOne.exitN++
		return true
	}
	if assignChanged0 {
		pool_entity_Jobs.Entities[ref.Idx].field_JobType = assignOld0
	}
	return false
}

func func_block_ASSIGN_AssignJobTypeTwo(ref EntityRef) bool {
	var assignOld0 int64
	assignChanged0 := false
	if ref.Type == entityType_Jobs {
		assignOld0 = pool_entity_Jobs.Entities[ref.Idx].field_JobType
		assignChanged0 = true
		pool_entity_Jobs.Entities[ref.Idx].field_JobType = 2
	}
	if func_block_BRANCH_BranchByType(ref) {
		block_ASSIGN_AssignJobTypeTwo.enterN++
		block_ASSIGN_AssignJobTypeTwo.exitN++
		return true
	}
	if assignChanged0 {
		pool_entity_Jobs.Entities[ref.Idx].field_JobType = assignOld0
	}
	return false
}

func func_block_BRANCH_BranchByType(ref EntityRef) bool {
	if ref.Type == entityType_Jobs && (pool_entity_Jobs.Entities[ref.Idx].field_JobType == 1) {
		if func_block_QUEUE_QueueA(ref) {
			block_BRANCH_BranchByType.enterN++
			block_BRANCH_BranchByType.exitN++
			return true
		}
		return false
	} else if ref.Type == entityType_Jobs && (pool_entity_Jobs.Entities[ref.Idx].field_JobType == 2) {
		if func_block_QUEUE_QueueB(ref) {
			block_BRANCH_BranchByType.enterN++
			block_BRANCH_BranchByType.exitN++
			return true
		}
		return false
	} else {
		stop = true
		reasonStop = "BranchByType: no branch condition matched"
		return false
	}
}

func func_queue_push_QueueA(ref EntityRef) uint64 {
	idx := block_QUEUE_QueueA.total
	block_QUEUE_QueueA.total++
	block_QUEUE_QueueA.arr = append(block_QUEUE_QueueA.arr, queueSlot{ref: ref, valid: true})
	block_QUEUE_QueueA.len++
	if block_QUEUE_QueueA.len > block_QUEUE_QueueA.maxLen {
		block_QUEUE_QueueA.maxLen = block_QUEUE_QueueA.len
	}
	return idx
}

func func_queue_top_QueueA() (EntityRef, bool) {
	for len(block_QUEUE_QueueA.arr) > 0 && !block_QUEUE_QueueA.arr[0].valid {
		block_QUEUE_QueueA.arr = block_QUEUE_QueueA.arr[1:]
	}
	if block_QUEUE_QueueA.len == 0 || len(block_QUEUE_QueueA.arr) == 0 {
		return EntityRef{Type: nullEntityType, Idx: container.NullIdx}, false
	}
	return block_QUEUE_QueueA.arr[0].ref, true
}

func func_queue_delete_QueueA() {
	for len(block_QUEUE_QueueA.arr) > 0 && !block_QUEUE_QueueA.arr[0].valid {
		block_QUEUE_QueueA.arr = block_QUEUE_QueueA.arr[1:]
	}
	if len(block_QUEUE_QueueA.arr) == 0 {
		return
	}
	block_QUEUE_QueueA.arr = block_QUEUE_QueueA.arr[1:]
	if block_QUEUE_QueueA.len > 0 {
		block_QUEUE_QueueA.len--
	}
}

func func_queue_get_QueueA(idx uint64) (EntityRef, bool) {
	deleted := block_QUEUE_QueueA.total - uint64(len(block_QUEUE_QueueA.arr))
	if idx < deleted {
		return EntityRef{Type: nullEntityType, Idx: container.NullIdx}, false
	}
	pos := idx - deleted
	if pos >= uint64(len(block_QUEUE_QueueA.arr)) {
		return EntityRef{Type: nullEntityType, Idx: container.NullIdx}, false
	}
	slot := &block_QUEUE_QueueA.arr[pos]
	if !slot.valid {
		return EntityRef{Type: nullEntityType, Idx: container.NullIdx}, false
	}
	slot.valid = false
	if block_QUEUE_QueueA.len > 0 {
		block_QUEUE_QueueA.len--
	}
	return slot.ref, true
}

func func_queue_try_advance_QueueA() bool {
	ref, ok := func_queue_top_QueueA()
	if !ok {
		return false
	}
	if !func_block_DELAY_DelayA(ref) {
		return false
	}
	func_queue_delete_QueueA()
	block_QUEUE_QueueA.exitN++
	return true
}

func func_block_QUEUE_QueueA(ref EntityRef) bool {
	block_QUEUE_QueueA.enterN++
	func_queue_push_QueueA(ref)
	func_queue_try_advance_QueueA()
	return true
}

func func_block_DELAY_DelayA(ref EntityRef) bool {
	capacity := uint64(1)
	if block_DELAY_DelayA.busy >= capacity {
		return false
	}
	duration := float64(rng.Exponential(var_serviceARate))
	block_DELAY_DelayA.enterN++
	block_DELAY_DelayA.busy++
	if block_DELAY_DelayA.busy > block_DELAY_DelayA.maxBusy {
		block_DELAY_DelayA.maxBusy = block_DELAY_DelayA.busy
	}
	Schedule(duration, func() {
		if func_block_QUEUE_QueueQC(ref) {
			block_DELAY_DelayA.exitN++
		} else if stop {
			return
		} else {
			stop = true
			reasonStop = "DelayA: заявка не смогла покинуть блок"
			return
		}
		if block_DELAY_DelayA.busy > 0 {
			block_DELAY_DelayA.busy--
		}
		if func_queue_try_advance_QueueA() {
			return
		}
	})
	return true
}

func func_queue_push_QueueB(ref EntityRef) uint64 {
	idx := block_QUEUE_QueueB.total
	block_QUEUE_QueueB.total++
	block_QUEUE_QueueB.arr = append(block_QUEUE_QueueB.arr, queueSlot{ref: ref, valid: true})
	block_QUEUE_QueueB.len++
	if block_QUEUE_QueueB.len > block_QUEUE_QueueB.maxLen {
		block_QUEUE_QueueB.maxLen = block_QUEUE_QueueB.len
	}
	return idx
}

func func_queue_top_QueueB() (EntityRef, bool) {
	for len(block_QUEUE_QueueB.arr) > 0 && !block_QUEUE_QueueB.arr[0].valid {
		block_QUEUE_QueueB.arr = block_QUEUE_QueueB.arr[1:]
	}
	if block_QUEUE_QueueB.len == 0 || len(block_QUEUE_QueueB.arr) == 0 {
		return EntityRef{Type: nullEntityType, Idx: container.NullIdx}, false
	}
	return block_QUEUE_QueueB.arr[0].ref, true
}

func func_queue_delete_QueueB() {
	for len(block_QUEUE_QueueB.arr) > 0 && !block_QUEUE_QueueB.arr[0].valid {
		block_QUEUE_QueueB.arr = block_QUEUE_QueueB.arr[1:]
	}
	if len(block_QUEUE_QueueB.arr) == 0 {
		return
	}
	block_QUEUE_QueueB.arr = block_QUEUE_QueueB.arr[1:]
	if block_QUEUE_QueueB.len > 0 {
		block_QUEUE_QueueB.len--
	}
}

func func_queue_get_QueueB(idx uint64) (EntityRef, bool) {
	deleted := block_QUEUE_QueueB.total - uint64(len(block_QUEUE_QueueB.arr))
	if idx < deleted {
		return EntityRef{Type: nullEntityType, Idx: container.NullIdx}, false
	}
	pos := idx - deleted
	if pos >= uint64(len(block_QUEUE_QueueB.arr)) {
		return EntityRef{Type: nullEntityType, Idx: container.NullIdx}, false
	}
	slot := &block_QUEUE_QueueB.arr[pos]
	if !slot.valid {
		return EntityRef{Type: nullEntityType, Idx: container.NullIdx}, false
	}
	slot.valid = false
	if block_QUEUE_QueueB.len > 0 {
		block_QUEUE_QueueB.len--
	}
	return slot.ref, true
}

func func_queue_try_advance_QueueB() bool {
	ref, ok := func_queue_top_QueueB()
	if !ok {
		return false
	}
	if !func_block_DELAY_DelayB(ref) {
		return false
	}
	func_queue_delete_QueueB()
	block_QUEUE_QueueB.exitN++
	return true
}

func func_block_QUEUE_QueueB(ref EntityRef) bool {
	block_QUEUE_QueueB.enterN++
	func_queue_push_QueueB(ref)
	func_queue_try_advance_QueueB()
	return true
}

func func_block_DELAY_DelayB(ref EntityRef) bool {
	capacity := uint64(1)
	if block_DELAY_DelayB.busy >= capacity {
		return false
	}
	duration := float64(rng.Exponential(var_serviceBRate))
	block_DELAY_DelayB.enterN++
	block_DELAY_DelayB.busy++
	if block_DELAY_DelayB.busy > block_DELAY_DelayB.maxBusy {
		block_DELAY_DelayB.maxBusy = block_DELAY_DelayB.busy
	}
	Schedule(duration, func() {
		if func_block_QUEUE_QueueQC(ref) {
			block_DELAY_DelayB.exitN++
		} else if stop {
			return
		} else {
			stop = true
			reasonStop = "DelayB: заявка не смогла покинуть блок"
			return
		}
		if block_DELAY_DelayB.busy > 0 {
			block_DELAY_DelayB.busy--
		}
		if func_queue_try_advance_QueueB() {
			return
		}
	})
	return true
}

func func_queue_push_QueueQC(ref EntityRef) uint64 {
	idx := block_QUEUE_QueueQC.total
	block_QUEUE_QueueQC.total++
	block_QUEUE_QueueQC.arr = append(block_QUEUE_QueueQC.arr, queueSlot{ref: ref, valid: true})
	block_QUEUE_QueueQC.len++
	if block_QUEUE_QueueQC.len > block_QUEUE_QueueQC.maxLen {
		block_QUEUE_QueueQC.maxLen = block_QUEUE_QueueQC.len
	}
	return idx
}

func func_queue_top_QueueQC() (EntityRef, bool) {
	for len(block_QUEUE_QueueQC.arr) > 0 && !block_QUEUE_QueueQC.arr[0].valid {
		block_QUEUE_QueueQC.arr = block_QUEUE_QueueQC.arr[1:]
	}
	if block_QUEUE_QueueQC.len == 0 || len(block_QUEUE_QueueQC.arr) == 0 {
		return EntityRef{Type: nullEntityType, Idx: container.NullIdx}, false
	}
	return block_QUEUE_QueueQC.arr[0].ref, true
}

func func_queue_delete_QueueQC() {
	for len(block_QUEUE_QueueQC.arr) > 0 && !block_QUEUE_QueueQC.arr[0].valid {
		block_QUEUE_QueueQC.arr = block_QUEUE_QueueQC.arr[1:]
	}
	if len(block_QUEUE_QueueQC.arr) == 0 {
		return
	}
	block_QUEUE_QueueQC.arr = block_QUEUE_QueueQC.arr[1:]
	if block_QUEUE_QueueQC.len > 0 {
		block_QUEUE_QueueQC.len--
	}
}

func func_queue_get_QueueQC(idx uint64) (EntityRef, bool) {
	deleted := block_QUEUE_QueueQC.total - uint64(len(block_QUEUE_QueueQC.arr))
	if idx < deleted {
		return EntityRef{Type: nullEntityType, Idx: container.NullIdx}, false
	}
	pos := idx - deleted
	if pos >= uint64(len(block_QUEUE_QueueQC.arr)) {
		return EntityRef{Type: nullEntityType, Idx: container.NullIdx}, false
	}
	slot := &block_QUEUE_QueueQC.arr[pos]
	if !slot.valid {
		return EntityRef{Type: nullEntityType, Idx: container.NullIdx}, false
	}
	slot.valid = false
	if block_QUEUE_QueueQC.len > 0 {
		block_QUEUE_QueueQC.len--
	}
	return slot.ref, true
}

func func_queue_try_advance_QueueQC() bool {
	ref, ok := func_queue_top_QueueQC()
	if !ok {
		return false
	}
	if !func_block_DELAY_DelayQC(ref) {
		return false
	}
	func_queue_delete_QueueQC()
	block_QUEUE_QueueQC.exitN++
	return true
}

func func_block_QUEUE_QueueQC(ref EntityRef) bool {
	block_QUEUE_QueueQC.enterN++
	func_queue_push_QueueQC(ref)
	func_queue_try_advance_QueueQC()
	return true
}

func func_block_DELAY_DelayQC(ref EntityRef) bool {
	capacity := uint64(1)
	if block_DELAY_DelayQC.busy >= capacity {
		return false
	}
	duration := float64(rng.Exponential(var_serviceQCRate))
	block_DELAY_DelayQC.enterN++
	block_DELAY_DelayQC.busy++
	if block_DELAY_DelayQC.busy > block_DELAY_DelayQC.maxBusy {
		block_DELAY_DelayQC.maxBusy = block_DELAY_DelayQC.busy
	}
	Schedule(duration, func() {
		if func_block_BRANCH_QCResult(ref) {
			block_DELAY_DelayQC.exitN++
		} else if stop {
			return
		} else {
			stop = true
			reasonStop = "DelayQC: заявка не смогла покинуть блок"
			return
		}
		if block_DELAY_DelayQC.busy > 0 {
			block_DELAY_DelayQC.busy--
		}
		if func_queue_try_advance_QueueQC() {
			return
		}
	})
	return true
}

func func_block_BRANCH_QCResult(ref EntityRef) bool {
	if rng.Uniform(0.0, 1.0) < 0.9 {
		if func_block_ASSIGN_SinkSuccessCount(ref) {
			block_BRANCH_QCResult.enterN++
			block_BRANCH_QCResult.exitN++
			return true
		}
		return false
	} else {
		if func_block_ASSIGN_ReworkCountUp(ref) {
			block_BRANCH_QCResult.enterN++
			block_BRANCH_QCResult.exitN++
			return true
		}
		return false
	}
}

func func_block_ASSIGN_SinkSuccessCount(ref EntityRef) bool {
	var assignOld0 uint64
	assignChanged0 := false
	assignOld0 = var_totalEntitiesSink
	assignChanged0 = true
	var_totalEntitiesSink = var_totalEntitiesSink + 1
	if func_block_TERMINATE_SinkSuccess(ref) {
		block_ASSIGN_SinkSuccessCount.enterN++
		block_ASSIGN_SinkSuccessCount.exitN++
		return true
	}
	if assignChanged0 {
		var_totalEntitiesSink = assignOld0
	}
	return false
}

func func_block_TERMINATE_SinkSuccess(ref EntityRef) bool {
	block_TERMINATE_SinkSuccess.enterN++
	freeEntity(ref)
	return true
}

func func_block_ASSIGN_ReworkCountUp(ref EntityRef) bool {
	var assignOld0 int64
	assignChanged0 := false
	if ref.Type == entityType_Jobs {
		assignOld0 = pool_entity_Jobs.Entities[ref.Idx].field_ReworkCount
		assignChanged0 = true
		pool_entity_Jobs.Entities[ref.Idx].field_ReworkCount = pool_entity_Jobs.Entities[ref.Idx].field_ReworkCount + 1
	}
	if func_block_BRANCH_ReworkDecision(ref) {
		block_ASSIGN_ReworkCountUp.enterN++
		block_ASSIGN_ReworkCountUp.exitN++
		return true
	}
	if assignChanged0 {
		pool_entity_Jobs.Entities[ref.Idx].field_ReworkCount = assignOld0
	}
	return false
}

func func_block_BRANCH_ReworkDecision(ref EntityRef) bool {
	if ref.Type == entityType_Jobs && (pool_entity_Jobs.Entities[ref.Idx].field_ReworkCount <= 3) {
		if func_block_BRANCH_BranchByType(ref) {
			block_BRANCH_ReworkDecision.enterN++
			block_BRANCH_ReworkDecision.exitN++
			return true
		}
		return false
	} else {
		if func_block_ASSIGN_SinkScrapCount(ref) {
			block_BRANCH_ReworkDecision.enterN++
			block_BRANCH_ReworkDecision.exitN++
			return true
		}
		return false
	}
}

func func_block_ASSIGN_SinkScrapCount(ref EntityRef) bool {
	var assignOld0 uint64
	assignChanged0 := false
	assignOld0 = var_totalEntitiesSink
	assignChanged0 = true
	var_totalEntitiesSink = var_totalEntitiesSink + 1
	if func_block_TERMINATE_SinkScrap(ref) {
		block_ASSIGN_SinkScrapCount.enterN++
		block_ASSIGN_SinkScrapCount.exitN++
		return true
	}
	if assignChanged0 {
		var_totalEntitiesSink = assignOld0
	}
	return false
}

func func_block_TERMINATE_SinkScrap(ref EntityRef) bool {
	block_TERMINATE_SinkScrap.enterN++
	freeEntity(ref)
	return true
}

func sendLine(format string, args ...any) {
	if protoOut == nil {
		return
	}
	_, _ = fmt.Fprintf(protoOut, format+"\n", args...)
	_ = protoOut.Flush()
}

func sendErr(code string, message string) {
	message = strings.ReplaceAll(message, "\n", " ")
	sendLine("ERR %s %s", code, message)
}

func configureExperiment() bool {
	protoOut = bufio.NewWriter(os.Stdout)
	scanner := bufio.NewScanner(os.Stdin)
	sendLine("HELLO %s", modelHash)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if handleCommand(line) {
			return true
		}
	}
	if err := scanner.Err(); err != nil {
		sendErr("read_error", err.Error())
	}
	return false
}

func handleCommand(line string) bool {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return false
	}
	switch fields[0] {
	case "RULE":
		if len(fields) != 5 || fields[1] != "STOP" {
			sendErr("bad_rule", "expected RULE STOP <left> <op> <right>")
			return false
		}
		if !configureStopRule(fields[2], fields[3], fields[4]) {
			return false
		}
	case "METRIC":
		raw := strings.TrimSpace(strings.TrimPrefix(line, "METRIC"))
		if raw == "" {
			metrics = nil
			return false
		}
		parts := strings.Split(raw, ";")
		nextMetrics := make([]metricFunc, 0, len(parts))
		for _, part := range parts {
			name := strings.TrimSpace(part)
			if name == "" {
				continue
			}
			metric, ok := resolveMetric(name)
			if !ok {
				sendErr("unknown_metric", name)
				return false
			}
			nextMetrics = append(nextMetrics, metric)
		}
		metrics = nextMetrics
	case "METRIC_STEP":
		if len(fields) != 2 {
			sendErr("bad_metric_step", "expected METRIC_STEP <model_time_step>")
			return false
		}
		value, err := strconv.ParseFloat(fields[1], 64)
		if err != nil || value <= 0 {
			sendErr("bad_metric_step", fields[1])
			return false
		}
		metricStep = value
		hasMetricStep = true
		nextMetricAt = metricStep
	case "START":
		if len(fields) != 1 {
			sendErr("bad_start", "START does not accept arguments")
			return false
		}
		sendLine("READY")
		return true
	case "SET":
		if len(fields) == 3 {
			if !setVar(fields[1], fields[2]) {
				return false
			}
		} else if len(fields) == 4 && fields[2] == "=" {
			if !setVar(fields[1], fields[3]) {
				return false
			}
		} else {
			sendErr("bad_set", "expected SET <var_name> <value> or SET <var_name> = <value>")
			return false
		}
	default:
		sendErr("unknown_command", fields[0])
	}
	return false
}

func setVar(name string, raw string) bool {
	switch name {
	case "totalEntitiesEnd":
		value, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			sendErr("bad_set_value", name+"="+raw)
			return false
		}
		var_totalEntitiesEnd = value
	case "totalEntitiesSink":
		value, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			sendErr("bad_set_value", name+"="+raw)
			return false
		}
		var_totalEntitiesSink = value
	case "arrivalRate":
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			sendErr("bad_set_value", name+"="+raw)
			return false
		}
		var_arrivalRate = value
	case "serviceARate":
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			sendErr("bad_set_value", name+"="+raw)
			return false
		}
		var_serviceARate = value
	case "serviceBRate":
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			sendErr("bad_set_value", name+"="+raw)
			return false
		}
		var_serviceBRate = value
	case "serviceQCRate":
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			sendErr("bad_set_value", name+"="+raw)
			return false
		}
		var_serviceQCRate = value
	default:
		sendErr("unknown_set_target", name)
		return false
	}
	return true
}

func resolveMetric(name string) (metricFunc, bool) {
	switch name {
	case "Simulation.clock":
		return metricFunc{name: name, value: func() string { return strconv.FormatFloat(clock, 'f', -1, 64) }}, true
	case "totalEntitiesEnd":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(var_totalEntitiesEnd, 10) }}, true
	case "totalEntitiesSink":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(var_totalEntitiesSink, 10) }}, true
	case "arrivalRate":
		return metricFunc{name: name, value: func() string { return strconv.FormatFloat(var_arrivalRate, 'f', -1, 64) }}, true
	case "serviceARate":
		return metricFunc{name: name, value: func() string { return strconv.FormatFloat(var_serviceARate, 'f', -1, 64) }}, true
	case "serviceBRate":
		return metricFunc{name: name, value: func() string { return strconv.FormatFloat(var_serviceBRate, 'f', -1, 64) }}, true
	case "serviceQCRate":
		return metricFunc{name: name, value: func() string { return strconv.FormatFloat(var_serviceQCRate, 'f', -1, 64) }}, true
	case "Source.createdN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_CREATE_Source.createdN, 10) }}, true
	case "Source.exitN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_CREATE_Source.exitN, 10) }}, true
	case "Source.dropN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_CREATE_Source.dropN, 10) }}, true
	case "BranchJobType.enterN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_BRANCH_BranchJobType.enterN, 10) }}, true
	case "BranchJobType.exitN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_BRANCH_BranchJobType.exitN, 10) }}, true
	case "AssignJobTypeOne.enterN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_ASSIGN_AssignJobTypeOne.enterN, 10) }}, true
	case "AssignJobTypeOne.exitN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_ASSIGN_AssignJobTypeOne.exitN, 10) }}, true
	case "AssignJobTypeTwo.enterN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_ASSIGN_AssignJobTypeTwo.enterN, 10) }}, true
	case "AssignJobTypeTwo.exitN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_ASSIGN_AssignJobTypeTwo.exitN, 10) }}, true
	case "BranchByType.enterN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_BRANCH_BranchByType.enterN, 10) }}, true
	case "BranchByType.exitN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_BRANCH_BranchByType.exitN, 10) }}, true
	case "QueueA.enterN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_QUEUE_QueueA.enterN, 10) }}, true
	case "QueueA.exitN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_QUEUE_QueueA.exitN, 10) }}, true
	case "QueueA.dropN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_QUEUE_QueueA.dropN, 10) }}, true
	case "QueueA.timeoutN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_QUEUE_QueueA.timeoutN, 10) }}, true
	case "QueueA.len":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_QUEUE_QueueA.len, 10) }}, true
	case "QueueA.maxLen":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_QUEUE_QueueA.maxLen, 10) }}, true
	case "DelayA.enterN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_DELAY_DelayA.enterN, 10) }}, true
	case "DelayA.exitN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_DELAY_DelayA.exitN, 10) }}, true
	case "DelayA.busy":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_DELAY_DelayA.busy, 10) }}, true
	case "DelayA.maxBusy":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_DELAY_DelayA.maxBusy, 10) }}, true
	case "QueueB.enterN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_QUEUE_QueueB.enterN, 10) }}, true
	case "QueueB.exitN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_QUEUE_QueueB.exitN, 10) }}, true
	case "QueueB.dropN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_QUEUE_QueueB.dropN, 10) }}, true
	case "QueueB.timeoutN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_QUEUE_QueueB.timeoutN, 10) }}, true
	case "QueueB.len":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_QUEUE_QueueB.len, 10) }}, true
	case "QueueB.maxLen":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_QUEUE_QueueB.maxLen, 10) }}, true
	case "DelayB.enterN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_DELAY_DelayB.enterN, 10) }}, true
	case "DelayB.exitN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_DELAY_DelayB.exitN, 10) }}, true
	case "DelayB.busy":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_DELAY_DelayB.busy, 10) }}, true
	case "DelayB.maxBusy":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_DELAY_DelayB.maxBusy, 10) }}, true
	case "QueueQC.enterN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_QUEUE_QueueQC.enterN, 10) }}, true
	case "QueueQC.exitN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_QUEUE_QueueQC.exitN, 10) }}, true
	case "QueueQC.dropN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_QUEUE_QueueQC.dropN, 10) }}, true
	case "QueueQC.timeoutN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_QUEUE_QueueQC.timeoutN, 10) }}, true
	case "QueueQC.len":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_QUEUE_QueueQC.len, 10) }}, true
	case "QueueQC.maxLen":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_QUEUE_QueueQC.maxLen, 10) }}, true
	case "DelayQC.enterN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_DELAY_DelayQC.enterN, 10) }}, true
	case "DelayQC.exitN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_DELAY_DelayQC.exitN, 10) }}, true
	case "DelayQC.busy":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_DELAY_DelayQC.busy, 10) }}, true
	case "DelayQC.maxBusy":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_DELAY_DelayQC.maxBusy, 10) }}, true
	case "QCResult.enterN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_BRANCH_QCResult.enterN, 10) }}, true
	case "QCResult.exitN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_BRANCH_QCResult.exitN, 10) }}, true
	case "SinkSuccessCount.enterN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_ASSIGN_SinkSuccessCount.enterN, 10) }}, true
	case "SinkSuccessCount.exitN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_ASSIGN_SinkSuccessCount.exitN, 10) }}, true
	case "SinkSuccess.enterN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_TERMINATE_SinkSuccess.enterN, 10) }}, true
	case "ReworkCountUp.enterN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_ASSIGN_ReworkCountUp.enterN, 10) }}, true
	case "ReworkCountUp.exitN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_ASSIGN_ReworkCountUp.exitN, 10) }}, true
	case "ReworkDecision.enterN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_BRANCH_ReworkDecision.enterN, 10) }}, true
	case "ReworkDecision.exitN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_BRANCH_ReworkDecision.exitN, 10) }}, true
	case "SinkScrapCount.enterN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_ASSIGN_SinkScrapCount.enterN, 10) }}, true
	case "SinkScrapCount.exitN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_ASSIGN_SinkScrapCount.exitN, 10) }}, true
	case "SinkScrap.enterN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_TERMINATE_SinkScrap.enterN, 10) }}, true
	default:
		return metricFunc{}, false
	}
}

func sendStat() {
	if len(metrics) == 0 {
		return
	}
	parts := make([]string, 0, len(metrics))
	for _, metric := range metrics {
		parts = append(parts, metric.name+"="+metric.value())
	}
	sendLine("STAT %s", strings.Join(parts, ";"))
}

func maybeSendMetricStep() {
	if !hasMetricStep || len(metrics) == 0 {
		return
	}
	if clock < nextMetricAt {
		return
	}
	sendStat()
	for nextMetricAt <= clock {
		nextMetricAt += metricStep
	}
}

func configureStopRule(leftName string, op string, rightName string) bool {
	left, ok := resolveNumericOperand(leftName)
	if !ok {
		sendErr("bad_rule_left", leftName)
		return false
	}
	right, ok := resolveNumericOperand(rightName)
	if !ok {
		sendErr("bad_rule_right", rightName)
		return false
	}
	switch op {
	case "==":
		stopRule = func() bool { return left() == right() }
	case "!=":
		stopRule = func() bool { return left() != right() }
	case ">":
		stopRule = func() bool { return left() > right() }
	case ">=":
		stopRule = func() bool { return left() >= right() }
	case "<":
		stopRule = func() bool { return left() < right() }
	case "<=":
		stopRule = func() bool { return left() <= right() }
	default:
		sendErr("bad_rule_op", op)
		return false
	}
	return true
}

func resolveNumericOperand(name string) (func() float64, bool) {
	switch name {
	case "Simulation.clock":
		return func() float64 { return clock }, true
	case "totalEntitiesEnd":
		return func() float64 { return float64(var_totalEntitiesEnd) }, true
	case "totalEntitiesSink":
		return func() float64 { return float64(var_totalEntitiesSink) }, true
	case "arrivalRate":
		return func() float64 { return var_arrivalRate }, true
	case "serviceARate":
		return func() float64 { return var_serviceARate }, true
	case "serviceBRate":
		return func() float64 { return var_serviceBRate }, true
	case "serviceQCRate":
		return func() float64 { return var_serviceQCRate }, true
	case "Source.createdN":
		return func() float64 { return float64(block_CREATE_Source.createdN) }, true
	case "Source.exitN":
		return func() float64 { return float64(block_CREATE_Source.exitN) }, true
	case "Source.dropN":
		return func() float64 { return float64(block_CREATE_Source.dropN) }, true
	case "BranchJobType.enterN":
		return func() float64 { return float64(block_BRANCH_BranchJobType.enterN) }, true
	case "BranchJobType.exitN":
		return func() float64 { return float64(block_BRANCH_BranchJobType.exitN) }, true
	case "AssignJobTypeOne.enterN":
		return func() float64 { return float64(block_ASSIGN_AssignJobTypeOne.enterN) }, true
	case "AssignJobTypeOne.exitN":
		return func() float64 { return float64(block_ASSIGN_AssignJobTypeOne.exitN) }, true
	case "AssignJobTypeTwo.enterN":
		return func() float64 { return float64(block_ASSIGN_AssignJobTypeTwo.enterN) }, true
	case "AssignJobTypeTwo.exitN":
		return func() float64 { return float64(block_ASSIGN_AssignJobTypeTwo.exitN) }, true
	case "BranchByType.enterN":
		return func() float64 { return float64(block_BRANCH_BranchByType.enterN) }, true
	case "BranchByType.exitN":
		return func() float64 { return float64(block_BRANCH_BranchByType.exitN) }, true
	case "QueueA.enterN":
		return func() float64 { return float64(block_QUEUE_QueueA.enterN) }, true
	case "QueueA.exitN":
		return func() float64 { return float64(block_QUEUE_QueueA.exitN) }, true
	case "QueueA.dropN":
		return func() float64 { return float64(block_QUEUE_QueueA.dropN) }, true
	case "QueueA.timeoutN":
		return func() float64 { return float64(block_QUEUE_QueueA.timeoutN) }, true
	case "QueueA.len":
		return func() float64 { return float64(block_QUEUE_QueueA.len) }, true
	case "QueueA.maxLen":
		return func() float64 { return float64(block_QUEUE_QueueA.maxLen) }, true
	case "DelayA.enterN":
		return func() float64 { return float64(block_DELAY_DelayA.enterN) }, true
	case "DelayA.exitN":
		return func() float64 { return float64(block_DELAY_DelayA.exitN) }, true
	case "DelayA.busy":
		return func() float64 { return float64(block_DELAY_DelayA.busy) }, true
	case "DelayA.maxBusy":
		return func() float64 { return float64(block_DELAY_DelayA.maxBusy) }, true
	case "QueueB.enterN":
		return func() float64 { return float64(block_QUEUE_QueueB.enterN) }, true
	case "QueueB.exitN":
		return func() float64 { return float64(block_QUEUE_QueueB.exitN) }, true
	case "QueueB.dropN":
		return func() float64 { return float64(block_QUEUE_QueueB.dropN) }, true
	case "QueueB.timeoutN":
		return func() float64 { return float64(block_QUEUE_QueueB.timeoutN) }, true
	case "QueueB.len":
		return func() float64 { return float64(block_QUEUE_QueueB.len) }, true
	case "QueueB.maxLen":
		return func() float64 { return float64(block_QUEUE_QueueB.maxLen) }, true
	case "DelayB.enterN":
		return func() float64 { return float64(block_DELAY_DelayB.enterN) }, true
	case "DelayB.exitN":
		return func() float64 { return float64(block_DELAY_DelayB.exitN) }, true
	case "DelayB.busy":
		return func() float64 { return float64(block_DELAY_DelayB.busy) }, true
	case "DelayB.maxBusy":
		return func() float64 { return float64(block_DELAY_DelayB.maxBusy) }, true
	case "QueueQC.enterN":
		return func() float64 { return float64(block_QUEUE_QueueQC.enterN) }, true
	case "QueueQC.exitN":
		return func() float64 { return float64(block_QUEUE_QueueQC.exitN) }, true
	case "QueueQC.dropN":
		return func() float64 { return float64(block_QUEUE_QueueQC.dropN) }, true
	case "QueueQC.timeoutN":
		return func() float64 { return float64(block_QUEUE_QueueQC.timeoutN) }, true
	case "QueueQC.len":
		return func() float64 { return float64(block_QUEUE_QueueQC.len) }, true
	case "QueueQC.maxLen":
		return func() float64 { return float64(block_QUEUE_QueueQC.maxLen) }, true
	case "DelayQC.enterN":
		return func() float64 { return float64(block_DELAY_DelayQC.enterN) }, true
	case "DelayQC.exitN":
		return func() float64 { return float64(block_DELAY_DelayQC.exitN) }, true
	case "DelayQC.busy":
		return func() float64 { return float64(block_DELAY_DelayQC.busy) }, true
	case "DelayQC.maxBusy":
		return func() float64 { return float64(block_DELAY_DelayQC.maxBusy) }, true
	case "QCResult.enterN":
		return func() float64 { return float64(block_BRANCH_QCResult.enterN) }, true
	case "QCResult.exitN":
		return func() float64 { return float64(block_BRANCH_QCResult.exitN) }, true
	case "SinkSuccessCount.enterN":
		return func() float64 { return float64(block_ASSIGN_SinkSuccessCount.enterN) }, true
	case "SinkSuccessCount.exitN":
		return func() float64 { return float64(block_ASSIGN_SinkSuccessCount.exitN) }, true
	case "SinkSuccess.enterN":
		return func() float64 { return float64(block_TERMINATE_SinkSuccess.enterN) }, true
	case "ReworkCountUp.enterN":
		return func() float64 { return float64(block_ASSIGN_ReworkCountUp.enterN) }, true
	case "ReworkCountUp.exitN":
		return func() float64 { return float64(block_ASSIGN_ReworkCountUp.exitN) }, true
	case "ReworkDecision.enterN":
		return func() float64 { return float64(block_BRANCH_ReworkDecision.enterN) }, true
	case "ReworkDecision.exitN":
		return func() float64 { return float64(block_BRANCH_ReworkDecision.exitN) }, true
	case "SinkScrapCount.enterN":
		return func() float64 { return float64(block_ASSIGN_SinkScrapCount.enterN) }, true
	case "SinkScrapCount.exitN":
		return func() float64 { return float64(block_ASSIGN_SinkScrapCount.exitN) }, true
	case "SinkScrap.enterN":
		return func() float64 { return float64(block_TERMINATE_SinkScrap.enterN) }, true
	}
	value, err := strconv.ParseFloat(name, 64)
	if err != nil {
		return nil, false
	}
	return func() float64 { return value }, true
}

func startCreateBlocks() {
	func_block_CREATE_Source()
}

func run() {
	for event := eventQueue.Pop(); event.Time != -1.0 && !stop; event = eventQueue.Pop() {
		clock = event.Time
		event.Handler()
		maybeSendMetricStep()
		if stop {
			break
		}
		if stopRule != nil && stopRule() {
			stop = true
			reasonStop = "stop_rule"
			break
		}
	}
	if reasonStop == "" {
		reasonStop = "empty_event_queue"
	}
	sendStat()
	sendLine("END %s", reasonStop)
}

func main() {
	if !configureExperiment() {
		return
	}
	initModel()
	startCreateBlocks()
	run()
}
