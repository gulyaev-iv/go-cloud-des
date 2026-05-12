package generator

import (
	"bytes"
	"fmt"
	"go/format"
)

type emitter struct {
	model *Model
	buf   bytes.Buffer
	err   error
}

func emit(model *Model) ([]byte, error) {
	e := &emitter{
		model: model,
	}

	e.emitHeader()
	e.emitSimulationDeclarations()
	e.emitTransportDeclarations()
	e.emitEntityDeclarations()
	e.emitVarDeclarations()
	e.emitResourceDeclarations()
	e.emitBlockDeclarations()
	e.emitInitFunction()
	e.emitScheduleFunction()
	e.emitFreeEntityFunction()
	e.emitBlockFunctions()
	e.emitTransportFunctions()
	e.emitStartCreateBlocksFunction()
	e.emitRunFunction()
	e.emitMainFunction()

	if e.err != nil {
		return nil, e.err
	}

	source := e.buf.Bytes()

	formatted, err := format.Source(source)
	if err != nil {
		return source, fmt.Errorf("Go format error: %w\n", err)
	}

	return formatted, nil
}

func (e *emitter) printf(format string, args ...any) {
	if e.err != nil {
		return
	}

	_, _ = fmt.Fprintf(&e.buf, format, args...)
}

func (e *emitter) fail(format string, args ...any) {
	if e.err != nil {
		return
	}

	e.err = fmt.Errorf(format, args...)
}

func (e *emitter) emitHeader() {
	e.printf("package main\n\n")

	e.printf("import (\n")
	e.printf("%q\n", "bufio")
	e.printf("%q\n", "fmt")
	e.printf("stdrand %q\n", "math/rand/v2")
	e.printf("%q\n", "os")
	e.printf("%q\n", "strconv")
	e.printf("%q\n", "strings")
	e.printf("%q\n", "github.com/gulyaev-iv/go-cloud-des/codegen/container")
	e.printf("random %q\n", "github.com/gulyaev-iv/go-cloud-des/codegen/rand")
	e.printf(")\n\n")

	e.printf("const modelHash = %q\n\n", e.model.Hash)

	e.printf("const nullEntityType uint8 = 0\n\n")

	e.printf("type EntityRef struct {\n")
	e.printf("Type uint8\n")
	e.printf("Idx uint32\n")
	e.printf("}\n\n")

	e.printf("type queueSlot struct {\n")
	e.printf("ref EntityRef\n")
	e.printf("valid bool\n")
	e.printf("}\n\n")
}

func (e *emitter) emitSimulationDeclarations() {
	e.printf("var (\n")
	e.printf("eventQueue *container.EventQueue\n")
	e.printf("clock float64 = 0.0\n")
	e.printf("stop bool = false\n")
	e.printf("reasonStop string\n")
	e.printf("rng *random.Rand\n")
	e.printf(")\n\n")
}

func (e *emitter) emitEntityDeclarations() {
	for i, entity := range e.model.Entities {
		e.printf("const %s uint8 = %d\n\n", entityTypeName(entity), i+1)

		e.printf("type %s struct {\n", entityStructName(entity))
		for _, field := range entity.Fields {
			e.printf("%s %s\n", entityFieldName(field), goType(field.Type))
		}
		e.printf("}\n\n")

		e.printf("var %s *container.Pool[%s]\n\n", entityPoolName(entity), entityStructName(entity))
	}
}

func (e *emitter) emitVarDeclarations() {
	for _, v := range e.model.Vars {
		e.printf("var %s %s = %s\n", varGoName(v), goType(v.Type), v.Default)
	}

	e.printf("\n")
}

func (e *emitter) emitResourceDeclarations() {
	if len(e.model.Resources) == 0 {
		return
	}
	e.printf("type struct_resource struct {\n")
	e.printf("capacity uint64\n")
	e.printf("busy uint64\n")
	e.printf("maxBusy uint64\n")
	e.printf("seizeN uint64\n")
	e.printf("releaseN uint64\n")
	e.printf("}\n\n")

	for _, resource := range e.model.Resources {
		e.printf("var %s struct_resource\n", resourceGoName(resource))
	}

	e.printf("\n")
}

func (e *emitter) emitBlockDeclarations() {
	for _, block := range e.model.Blocks {
		e.printf("type %s struct {\n", blockStructName(block))
		e.emitBlockFields(block)
		e.printf("}\n\n")

		e.printf("var %s *%s\n\n", blockGoName(block), blockStructName(block))
	}
}

func (e *emitter) emitInitFunction() {
	e.printf("func initModel() {\n")

	e.printf("eventQueue = container.NewEventQueue(1000)\n")
	e.printf("clock = 0.0\n")
	e.printf("stop = false\n")
	e.printf("reasonStop = \"\"\n")
	e.printf("rng = random.New(stdrand.NewPCG(stdrand.Uint64(), stdrand.Uint64()))\n")

	for _, entity := range e.model.Entities {
		e.printf("%s = container.NewPool[%s](10)\n", entityPoolName(entity), entityStructName(entity))
	}

	for _, resource := range e.model.Resources {
		capacity := e.emitNoEntityTypedExpr(
			resource.Capacity,
			ValueUint64,
			"RESOURCE "+resource.Name+" capacity",
		)
		e.printf("%s = struct_resource{capacity: uint64(%s)}\n", resourceGoName(resource), capacity)
	}

	for _, block := range e.model.Blocks {
		e.printf("%s = &%s{}\n", blockGoName(block), blockStructName(block))
	}

	e.printf("}\n\n")
}

func (e *emitter) emitScheduleFunction() {
	e.printf("func Schedule(time float64, handler func()) {\n")
	e.printf("eventQueue.Push(container.Event{Time: clock + time, Handler: handler})\n")
	e.printf("}\n\n")
}

func (e *emitter) emitFreeEntityFunction() {
	e.printf("func freeEntity(ref EntityRef) {\n")
	e.printf("switch ref.Type {\n")

	for _, entity := range e.model.Entities {
		e.printf("case %s:\n", entityTypeName(entity))
		e.printf("%s.Free(ref.Idx)\n", entityPoolName(entity))
	}

	e.printf("default:\n")
	e.printf("stop = true\n")
	e.printf("reasonStop = %q\n", "unknown entity type")
	e.printf("}\n")
	e.printf("}\n\n")
}
