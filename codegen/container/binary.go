package container

type Event struct {
	Time    float64
	Handler func()
}

type EventQueue struct {
	nodes []Event
}

func NewEventQueue(capacity int) *EventQueue {
	return &EventQueue{nodes: make([]Event, 0, capacity)}
}

func (q *EventQueue) Len() int {
	return len(q.nodes)
}

func (q *EventQueue) Push(e Event) {
	q.nodes = append(q.nodes, e)
	q.siftUp(len(q.nodes) - 1)
}

func (q *EventQueue) Pop() Event {
	if len(q.nodes) == 0 {
		return Event{Time: -1.0}
	}
	result := q.nodes[0]
	lastIdx := len(q.nodes) - 1

	q.nodes[0] = q.nodes[lastIdx]
	q.nodes = q.nodes[:lastIdx]

	if len(q.nodes) > 0 {
		q.siftDown(0)
	}
	return result
}

func (q *EventQueue) siftUp(idx int) {
	for idx > 0 {
		parent := (idx - 1) / 4
		if q.nodes[idx].Time >= q.nodes[parent].Time {
			break
		}
		q.nodes[idx], q.nodes[parent] = q.nodes[parent], q.nodes[idx]
		idx = parent
	}
}

func (q *EventQueue) siftDown(idx int) {
	for 4*idx+1 < len(q.nodes) {
		child1, child2, child3, child4 := 4*idx+1, 4*idx+2, 4*idx+3, 4*idx+4
		minIdx := idx
		if child1 < len(q.nodes) {
			if q.nodes[child1].Time < q.nodes[minIdx].Time {
				minIdx = child1
			}

			if child2 < len(q.nodes) {
				if q.nodes[child2].Time < q.nodes[minIdx].Time {
					minIdx = child2
				}

				if child3 < len(q.nodes) {
					if q.nodes[child3].Time < q.nodes[minIdx].Time {
						minIdx = child3
					}

					if child4 < len(q.nodes) {
						if q.nodes[child4].Time < q.nodes[minIdx].Time {
							minIdx = child4
						}

					}

				}

			}

		}

		if minIdx == idx {
			break
		}

		q.nodes[idx], q.nodes[minIdx] = q.nodes[minIdx], q.nodes[idx]
		idx = minIdx
	}
}
