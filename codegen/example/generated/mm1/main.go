package main

import (
	"bufio"
	"fmt"
	"github.com/gulyaev-iv/go-cloud-des/codegen/container"
	random "github.com/gulyaev-iv/go-cloud-des/codegen/rand"
	stdrand "math/rand/v2"
	"os"
	"strconv"
	"strings"
)

const modelHash = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

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

const entityType_Order uint8 = 1

type entity_Order struct {
}

var pool_entity_Order *container.Pool[entity_Order]

var var_arrivalRate float64 = 1.0
var var_serviceRate float64 = 1.25

type struct_block_CREATE_Source struct {
	createdN uint64
	exitN    uint64
	dropN    uint64
}

var block_CREATE_Source *struct_block_CREATE_Source

type struct_block_QUEUE_WaitQueue struct {
	arr      []queueSlot
	total    uint64
	enterN   uint64
	exitN    uint64
	dropN    uint64
	timeoutN uint64
	len      uint64
	maxLen   uint64
}

var block_QUEUE_WaitQueue *struct_block_QUEUE_WaitQueue

type struct_block_DELAY_ServiceDelay struct {
	enterN  uint64
	exitN   uint64
	busy    uint64
	maxBusy uint64
}

var block_DELAY_ServiceDelay *struct_block_DELAY_ServiceDelay

type struct_block_TERMINATE_Sink struct {
	enterN uint64
}

var block_TERMINATE_Sink *struct_block_TERMINATE_Sink

func initModel() {
	eventQueue = container.NewEventQueue(1000)
	clock = 0.0
	stop = false
	reasonStop = ""
	rng = random.New(stdrand.NewPCG(stdrand.Uint64(), stdrand.Uint64()))
	pool_entity_Order = container.NewPool[entity_Order](10)
	block_CREATE_Source = &struct_block_CREATE_Source{}
	block_QUEUE_WaitQueue = &struct_block_QUEUE_WaitQueue{}
	block_DELAY_ServiceDelay = &struct_block_DELAY_ServiceDelay{}
	block_TERMINATE_Sink = &struct_block_TERMINATE_Sink{}
}

func Schedule(time float64, handler func()) {
	eventQueue.Push(container.Event{Time: clock + time, Handler: handler})
}

func freeEntity(ref EntityRef) {
	switch ref.Type {
	case entityType_Order:
		pool_entity_Order.Free(ref.Idx)
	default:
		stop = true
		reasonStop = "unknown entity type"
	}
}

func func_block_CREATE_Source() {
	Schedule(float64(rng.Exponential(var_arrivalRate)), func() {
		batch := uint64(1)
		for i := uint64(0); i < batch; i++ {
			newEntity := pool_entity_Order.Alloc()
			ref := EntityRef{Type: entityType_Order, Idx: newEntity}
			block_CREATE_Source.createdN++
			if func_block_QUEUE_WaitQueue(ref) {
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

func func_queue_push_WaitQueue(ref EntityRef) uint64 {
	idx := block_QUEUE_WaitQueue.total
	block_QUEUE_WaitQueue.total++
	block_QUEUE_WaitQueue.arr = append(block_QUEUE_WaitQueue.arr, queueSlot{ref: ref, valid: true})
	block_QUEUE_WaitQueue.len++
	if block_QUEUE_WaitQueue.len > block_QUEUE_WaitQueue.maxLen {
		block_QUEUE_WaitQueue.maxLen = block_QUEUE_WaitQueue.len
	}
	return idx
}

func func_queue_top_WaitQueue() (EntityRef, bool) {
	for len(block_QUEUE_WaitQueue.arr) > 0 && !block_QUEUE_WaitQueue.arr[0].valid {
		block_QUEUE_WaitQueue.arr = block_QUEUE_WaitQueue.arr[1:]
	}
	if block_QUEUE_WaitQueue.len == 0 || len(block_QUEUE_WaitQueue.arr) == 0 {
		return EntityRef{Type: nullEntityType, Idx: container.NullIdx}, false
	}
	return block_QUEUE_WaitQueue.arr[0].ref, true
}

func func_queue_delete_WaitQueue() {
	for len(block_QUEUE_WaitQueue.arr) > 0 && !block_QUEUE_WaitQueue.arr[0].valid {
		block_QUEUE_WaitQueue.arr = block_QUEUE_WaitQueue.arr[1:]
	}
	if len(block_QUEUE_WaitQueue.arr) == 0 {
		return
	}
	block_QUEUE_WaitQueue.arr = block_QUEUE_WaitQueue.arr[1:]
	if block_QUEUE_WaitQueue.len > 0 {
		block_QUEUE_WaitQueue.len--
	}
}

func func_queue_get_WaitQueue(idx uint64) (EntityRef, bool) {
	deleted := block_QUEUE_WaitQueue.total - uint64(len(block_QUEUE_WaitQueue.arr))
	if idx < deleted {
		return EntityRef{Type: nullEntityType, Idx: container.NullIdx}, false
	}
	pos := idx - deleted
	if pos >= uint64(len(block_QUEUE_WaitQueue.arr)) {
		return EntityRef{Type: nullEntityType, Idx: container.NullIdx}, false
	}
	slot := &block_QUEUE_WaitQueue.arr[pos]
	if !slot.valid {
		return EntityRef{Type: nullEntityType, Idx: container.NullIdx}, false
	}
	slot.valid = false
	if block_QUEUE_WaitQueue.len > 0 {
		block_QUEUE_WaitQueue.len--
	}
	return slot.ref, true
}

func func_queue_try_advance_WaitQueue() bool {
	ref, ok := func_queue_top_WaitQueue()
	if !ok {
		return false
	}
	if !func_block_DELAY_ServiceDelay(ref) {
		return false
	}
	func_queue_delete_WaitQueue()
	block_QUEUE_WaitQueue.exitN++
	return true
}

func func_block_QUEUE_WaitQueue(ref EntityRef) bool {
	block_QUEUE_WaitQueue.enterN++
	func_queue_push_WaitQueue(ref)
	func_queue_try_advance_WaitQueue()
	return true
}

func func_block_DELAY_ServiceDelay(ref EntityRef) bool {
	capacity := uint64(1)
	if block_DELAY_ServiceDelay.busy >= capacity {
		return false
	}
	duration := float64(rng.Exponential(var_serviceRate))
	block_DELAY_ServiceDelay.enterN++
	block_DELAY_ServiceDelay.busy++
	if block_DELAY_ServiceDelay.busy > block_DELAY_ServiceDelay.maxBusy {
		block_DELAY_ServiceDelay.maxBusy = block_DELAY_ServiceDelay.busy
	}
	Schedule(duration, func() {
		if func_block_TERMINATE_Sink(ref) {
			block_DELAY_ServiceDelay.exitN++
		} else if stop {
			return
		} else {
			stop = true
			reasonStop = "ServiceDelay: заявка не смогла покинуть блок"
			return
		}
		if block_DELAY_ServiceDelay.busy > 0 {
			block_DELAY_ServiceDelay.busy--
		}
		if func_queue_try_advance_WaitQueue() {
			return
		}
	})
	return true
}

func func_block_TERMINATE_Sink(ref EntityRef) bool {
	block_TERMINATE_Sink.enterN++
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
		sendErr("unsupported_command", "SET is not supported yet")
	default:
		sendErr("unknown_command", fields[0])
	}
	return false
}

func resolveMetric(name string) (metricFunc, bool) {
	switch name {
	case "Simulation.clock":
		return metricFunc{name: name, value: func() string { return strconv.FormatFloat(clock, 'f', -1, 64) }}, true
	case "arrivalRate":
		return metricFunc{name: name, value: func() string { return strconv.FormatFloat(var_arrivalRate, 'f', -1, 64) }}, true
	case "serviceRate":
		return metricFunc{name: name, value: func() string { return strconv.FormatFloat(var_serviceRate, 'f', -1, 64) }}, true
	case "Source.createdN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_CREATE_Source.createdN, 10) }}, true
	case "Source.exitN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_CREATE_Source.exitN, 10) }}, true
	case "Source.dropN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_CREATE_Source.dropN, 10) }}, true
	case "WaitQueue.enterN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_QUEUE_WaitQueue.enterN, 10) }}, true
	case "WaitQueue.exitN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_QUEUE_WaitQueue.exitN, 10) }}, true
	case "WaitQueue.dropN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_QUEUE_WaitQueue.dropN, 10) }}, true
	case "WaitQueue.timeoutN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_QUEUE_WaitQueue.timeoutN, 10) }}, true
	case "WaitQueue.len":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_QUEUE_WaitQueue.len, 10) }}, true
	case "WaitQueue.maxLen":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_QUEUE_WaitQueue.maxLen, 10) }}, true
	case "ServiceDelay.enterN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_DELAY_ServiceDelay.enterN, 10) }}, true
	case "ServiceDelay.exitN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_DELAY_ServiceDelay.exitN, 10) }}, true
	case "ServiceDelay.busy":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_DELAY_ServiceDelay.busy, 10) }}, true
	case "ServiceDelay.maxBusy":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_DELAY_ServiceDelay.maxBusy, 10) }}, true
	case "Sink.enterN":
		return metricFunc{name: name, value: func() string { return strconv.FormatUint(block_TERMINATE_Sink.enterN, 10) }}, true
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
	case "arrivalRate":
		return func() float64 { return var_arrivalRate }, true
	case "serviceRate":
		return func() float64 { return var_serviceRate }, true
	case "Source.createdN":
		return func() float64 { return float64(block_CREATE_Source.createdN) }, true
	case "Source.exitN":
		return func() float64 { return float64(block_CREATE_Source.exitN) }, true
	case "Source.dropN":
		return func() float64 { return float64(block_CREATE_Source.dropN) }, true
	case "WaitQueue.enterN":
		return func() float64 { return float64(block_QUEUE_WaitQueue.enterN) }, true
	case "WaitQueue.exitN":
		return func() float64 { return float64(block_QUEUE_WaitQueue.exitN) }, true
	case "WaitQueue.dropN":
		return func() float64 { return float64(block_QUEUE_WaitQueue.dropN) }, true
	case "WaitQueue.timeoutN":
		return func() float64 { return float64(block_QUEUE_WaitQueue.timeoutN) }, true
	case "WaitQueue.len":
		return func() float64 { return float64(block_QUEUE_WaitQueue.len) }, true
	case "WaitQueue.maxLen":
		return func() float64 { return float64(block_QUEUE_WaitQueue.maxLen) }, true
	case "ServiceDelay.enterN":
		return func() float64 { return float64(block_DELAY_ServiceDelay.enterN) }, true
	case "ServiceDelay.exitN":
		return func() float64 { return float64(block_DELAY_ServiceDelay.exitN) }, true
	case "ServiceDelay.busy":
		return func() float64 { return float64(block_DELAY_ServiceDelay.busy) }, true
	case "ServiceDelay.maxBusy":
		return func() float64 { return float64(block_DELAY_ServiceDelay.maxBusy) }, true
	case "Sink.enterN":
		return func() float64 { return float64(block_TERMINATE_Sink.enterN) }, true
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
