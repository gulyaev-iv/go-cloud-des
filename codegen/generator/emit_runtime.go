package generator

func (e *emitter) emitStartCreateBlocksFunction() {
	e.printf("func startCreateBlocks() {\n")

	for _, block := range e.model.Blocks {
		if _, ok := block.Params.(*CreateParams); !ok {
			continue
		}

		e.printf("%s()\n", blockFuncName(block))
	}

	e.printf("}\n\n")
}

func (e *emitter) emitRunFunction() {
	e.printf("func run() {\n")
	e.printf("for event := eventQueue.Pop(); event.Time != -1.0 && !stop; event = eventQueue.Pop() {\n")
	e.printf("clock = event.Time\n")
	e.printf("event.Handler()\n")
	e.printf("maybeSendMetricStep()\n")
	e.printf("if stop { break }\n")
	e.printf("if stopRule != nil && stopRule() {\n")
	e.printf("stop = true\n")
	e.printf("reasonStop = %q\n", "stop_rule")
	e.printf("break\n")
	e.printf("}\n")
	e.printf("}\n")
	e.printf("if reasonStop == %q {\n", "")
	e.printf("reasonStop = %q\n", "empty_event_queue")
	e.printf("}\n")
	e.printf("sendStat()\n")
	e.printf("sendLine(%q, reasonStop)\n", "END %s")
	e.printf("}\n\n")
}

func (e *emitter) emitMainFunction() {
	e.printf("func main() {\n")
	e.printf("if !configureExperiment() { return }\n")
	e.printf("initModel()\n")
	e.printf("startCreateBlocks()\n")
	e.printf("run()\n")
	e.printf("}\n")
}
