package container

import (
	"fmt"
	"math/rand/v2"
	"testing"
)

func TestEventQueuePush(t *testing.T) {
	hSize := 100000
	q := NewEventQueue(hSize)

	minValue := 1.0
	for range hSize {
		value := rand.Float64()
		if value < minValue {
			minValue = value
		}
		q.Push(Event{Time: value})
		if q.nodes[0].Time != minValue {
			t.Fatalf("ERROR PUSH: First in heap = %f, expected = %f", q.nodes[0].Time, minValue)
		}
	}
}

func TestEventQueuePop(t *testing.T) {
	hSize := 1000000
	q := NewEventQueue(hSize)

	for range hSize {
		q.Push(Event{Time: rand.Float64()})
	}
	prevEventTime := 0.0
	for event := q.Pop(); event.Time != -1.0; event = q.Pop() {
		if event.Time < prevEventTime {
			t.Fatalf("ERROR POP: Expecter that pop Event.Time=%f >= previous pop Event.Time=%f", event.Time, prevEventTime)
		}
	}
}

func BenchmarkBinaryHeap(b *testing.B) {
	var hSize int

	benchFuncPush := func(b *testing.B) {
		for b.Loop() {
			b.StopTimer()
			q := NewEventQueue(hSize)
			pushValues := make([]float64, 0, hSize)
			for range hSize {
				pushValues = append(pushValues, rand.Float64())
			}
			b.StartTimer()
			for i := range hSize {
				q.Push(Event{Time: pushValues[i]})
			}
		}
	}

	benchFuncPop := func(b *testing.B) {
		q := NewEventQueue(hSize)

		for b.Loop() {
			b.StopTimer()
			for range hSize {
				q.Push(Event{Time: rand.Float64()})
			}
			b.StartTimer()

			for range hSize {
				q.Pop()
			}
		}
	}

	for hSize = 100; hSize <= 1_000_000; hSize *= 10 {
		b.Run(fmt.Sprintf("size-%v", hSize), func(b *testing.B) {
			b.Run("Push", benchFuncPush)
			b.Run("Pop", benchFuncPop)
		})
	}
}

func BenchmarkBinaryHeap4(b *testing.B) {

	for hSize := 1000; hSize <= 1_000_000; hSize *= 10 {
		b.Run(fmt.Sprintf("size-%v", hSize), func(b *testing.B) {
			q := NewEventQueue(hSize)

			for range hSize {
				q.Push(Event{Time: rand.Float64()})
			}

			for b.Loop() {
				b.StopTimer()
				event := Event{Time: rand.Float64()}
				b.StartTimer()
				q.Pop()
				q.Push(event)
			}
		})
	}
}
