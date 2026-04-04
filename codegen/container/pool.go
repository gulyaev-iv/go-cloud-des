package container

const NullIdx = ^uint32(0)

// Pool is free list for storing T entities.
type Pool[T any] struct {
	Entities []T
	FreeList []uint32
}

func NewPool[T any](cap int) *Pool[T] {
	return &Pool[T]{make([]T, 0, cap), make([]uint32, 0, cap)}
}

func (p *Pool[T]) Alloc() uint32 {
	if len(p.FreeList) == 0 {
		idx := uint32(len(p.Entities))
		p.Entities = append(p.Entities, *new(T))
		return idx
	}
	idx := p.FreeList[len(p.FreeList)-1]
	p.FreeList = p.FreeList[:len(p.FreeList)-1]
	return idx
}

func (p *Pool[T]) Free(idx uint32) {
	p.Entities[idx] = *new(T)
	p.FreeList = append(p.FreeList, idx)
}

func (p *Pool[T]) Get(idx uint32) *T {
	return &p.Entities[idx]
}
