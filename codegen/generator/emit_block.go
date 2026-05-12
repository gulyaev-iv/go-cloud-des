package generator

import (
	"strings"
)

func (e *emitter) emitBlockFields(block *BlockDesc) {
	switch block.Params.(type) {
	case *CreateParams:
		e.emitCreateBlockFields()

	case *QueueParams:
		e.emitQueueBlockFields()

	case *DelayParams:
		e.emitDelayBlockFields()

	case *SeizeParams:
		e.emitSeizeBlockFields()

	case *ReleaseParams:
		e.emitReleaseBlockFields()

	case *AssignParams:
		e.emitAssignBlockFields()

	case *BranchParams:
		e.emitBranchBlockFields()

	case *TerminateParams:
		e.emitTerminateBlockFields()

	default:
		e.fail("unknown block params type for block %q", block.Name)
	}
}

func (e *emitter) emitCreateBlockFields() {
	e.printf("createdN uint64\n")
	e.printf("exitN uint64\n")
	e.printf("dropN uint64\n")
}

func (e *emitter) emitQueueBlockFields() {
	e.printf("arr []queueSlot\n")
	e.printf("total uint64\n")

	e.printf("enterN uint64\n")
	e.printf("exitN uint64\n")
	e.printf("dropN uint64\n")
	e.printf("timeoutN uint64\n")
	e.printf("len uint64\n")
	e.printf("maxLen uint64\n")
}

func (e *emitter) emitDelayBlockFields() {
	e.printf("enterN uint64\n")
	e.printf("exitN uint64\n")
	e.printf("busy uint64\n")
	e.printf("maxBusy uint64\n")
}

func (e *emitter) emitSeizeBlockFields() {
	e.printf("enterN uint64\n")
	e.printf("exitN uint64\n")
}

func (e *emitter) emitReleaseBlockFields() {
	e.printf("enterN uint64\n")
	e.printf("exitN uint64\n")
}

func (e *emitter) emitAssignBlockFields() {
	e.printf("enterN uint64\n")
	e.printf("exitN uint64\n")
}

func (e *emitter) emitBranchBlockFields() {
	e.printf("enterN uint64\n")
	e.printf("exitN uint64\n")
}

func (e *emitter) emitTerminateBlockFields() {
	e.printf("enterN uint64\n")
}

func (e *emitter) emitBlockFunctions() {
	for _, block := range e.model.Blocks {
		switch params := block.Params.(type) {
		case *CreateParams:
			e.emitCreateBlockFunction(block, params)

		case *QueueParams:
			e.emitQueueBlockFunction(block, params)

		case *DelayParams:
			e.emitDelayBlockFunction(block, params)

		case *SeizeParams:
			e.emitSeizeBlockFunction(block, params)

		case *ReleaseParams:
			e.emitReleaseBlockFunction(block, params)

		case *AssignParams:
			e.emitAssignBlockFunction(block, params)

		case *BranchParams:
			e.emitBranchBlockFunction(block, params)

		case *TerminateParams:
			e.emitTerminateBlockFunction(block)
		}
	}
}
func (e *emitter) emitCreateBlockFunction(block *BlockDesc, params *CreateParams) {
	interval := e.emitNoEntityTypedExpr(
		params.Interval,
		ValueFloat64,
		"CREATE "+block.Name+" interval",
	)

	e.printf("func %s() {\n", blockFuncName(block))

	if params.Immediately {
		e.emitCreateBlockBody(block, params)

		e.printf("if !stop {\n")
		e.printf("Schedule(float64(%s), func() {\n", interval)
		e.printf("%s()\n", blockFuncName(block))
		e.printf("})\n")
		e.printf("}\n")
	} else {
		e.printf("Schedule(float64(%s), func() {\n", interval)

		e.emitCreateBlockBody(block, params)

		e.printf("if !stop {\n")
		e.printf("%s()\n", blockFuncName(block))
		e.printf("}\n")

		e.printf("})\n")
	}

	e.printf("}\n\n")
}

func (e *emitter) emitCreateBlockBody(block *BlockDesc, params *CreateParams) {
	batch := e.emitNoEntityTypedExpr(
		params.Batch,
		ValueUint64,
		"CREATE "+block.Name+" batch",
	)

	e.printf("batch := uint64(%s)\n", batch)
	e.printf("for i := uint64(0); i < batch; i++ {\n")

	e.printf("newEntity := %s.Alloc()\n", entityPoolName(params.Entity))

	for _, field := range params.Entity.Fields {
		e.printf("%s.Entities[newEntity].%s = %s\n", entityPoolName(params.Entity), entityFieldName(field), field.Default)
	}

	e.printf("ref := EntityRef{Type: %s, Idx: newEntity}\n", entityTypeName(params.Entity))

	e.printf("%s.createdN++\n", blockGoName(block))

	e.printf("if %s(ref) {\n", blockFuncName(params.Next))
	e.printf("%s.exitN++\n", blockGoName(block))
	e.printf("} else if stop {\n")
	e.printf("return\n")
	e.printf("} else {\n")

	if params.BlockingPolicy == BlockingPolicyDestroy {
		e.printf("%s.dropN++\n", blockGoName(block))
		e.printf("freeEntity(ref)\n")
	} else {
		e.printf("stop = true\n")
		e.printf("reasonStop = %q\n", block.Name+": заявка не смогла покинуть блок")
		e.printf("return\n")
	}

	e.printf("}\n")
	e.printf("}\n")
}

func (e *emitter) emitQueueBlockFunction(block *BlockDesc, params *QueueParams) {
	e.emitQueuePushFunction(block)
	e.emitQueueTopFunction(block)
	e.emitQueueDeleteFunction(block)
	e.emitQueueGetFunction(block)
	e.emitQueueTryAdvanceFunction(block, params)
	e.emitQueueMainFunction(block, params)
}

func (e *emitter) emitQueuePushFunction(block *BlockDesc) {
	e.printf("func %s(ref EntityRef) uint64 {\n", queuePushFuncName(block))
	e.printf("idx := %s.total\n", blockGoName(block))
	e.printf("%s.total++\n", blockGoName(block))
	e.printf("%s.arr = append(%s.arr, queueSlot{ref: ref, valid: true})\n", blockGoName(block), blockGoName(block))
	e.printf("%s.len++\n", blockGoName(block))
	e.printf("if %s.len > %s.maxLen {\n", blockGoName(block), blockGoName(block))
	e.printf("%s.maxLen = %s.len\n", blockGoName(block), blockGoName(block))
	e.printf("}\n")
	e.printf("return idx\n")
	e.printf("}\n\n")
}

func (e *emitter) emitQueueTopFunction(block *BlockDesc) {
	e.printf("func %s() (EntityRef, bool) {\n", queueTopFuncName(block))
	e.printf("for len(%s.arr) > 0 && !%s.arr[0].valid {\n", blockGoName(block), blockGoName(block))
	e.printf("%s.arr = %s.arr[1:]\n", blockGoName(block), blockGoName(block))
	e.printf("}\n")
	e.printf("if %s.len == 0 || len(%s.arr) == 0 {\n", blockGoName(block), blockGoName(block))
	e.printf("return EntityRef{Type: nullEntityType, Idx: container.NullIdx}, false\n")
	e.printf("}\n")
	e.printf("return %s.arr[0].ref, true\n", blockGoName(block))
	e.printf("}\n\n")
}

func (e *emitter) emitQueueDeleteFunction(block *BlockDesc) {
	e.printf("func %s() {\n", queueDeleteFuncName(block))
	e.printf("for len(%s.arr) > 0 && !%s.arr[0].valid {\n", blockGoName(block), blockGoName(block))
	e.printf("%s.arr = %s.arr[1:]\n", blockGoName(block), blockGoName(block))
	e.printf("}\n")
	e.printf("if len(%s.arr) == 0 {\n", blockGoName(block))
	e.printf("return\n")
	e.printf("}\n")
	e.printf("%s.arr = %s.arr[1:]\n", blockGoName(block), blockGoName(block))
	e.printf("if %s.len > 0 {\n", blockGoName(block))
	e.printf("%s.len--\n", blockGoName(block))
	e.printf("}\n")
	e.printf("}\n\n")
}

func (e *emitter) emitQueueGetFunction(block *BlockDesc) {
	e.printf("func %s(idx uint64) (EntityRef, bool) {\n", queueGetFuncName(block))
	e.printf("deleted := %s.total - uint64(len(%s.arr))\n", blockGoName(block), blockGoName(block))
	e.printf("if idx < deleted {\n")
	e.printf("return EntityRef{Type: nullEntityType, Idx: container.NullIdx}, false\n")
	e.printf("}\n")
	e.printf("pos := idx - deleted\n")
	e.printf("if pos >= uint64(len(%s.arr)) {\n", blockGoName(block))
	e.printf("return EntityRef{Type: nullEntityType, Idx: container.NullIdx}, false\n")
	e.printf("}\n")
	e.printf("slot := &%s.arr[pos]\n", blockGoName(block))
	e.printf("if !slot.valid {\n")
	e.printf("return EntityRef{Type: nullEntityType, Idx: container.NullIdx}, false\n")
	e.printf("}\n")
	e.printf("slot.valid = false\n")
	e.printf("if %s.len > 0 {\n", blockGoName(block))
	e.printf("%s.len--\n", blockGoName(block))
	e.printf("}\n")
	e.printf("return slot.ref, true\n")
	e.printf("}\n\n")
}

func (e *emitter) emitQueueTryAdvanceFunction(block *BlockDesc, params *QueueParams) {
	e.printf("func %s() bool {\n", queueTryAdvanceFuncName(block))
	e.printf("ref, ok := %s()\n", queueTopFuncName(block))
	e.printf("if !ok {\n")
	e.printf("return false\n")
	e.printf("}\n")
	e.printf("if !%s(ref) {\n", blockFuncName(params.Next))
	e.printf("return false\n")
	e.printf("}\n")
	e.printf("%s()\n", queueDeleteFuncName(block))
	e.printf("%s.exitN++\n", blockGoName(block))
	e.printf("return true\n")
	e.printf("}\n\n")
}

func (e *emitter) emitQueueMainFunction(block *BlockDesc, params *QueueParams) {
	e.printf("func %s(ref EntityRef) bool {\n", blockFuncName(block))

	if !params.InfiniteCapacity {
		capacity := e.emitNoEntityTypedExpr(
			params.Capacity,
			ValueUint64,
			"QUEUE "+block.Name+" capacity",
		)
		e.printf("capacity := uint64(%s)\n", capacity)

		e.printf("if %s.len >= capacity {\n", blockGoName(block))

		if params.ExitOnOverflow {
			e.printf("%s.enterN++\n", blockGoName(block))
			e.printf("%s.dropN++\n", blockGoName(block))
			e.printf("if %s(ref) {\n", blockFuncName(params.NextOverflow))
			e.printf("return true\n")
			e.printf("}\n")
			e.printf("return false\n")
		} else {
			e.printf("return false\n")
		}
		e.printf("}\n")
	}

	e.printf("%s.enterN++\n", blockGoName(block))

	if params.HasTimeout {
		timeout, timeoutEntity := e.emitRuntimeTypedExpr(
			params.Timeout,
			ValueFloat64,
			"QUEUE "+block.Name+" timeout",
		)

		if timeoutEntity != nil {
			e.printf("if ref.Type != %s {\n", entityTypeName(timeoutEntity))
			e.printf("stop = true\n")
			e.printf("reasonStop = %q\n", block.Name+": timeout expression expects entity "+timeoutEntity.Name)
			e.printf("return false\n")
			e.printf("}\n")
		}

		e.printf("timeout := float64(%s)\n", timeout)
		e.printf("idx := %s(ref)\n", queuePushFuncName(block))

		e.printf("Schedule(timeout, func() {\n")
		e.printf("timeoutRef, ok := %s(idx)\n", queueGetFuncName(block))
		e.printf("if !ok {\n")
		e.printf("return\n")
		e.printf("}\n")
		e.printf("%s.timeoutN++\n", blockGoName(block))
		e.printf("if %s(timeoutRef) {\n", blockFuncName(params.NextTimeout))
		e.printf("return\n")
		e.printf("}\n")
		e.printf("if stop {\n")
		e.printf("return\n")
		e.printf("}\n")
		e.printf("stop = true\n")
		e.printf("reasonStop = %q\n", block.Name+": заявка не смогла покинуть next_timeout")
		e.printf("})\n")
	} else {
		e.printf("%s(ref)\n", queuePushFuncName(block))
	}

	e.printf("%s()\n", queueTryAdvanceFuncName(block))
	e.printf("return true\n")
	e.printf("}\n\n")
}

func (e *emitter) emitDelayBlockFunction(block *BlockDesc, params *DelayParams) {
	e.printf("func %s(ref EntityRef) bool {\n", blockFuncName(block))

	if params.HasCapacity {
		capacity := e.emitNoEntityTypedExpr(
			params.Capacity,
			ValueUint64,
			"DELAY "+block.Name+" capacity",
		)

		e.printf("capacity := uint64(%s)\n", capacity)
		e.printf("if %s.busy >= capacity {\n", blockGoName(block))
		e.printf("return false\n")
		e.printf("}\n")
	}

	duration, durationEntity := e.emitRuntimeTypedExpr(
		params.Duration,
		ValueFloat64,
		"DELAY "+block.Name+" duration",
	)

	if durationEntity != nil {
		e.printf("if ref.Type != %s {\n", entityTypeName(durationEntity))
		e.printf("stop = true\n")
		e.printf("reasonStop = %q\n", block.Name+": duration expression expects entity "+durationEntity.Name)
		e.printf("return false\n")
		e.printf("}\n")
	}

	e.printf("duration := float64(%s)\n", duration)

	e.printf("%s.enterN++\n", blockGoName(block))
	e.printf("%s.busy++\n", blockGoName(block))

	e.printf("if %s.busy > %s.maxBusy {\n", blockGoName(block), blockGoName(block))
	e.printf("%s.maxBusy = %s.busy\n", blockGoName(block), blockGoName(block))
	e.printf("}\n")

	e.printf("Schedule(duration, func() {\n")

	e.printf("if %s(ref) {\n", blockFuncName(params.Next))
	e.printf("%s.exitN++\n", blockGoName(block))
	e.printf("} else if stop {\n")
	e.printf("return\n")
	e.printf("} else {\n")
	e.printf("stop = true\n")
	e.printf("reasonStop = %q\n", block.Name+": заявка не смогла покинуть блок")
	e.printf("return\n")
	e.printf("}\n")

	e.printf("if %s.busy > 0 {\n", blockGoName(block))
	e.printf("%s.busy--\n", blockGoName(block))
	e.printf("}\n")

	e.emitDelayWakeWaitQueues(params)

	e.printf("})\n")

	e.printf("return true\n")
	e.printf("}\n\n")
}

func (e *emitter) emitDelayWakeWaitQueues(params *DelayParams) {
	for _, queue := range params.WaitQueues {
		e.printf("if %s() {\n", queueTryAdvanceFuncName(queue))
		e.printf("return\n")
		e.printf("}\n")
	}
}

func (e *emitter) emitTerminateBlockFunction(block *BlockDesc) {
	e.printf("func %s(ref EntityRef) bool {\n", blockFuncName(block))
	e.printf("%s.enterN++\n", blockGoName(block))
	e.printf("freeEntity(ref)\n")
	e.printf("return true\n")
	e.printf("}\n\n")
}

func (e *emitter) emitAssignBlockFunction(block *BlockDesc, params *AssignParams) {
	e.printf("func %s(ref EntityRef) bool {\n", blockFuncName(block))

	for i, assignment := range params.Assignments {
		_, _, targetType := e.emitAssignmentTarget(assignment.Target)
		e.printf("var assignOld%d %s\n", i, goType(targetType))
		e.printf("assignChanged%d := false\n", i)
	}

	for i, assignment := range params.Assignments {
		e.emitTransactionalAssignmentStatement(i, assignment)
	}

	e.printf("if %s(ref) {\n", blockFuncName(params.Next))
	e.printf("%s.enterN++\n", blockGoName(block))
	e.printf("%s.exitN++\n", blockGoName(block))
	e.printf("return true\n")
	e.printf("}\n")

	for i := len(params.Assignments) - 1; i >= 0; i-- {
		assignment := params.Assignments[i]
		target, _, _ := e.emitAssignmentTarget(assignment.Target)

		e.printf("if assignChanged%d {\n", i)
		e.printf("%s = assignOld%d\n", target, i)
		e.printf("}\n")
	}

	e.printf("return false\n")
	e.printf("}\n\n")
}

func (e *emitter) emitTransactionalAssignmentStatement(index int, assignment Assignment) {
	target, targetEntity, targetType := e.emitAssignmentTarget(assignment.Target)
	expr, exprEntity := e.emitRuntimeTypedExpr(
		assignment.Expr,
		targetType,
		"ASSIGN expression",
	)

	guardEntity := e.mergeAssignmentEntityGuard(string(assignment.Expr), targetEntity, exprEntity)
	if guardEntity != nil {
		e.printf("if ref.Type == %s {\n", entityTypeName(guardEntity))
	}

	e.printf("assignOld%d = %s\n", index, target)
	e.printf("assignChanged%d = true\n", index)
	e.printf("%s = %s\n", target, expr)

	if guardEntity != nil {
		e.printf("}\n")
	}
}

func (e *emitter) emitAssignmentTarget(target AssignmentTarget) (string, *EntityDesc, ValueType) {
	if target.IsEntityField {
		entity, ok := e.model.EntityByName[target.EntityName]
		if !ok {
			e.fail("unknown assignment target entity %q", target.EntityName)
			return "unknown", nil, ValueInt64
		}

		field, ok := entity.FieldByName[target.FieldName]
		if !ok {
			e.fail("unknown assignment target field %q.%q", target.EntityName, target.FieldName)
			return "unknown", nil, ValueInt64
		}

		return entityPoolName(entity) + ".Entities[ref.Idx]." + entityFieldName(field), entity, field.Type
	}

	v, ok := e.model.VarByName[target.VarName]
	if !ok {
		e.fail("unknown assignment target variable %q", target.VarName)
		return "unknown", nil, ValueInt64
	}

	return varGoName(v), nil, v.Type
}

func (e *emitter) mergeAssignmentEntityGuard(raw string, targetEntity *EntityDesc, exprEntity *EntityDesc) *EntityDesc {
	if targetEntity == nil {
		return exprEntity
	}

	if exprEntity == nil {
		return targetEntity
	}

	if targetEntity != exprEntity {
		e.fail(
			"assignment expression %q uses entity %q, but assignment target belongs to entity %q",
			raw,
			exprEntity.Name,
			targetEntity.Name,
		)
		return targetEntity
	}

	return targetEntity
}

func (e *emitter) emitBranchBlockFunction(block *BlockDesc, params *BranchParams) {
	e.printf("func %s(ref EntityRef) bool {\n", blockFuncName(block))

	for i, branchCase := range params.Cases {
		condition := e.emitBranchCondition(block, branchCase.Condition)

		if i == 0 {
			e.printf("if %s {\n", condition)
		} else {
			e.printf("} else if %s {\n", condition)
		}

		e.emitBranchNext(block, branchCase.Next)
	}

	if params.HasElse {
		e.printf("} else {\n")
		e.emitBranchNext(block, params.Else)
		e.printf("}\n")
	} else {
		e.printf("} else {\n")
		e.printf("stop = true\n")
		e.printf("reasonStop = %q\n", block.Name+": no branch condition matched")
		e.printf("return false\n")
		e.printf("}\n")
	}

	e.printf("}\n\n")
}

func (e *emitter) emitBranchNext(block *BlockDesc, next *BlockDesc) {
	e.printf("if %s(ref) {\n", blockFuncName(next))
	e.printf("%s.enterN++\n", blockGoName(block))
	e.printf("%s.exitN++\n", blockGoName(block))
	e.printf("return true\n")
	e.printf("}\n")
	e.printf("return false\n")
}

func (e *emitter) emitBranchCondition(block *BlockDesc, condition Expr) string {
	raw := strings.TrimSpace(string(condition))
	if raw == "" {
		e.fail("BRANCH %s condition: empty condition", block.Name)
		return "false"
	}

	if entity, ok := e.model.EntityByName[raw]; ok {
		return "ref.Type == " + entityTypeName(entity)
	}

	expr, exprEntity := e.emitRuntimeTypedExpr(
		condition,
		ValueBool,
		"BRANCH "+block.Name+" condition",
	)

	if exprEntity != nil {
		return "ref.Type == " + entityTypeName(exprEntity) + " && (" + expr + ")"
	}

	return expr
}

func (e *emitter) emitSeizeBlockFunction(block *BlockDesc, params *SeizeParams) {
	resource := resourceGoName(params.Resource)

	e.printf("func %s(ref EntityRef) bool {\n", blockFuncName(block))

	e.printf("if %s.busy >= %s.capacity {\n", resource, resource)
	e.printf("return false\n")
	e.printf("}\n")

	e.printf("oldBusy := %s.busy\n", resource)
	e.printf("oldMaxBusy := %s.maxBusy\n", resource)

	e.printf("%s.busy++\n", resource)
	e.printf("if %s.busy > %s.maxBusy {\n", resource, resource)
	e.printf("%s.maxBusy = %s.busy\n", resource, resource)
	e.printf("}\n")

	e.printf("if %s(ref) {\n", blockFuncName(params.Next))
	e.printf("%s.enterN++\n", blockGoName(block))
	e.printf("%s.exitN++\n", blockGoName(block))
	e.printf("%s.seizeN++\n", resource)
	e.printf("return true\n")
	e.printf("}\n")

	e.printf("%s.busy = oldBusy\n", resource)
	e.printf("%s.maxBusy = oldMaxBusy\n", resource)
	e.printf("return false\n")

	e.printf("}\n\n")
}

func (e *emitter) emitReleaseBlockFunction(block *BlockDesc, params *ReleaseParams) {
	resource := resourceGoName(params.Resource)

	e.printf("func %s(ref EntityRef) bool {\n", blockFuncName(block))

	e.printf("if %s.busy == 0 {\n", resource)
	e.printf("stop = true\n")
	e.printf("reasonStop = %q\n", block.Name+": попытка освободить пустой ресурс "+params.Resource.Name)
	e.printf("return false\n")
	e.printf("}\n")

	e.printf("oldBusy := %s.busy\n", resource)
	e.printf("%s.busy--\n", resource)

	e.printf("if %s(ref) {\n", blockFuncName(params.Next))
	e.printf("%s.enterN++\n", blockGoName(block))
	e.printf("%s.exitN++\n", blockGoName(block))
	e.printf("%s.releaseN++\n", resource)

	e.emitReleaseWakeResourceQueues(params.Resource)

	e.printf("return true\n")
	e.printf("}\n")

	e.printf("%s.busy = oldBusy\n", resource)
	e.printf("return false\n")

	e.printf("}\n\n")
}

func (e *emitter) emitReleaseWakeResourceQueues(resource *ResourceDesc) {
	for _, seizeBlock := range resource.Seizes {
		seizeParams, ok := seizeBlock.Params.(*SeizeParams)
		if !ok {
			continue
		}

		for _, queue := range seizeParams.WaitQueues {
			e.printf("if %s() {\n", queueTryAdvanceFuncName(queue))
			e.printf("return true\n")
			e.printf("}\n")
		}
	}
}
