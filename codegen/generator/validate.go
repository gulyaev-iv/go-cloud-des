package generator

type ModelErr struct {
	Err error
	Msg string
}

func modelErr(err error, msg string) error {
	if err == nil {
		return nil
	}
	return &ModelErr{
		Err: err,
		Msg: msg,
	}
}

func (e *ModelErr) Error() string {
	if e.Msg == "" {
		return e.Err.Error()
	}
	return e.Err.Error() + ": " + e.Msg
}

func (e *ModelErr) Unwrap() error {
	return e.Err
}

func ValidateAndLink(model *Model) error {
	if model == nil {
		return modelErr(ErrIncorrectFormat, "model is nil")
	}

	if err := validateRequiredBlocks(model); err != nil {
		return err
	}

	for _, block := range model.Blocks {
		if err := validateAndLinkBlock(model, block); err != nil {
			return err
		}
	}

	for _, resource := range model.Resources {
		if err := validateAndLinkResource(model, resource); err != nil {
			return err
		}
	}

	return nil
}

func validateRequiredBlocks(model *Model) error {
	hasCreate := false
	hasTerminate := false

	for _, block := range model.Blocks {
		switch block.Params.(type) {
		case *CreateParams:
			hasCreate = true
		case *TerminateParams:
			hasTerminate = true
		}
	}

	if !hasCreate {
		return modelErr(ErrUnknownReference, "model must contain at least one CREATE block")
	}

	if !hasTerminate {
		return modelErr(ErrUnknownReference, "model must contain at least one TERMINATE block")
	}

	return nil
}

func validateAndLinkBlock(model *Model, block *BlockDesc) error {
	if block == nil {
		return modelErr(ErrIncorrectFormat, "nil block")
	}

	if block.Params == nil {
		return modelErr(ErrIncorrectFormat, `block "`+block.Name+`" has nil params`)
	}

	switch params := block.Params.(type) {
	case *CreateParams:
		return validateAndLinkCreate(model, block, params)

	case *QueueParams:
		return validateAndLinkQueue(model, block, params)

	case *DelayParams:
		return validateAndLinkDelay(model, block, params)

	case *SeizeParams:
		return validateAndLinkSeize(model, block, params)

	case *ReleaseParams:
		return validateAndLinkRelease(model, block, params)

	case *AssignParams:
		return validateAndLinkAssign(model, block, params)

	case *BranchParams:
		return validateAndLinkBranch(model, block, params)

	case *TerminateParams:
		return nil

	default:
		return modelErr(ErrIncorrectFormat, `block "`+block.Name+`" has unknown params type`)
	}
}

func requireBlock(model *Model, ownerName string, paramName string, blockName string) (*BlockDesc, error) {
	block, ok := model.BlockByName[blockName]
	if !ok {
		return nil, modelErr(ErrUnknownReference, `"`+ownerName+`" references unknown block "`+blockName+`" in parameter "`+paramName+`"`)
	}

	return block, nil
}

func validateAndLinkCreate(model *Model, block *BlockDesc, params *CreateParams) error {
	entity, ok := model.EntityByName[params.EntityName]
	if !ok {
		return modelErr(ErrUnknownReference, `CREATE block "`+block.Name+`" references unknown entity "`+params.EntityName+`"`)
	}
	params.Entity = entity

	next, err := requireBlock(model, block.Name, "next", params.NextName)
	if err != nil {
		return err
	}
	params.Next = next

	return nil
}

func validateAndLinkQueue(model *Model, block *BlockDesc, params *QueueParams) error {
	next, err := requireBlock(model, block.Name, "next", params.NextName)
	if err != nil {
		return err
	}
	params.Next = next

	if params.HasTimeout && params.NextTimeoutName == "" {
		return modelErr(ErrUnknownReference, `QUEUE block "`+block.Name+`" has timeout but does not define next_timeout`)
	}

	if !params.HasTimeout && params.NextTimeoutName != "" {
		return modelErr(ErrIncorrectFormat, `QUEUE block "`+block.Name+`" defines next_timeout without timeout`)
	}

	if params.HasTimeout {
		nextTimeout, err := requireBlock(model, block.Name, "next_timeout", params.NextTimeoutName)
		if err != nil {
			return err
		}
		params.NextTimeout = nextTimeout
	}

	if params.ExitOnOverflow && params.NextOverflowName == "" {
		return modelErr(ErrUnknownReference, `QUEUE block "`+block.Name+`" has exit_on_overflow=true but does not define next_overflow`)
	}

	if !params.ExitOnOverflow && params.NextOverflowName != "" {
		return modelErr(ErrIncorrectFormat, `QUEUE block "`+block.Name+`" defines next_overflow while exit_on_overflow=false`)
	}

	if params.ExitOnOverflow {
		nextOverflow, err := requireBlock(model, block.Name, "next_overflow", params.NextOverflowName)
		if err != nil {
			return err
		}
		params.NextOverflow = nextOverflow
	}

	return nil
}

func requireQueueBlock(model *Model, ownerName string, paramName string, blockName string) (*BlockDesc, *QueueParams, error) {
	block, err := requireBlock(model, ownerName, paramName, blockName)
	if err != nil {
		return nil, nil, err
	}

	params, ok := block.Params.(*QueueParams)
	if !ok {
		return nil, nil, modelErr(ErrIncorrectFormat, `"`+ownerName+`" parameter "`+paramName+`" must reference QUEUE block "`+blockName+`", got `+blockKind(block))
	}

	return block, params, nil
}

func validateAndLinkDelay(model *Model, block *BlockDesc, params *DelayParams) error {
	next, err := requireBlock(model, block.Name, "next", params.NextName)
	if err != nil {
		return err
	}
	params.Next = next

	params.WaitQueues = make([]*BlockDesc, 0, len(params.WaitQueueNames))
	for _, queueName := range params.WaitQueueNames {
		queue, _, err := requireQueueBlock(model, block.Name, "wait_queue", queueName)
		if err != nil {
			return err
		}
		params.WaitQueues = append(params.WaitQueues, queue)
	}

	return nil
}

func validateAndLinkSeize(model *Model, block *BlockDesc, params *SeizeParams) error {
	resource, ok := model.ResourceByName[params.ResourceName]
	if !ok {
		return modelErr(ErrUnknownReference, `SEIZE block "`+block.Name+`" references unknown resource "`+params.ResourceName+`"`)
	}
	params.Resource = resource

	next, err := requireBlock(model, block.Name, "next", params.NextName)
	if err != nil {
		return err
	}
	params.Next = next

	params.WaitQueues = make([]*BlockDesc, 0, len(params.WaitQueueNames))
	for _, queueName := range params.WaitQueueNames {
		queue, _, err := requireQueueBlock(model, block.Name, "wait_queue", queueName)
		if err != nil {
			return err
		}
		params.WaitQueues = append(params.WaitQueues, queue)
	}

	if !containsString(resource.SeizeNames, block.Name) {
		return modelErr(ErrUnknownReference, `SEIZE block "`+block.Name+`" is not listed in resource "`+resource.Name+`"`)
	}

	return nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func validateAndLinkRelease(model *Model, block *BlockDesc, params *ReleaseParams) error {
	resource, ok := model.ResourceByName[params.ResourceName]
	if !ok {
		return modelErr(ErrUnknownReference, `RELEASE block "`+block.Name+`" references unknown resource "`+params.ResourceName+`"`)
	}
	params.Resource = resource

	next, err := requireBlock(model, block.Name, "next", params.NextName)
	if err != nil {
		return err
	}
	params.Next = next

	return nil
}

func validateAndLinkAssign(model *Model, block *BlockDesc, params *AssignParams) error {
	next, err := requireBlock(model, block.Name, "next", params.NextName)
	if err != nil {
		return err
	}
	params.Next = next

	for _, assignment := range params.Assignments {
		if err := validateAssignmentTarget(model, block, assignment.Target); err != nil {
			return err
		}
	}

	return nil
}

func validateAssignmentTarget(model *Model, block *BlockDesc, target AssignmentTarget) error {
	if target.IsEntityField {
		entity, ok := model.EntityByName[target.EntityName]
		if !ok {
			return modelErr(ErrUnknownReference, `ASSIGN block "`+block.Name+`" references unknown entity "`+target.EntityName+`"`)
		}

		if _, ok := entity.FieldByName[target.FieldName]; !ok {
			return modelErr(ErrUnknownReference, `ASSIGN block "`+block.Name+`" references unknown field "`+target.EntityName+`.`+target.FieldName+`"`)
		}

		return nil
	}

	if _, ok := model.VarByName[target.VarName]; !ok {
		return modelErr(ErrUnknownReference, `ASSIGN block "`+block.Name+`" references unknown variable "`+target.VarName+`"`)
	}

	return nil
}

func validateAndLinkBranch(model *Model, block *BlockDesc, params *BranchParams) error {
	for i := range params.Cases {
		next, err := requireBlock(model, block.Name, "if", params.Cases[i].NextName)
		if err != nil {
			return err
		}
		params.Cases[i].Next = next
	}

	if params.HasElse {
		next, err := requireBlock(model, block.Name, "else", params.ElseName)
		if err != nil {
			return err
		}
		params.Else = next
	}

	return nil
}

func validateAndLinkResource(model *Model, resource *ResourceDesc) error {
	if resource == nil {
		return modelErr(ErrIncorrectFormat, "nil resource")
	}

	resource.Seizes = make([]*BlockDesc, 0, len(resource.SeizeNames))

	for _, seizeName := range resource.SeizeNames {
		seizeBlock, seizeParams, err := requireSeizeBlock(model, resource.Name, "seize", seizeName)
		if err != nil {
			return err
		}

		if seizeParams.ResourceName != resource.Name {
			return modelErr(ErrUnknownReference, `resource "`+resource.Name+`" lists SEIZE block "`+seizeBlock.Name+`", but this block references resource "`+seizeParams.ResourceName+`"`)
		}

		resource.Seizes = append(resource.Seizes, seizeBlock)
	}

	return nil
}

func requireSeizeBlock(model *Model, ownerName string, paramName string, blockName string) (*BlockDesc, *SeizeParams, error) {
	block, err := requireBlock(model, ownerName, paramName, blockName)
	if err != nil {
		return nil, nil, err
	}

	params, ok := block.Params.(*SeizeParams)
	if !ok {
		return nil, nil, modelErr(ErrIncorrectFormat, `"`+ownerName+`" parameter "`+paramName+`" must reference SEIZE block "`+blockName+`", got `+blockKind(block))
	}

	return block, params, nil
}

func blockKind(block *BlockDesc) string {
	if block == nil || block.Params == nil {
		return "UNKNOWN"
	}

	switch block.Params.(type) {
	case *CreateParams:
		return "CREATE"
	case *QueueParams:
		return "QUEUE"
	case *DelayParams:
		return "DELAY"
	case *SeizeParams:
		return "SEIZE"
	case *ReleaseParams:
		return "RELEASE"
	case *AssignParams:
		return "ASSIGN"
	case *BranchParams:
		return "BRANCH"
	case *TerminateParams:
		return "TERMINATE"
	default:
		return "UNKNOWN"
	}
}
