package generator

import "bytes"

func parseBlockSection(scanner *LineScanner, model *Model, count int) error {
	blocks, err := parseSection(scanner, model, count, model.Blocks, model.BlockByName, parseBlock, func(b *BlockDesc) string {
		return b.Name
	})
	if err != nil {
		return err
	}

	model.Blocks = blocks
	return nil
}

func parseBlock(scanner *LineScanner) (*BlockDesc, error) {
	if !scanner.ScanNoEmpty() {
		return nil, parseErr(scanner.LineNum(), ErrUnexpectedEOF, "expected block declaration")
	}

	typeToken := scanner.Word()

	nameToken := scanner.Word()
	if nameToken == nil {
		return nil, parseErr(scanner.LineNum(), ErrIncorrectFormat, "expected block name")
	}

	name, err := validateName(scanner, nameToken)
	if err != nil {
		return nil, err
	}

	if err := expectOpeningBrace(scanner, "block name"); err != nil {
		return nil, err
	}

	block := &BlockDesc{
		Name: name,
	}

	switch string(typeToken) {
	case "CREATE":
		block.Params, err = parseCreateParams(scanner)
	case "QUEUE":
		block.Params, err = parseQueueParams(scanner)
	case "DELAY":
		block.Params, err = parseDelayParams(scanner)
	case "SEIZE":
		block.Params, err = parseSeizeParams(scanner)
	case "RELEASE":
		block.Params, err = parseReleaseParams(scanner)
	case "TERMINATE":
		block.Params, err = parseTerminateParams(scanner)
	case "ASSIGN":
		block.Params, err = parseAssignParams(scanner)
	case "BRANCH":
		block.Params, err = parseBranchParams(scanner)
	default:
		return nil, parseErr(scanner.LineNum(), ErrUnknownBlockType, `unknown block type "`+string(typeToken)+`"`)
	}

	if err != nil {
		return nil, err
	}

	return block, nil
}

func readRequiredToken(scanner *LineScanner, param string) ([]byte, error) {
	token := scanner.Word()
	if token == nil {
		return nil, parseErr(scanner.LineNum(), ErrIncorrectFormat, `expected value for parameter "`+param+`"`)
	}

	if extra := scanner.Word(); extra != nil {
		return nil, parseErr(scanner.LineNum(), ErrIncorrectFormat, `unexpected token "`+string(extra)+`" after parameter "`+param+`"`)
	}

	return token, nil
}

func readRequiredName(scanner *LineScanner, param string) (string, error) {
	token, err := readRequiredToken(scanner, param)
	if err != nil {
		return "", err
	}

	name, err := validateName(scanner, token)
	if err != nil {
		return "", err
	}

	return name, nil
}

func readRequiredExpr(scanner *LineScanner, param string) (Expr, error) {
	expr := scanner.Line()
	if len(expr) == 0 {
		return "", parseErr(scanner.LineNum(), ErrIncorrectFormat, `expected expression for parameter "`+param+`"`)
	}

	return Expr(string(expr)), nil
}

func readRequiredBool(scanner *LineScanner, param string) (bool, error) {
	token, err := readRequiredToken(scanner, param)
	if err != nil {
		return false, err
	}

	switch string(token) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, parseErr(scanner.LineNum(), ErrIncorrectFormat, `expected bool value for parameter "`+param+`"`)
	}
}

func readBlockingPolicy(scanner *LineScanner, param string) (BlockingPolicy, error) {
	token, err := readRequiredToken(scanner, param)
	if err != nil {
		return 0, err
	}

	switch string(token) {
	case "error":
		return BlockingPolicyError, nil
	case "destroy":
		return BlockingPolicyDestroy, nil
	default:
		return 0, parseErr(scanner.LineNum(), ErrUnknownBlockingPolicy, `unknown blocking_policy "`+string(token)+`"`)
	}
}

func readQueueDiscipline(scanner *LineScanner, param string) (QueueDiscipline, error) {
	token, err := readRequiredToken(scanner, param)
	if err != nil {
		return 0, err
	}

	switch string(token) {
	case "fifo":
		return QueueDisciplineFIFO, nil
	default:
		return 0, parseErr(scanner.LineNum(), ErrIncorrectFormat, `unknown discipline "`+string(token)+`"`)
	}
}

func expectNoExtraAfterClosingBrace(scanner *LineScanner) error {
	if extra := scanner.Word(); extra != nil {
		return parseErr(scanner.LineNum(), ErrIncorrectFormat, `unexpected token "`+string(extra)+`" after "}"`)
	}

	return nil
}

type ReadFunc[T any] func(scanner *LineScanner, param string) (T, error)

func parseUniqueParam[T any](scanner *LineScanner, has *bool, field *T, read ReadFunc[T], block string, param string) error {
	if *has {
		return parseErr(scanner.LineNum(), ErrIncorrectFormat, "duplicate "+block+` parameter "`+param+`"`)
	}

	value, err := read(scanner, param)
	if err != nil {
		return err
	}

	*field = value
	*has = true
	return nil
}

func parseTerminateParams(scanner *LineScanner) (*TerminateParams, error) {
	if err := expectClosingBrace(scanner); err != nil {
		return nil, err
	}
	return &TerminateParams{}, nil
}

func parseCreateParams(scanner *LineScanner) (*CreateParams, error) {
	params := &CreateParams{
		Batch:          Expr("1"),
		BlockingPolicy: BlockingPolicyError,
	}

	hasEntity := false
	hasInterval := false
	hasNext := false
	hasImmediately := false
	hasBatch := false
	hasBlockingPolicy := false

	for scanner.ScanNoEmpty() {
		param := scanner.Word()

		if bytes.Equal(param, []byte("}")) {
			if err := expectNoExtraAfterClosingBrace(scanner); err != nil {
				return nil, err
			}

			if !hasEntity {
				return nil, parseErr(scanner.LineNum(), ErrIncorrectFormat, `missing required CREATE parameter "entity"`)
			}
			if !hasInterval {
				return nil, parseErr(scanner.LineNum(), ErrIncorrectFormat, `missing required CREATE parameter "interval"`)
			}
			if !hasNext {
				return nil, parseErr(scanner.LineNum(), ErrIncorrectFormat, `missing required CREATE parameter "next"`)
			}

			return params, nil
		}

		switch string(param) {
		case "entity":
			if err := parseUniqueParam(scanner, &hasEntity, &params.EntityName, readRequiredName, "CREATE", "entity"); err != nil {
				return nil, err
			}

		case "interval":
			if err := parseUniqueParam(scanner, &hasInterval, &params.Interval, readRequiredExpr, "CREATE", "interval"); err != nil {
				return nil, err
			}

		case "next":
			if err := parseUniqueParam(scanner, &hasNext, &params.NextName, readRequiredName, "CREATE", "next"); err != nil {
				return nil, err
			}

		case "immediately":
			if err := parseUniqueParam(scanner, &hasImmediately, &params.Immediately, readRequiredBool, "CREATE", "immediately"); err != nil {
				return nil, err
			}

		case "batch":
			if err := parseUniqueParam(scanner, &hasBatch, &params.Batch, readRequiredExpr, "CREATE", "batch"); err != nil {
				return nil, err
			}

		case "blocking_policy":
			if err := parseUniqueParam(scanner, &hasBlockingPolicy, &params.BlockingPolicy, readBlockingPolicy, "CREATE", "blocking_policy"); err != nil {
				return nil, err
			}

		default:
			return nil, parseErr(scanner.LineNum(), ErrUnknownBlockParam, `unknown CREATE parameter "`+string(param)+`"`)
		}
	}

	return nil, parseErr(scanner.LineNum(), ErrUnexpectedEOF, `expected "}" after CREATE block`)
}

func parseQueueParams(scanner *LineScanner) (*QueueParams, error) {
	params := &QueueParams{
		Discipline: QueueDisciplineFIFO,
	}

	hasNext := false
	hasCapacity := false
	hasDiscipline := false
	hasTimeout := false
	hasNextTimeout := false
	hasExitOnOverflow := false
	hasNextOverflow := false

	for scanner.ScanNoEmpty() {
		param := scanner.Word()

		if bytes.Equal(param, []byte("}")) {
			if err := expectNoExtraAfterClosingBrace(scanner); err != nil {
				return nil, err
			}

			if !hasNext {
				return nil, parseErr(scanner.LineNum(), ErrIncorrectFormat, `missing required QUEUE parameter "next"`)
			}
			if !hasCapacity {
				return nil, parseErr(scanner.LineNum(), ErrIncorrectFormat, `missing required QUEUE parameter "capacity"`)
			}
			if params.ExitOnOverflow && !hasNextOverflow {
				return nil, parseErr(scanner.LineNum(), ErrIncorrectFormat, `missing required QUEUE parameter "next_overflow"`)
			}

			if params.ExitOnOverflow && params.InfiniteCapacity {
				return nil, parseErr(scanner.LineNum(), ErrIncorrectFormat, `QUEUE with capacity infinity cannot use exit_on_overflow or next_overflow`)
			}

			return params, nil
		}

		switch string(param) {
		case "next":
			if err := parseUniqueParam(scanner, &hasNext, &params.NextName, readRequiredName, "QUEUE", "next"); err != nil {
				return nil, err
			}

		case "capacity":
			if hasCapacity {
				return nil, parseErr(scanner.LineNum(), ErrIncorrectFormat, `duplicate QUEUE parameter "capacity"`)
			}

			token := scanner.Line()
			if len(token) == 0 {
				return nil, parseErr(scanner.LineNum(), ErrIncorrectFormat, `expected expression for parameter "capacity"`)
			}

			raw := string(bytes.TrimSpace(token))
			if raw == "infinity" {
				params.InfiniteCapacity = true
				params.Capacity = Expr("")
			} else {
				params.Capacity = Expr(raw)
			}

			hasCapacity = true

		case "discipline":
			if err := parseUniqueParam(scanner, &hasDiscipline, &params.Discipline, readQueueDiscipline, "QUEUE", "discipline"); err != nil {
				return nil, err
			}

		case "timeout":
			if err := parseUniqueParam(scanner, &hasTimeout, &params.Timeout, readRequiredExpr, "QUEUE", "timeout"); err != nil {
				return nil, err
			}
			params.HasTimeout = true

		case "next_timeout":
			if err := parseUniqueParam(scanner, &hasNextTimeout, &params.NextTimeoutName, readRequiredName, "QUEUE", "next_timeout"); err != nil {
				return nil, err
			}

		case "exit_on_overflow":
			if err := parseUniqueParam(scanner, &hasExitOnOverflow, &params.ExitOnOverflow, readRequiredBool, "QUEUE", "exit_on_overflow"); err != nil {
				return nil, err
			}

		case "next_overflow":
			if err := parseUniqueParam(scanner, &hasNextOverflow, &params.NextOverflowName, readRequiredName, "QUEUE", "next_overflow"); err != nil {
				return nil, err
			}

		default:
			return nil, parseErr(scanner.LineNum(), ErrUnknownBlockParam, `unknown QUEUE parameter "`+string(param)+`"`)
		}
	}

	return nil, parseErr(scanner.LineNum(), ErrUnexpectedEOF, `expected "}" after QUEUE block`)
}

func parseDelayParams(scanner *LineScanner) (*DelayParams, error) {
	params := &DelayParams{
		WaitQueueNames: make([]string, 0),
	}

	hasNext := false
	hasDuration := false
	hasCapacity := false

	for scanner.ScanNoEmpty() {
		param := scanner.Word()

		if bytes.Equal(param, []byte("}")) {
			if err := expectNoExtraAfterClosingBrace(scanner); err != nil {
				return nil, err
			}

			if !hasNext {
				return nil, parseErr(scanner.LineNum(), ErrIncorrectFormat, `missing required DELAY parameter "next"`)
			}
			if !hasDuration {
				return nil, parseErr(scanner.LineNum(), ErrIncorrectFormat, `missing required DELAY parameter "duration"`)
			}

			return params, nil
		}

		switch string(param) {
		case "next":
			if err := parseUniqueParam(scanner, &hasNext, &params.NextName, readRequiredName, "DELAY", "next"); err != nil {
				return nil, err
			}

		case "duration":
			if err := parseUniqueParam(scanner, &hasDuration, &params.Duration, readRequiredExpr, "DELAY", "duration"); err != nil {
				return nil, err
			}

		case "capacity":
			if err := parseUniqueParam(scanner, &hasCapacity, &params.Capacity, readRequiredExpr, "DELAY", "capacity"); err != nil {
				return nil, err
			}
			params.HasCapacity = true

		case "wait_queue":
			name, err := readRequiredName(scanner, "wait_queue")
			if err != nil {
				return nil, err
			}

			params.WaitQueueNames = append(params.WaitQueueNames, name)

		default:
			return nil, parseErr(scanner.LineNum(), ErrUnknownBlockParam, `unknown DELAY parameter "`+string(param)+`"`)
		}
	}

	return nil, parseErr(scanner.LineNum(), ErrUnexpectedEOF, `expected "}" after DELAY block`)
}

func parseSeizeParams(scanner *LineScanner) (*SeizeParams, error) {
	params := &SeizeParams{
		WaitQueueNames: make([]string, 0),
	}

	hasResource := false
	hasNext := false

	for scanner.ScanNoEmpty() {
		param := scanner.Word()

		if bytes.Equal(param, []byte("}")) {
			if err := expectNoExtraAfterClosingBrace(scanner); err != nil {
				return nil, err
			}

			if !hasResource {
				return nil, parseErr(scanner.LineNum(), ErrIncorrectFormat, `missing required SEIZE parameter "resource"`)
			}
			if !hasNext {
				return nil, parseErr(scanner.LineNum(), ErrIncorrectFormat, `missing required SEIZE parameter "next"`)
			}

			return params, nil
		}

		switch string(param) {
		case "resource":
			if err := parseUniqueParam(scanner, &hasResource, &params.ResourceName, readRequiredName, "SEIZE", "resource"); err != nil {
				return nil, err
			}

		case "next":
			if err := parseUniqueParam(scanner, &hasNext, &params.NextName, readRequiredName, "SEIZE", "next"); err != nil {
				return nil, err
			}

		case "wait_queue":
			name, err := readRequiredName(scanner, "wait_queue")
			if err != nil {
				return nil, err
			}

			params.WaitQueueNames = append(params.WaitQueueNames, name)

		default:
			return nil, parseErr(scanner.LineNum(), ErrUnknownBlockParam, `unknown SEIZE parameter "`+string(param)+`"`)
		}
	}

	return nil, parseErr(scanner.LineNum(), ErrUnexpectedEOF, `expected "}" after SEIZE block`)
}

func parseReleaseParams(scanner *LineScanner) (*ReleaseParams, error) {
	params := &ReleaseParams{}

	hasResource := false
	hasNext := false

	for scanner.ScanNoEmpty() {
		param := scanner.Word()

		if bytes.Equal(param, []byte("}")) {
			if err := expectNoExtraAfterClosingBrace(scanner); err != nil {
				return nil, err
			}

			if !hasResource {
				return nil, parseErr(scanner.LineNum(), ErrIncorrectFormat, `missing required RELEASE parameter "resource"`)
			}
			if !hasNext {
				return nil, parseErr(scanner.LineNum(), ErrIncorrectFormat, `missing required RELEASE parameter "next"`)
			}

			return params, nil
		}

		switch string(param) {
		case "resource":
			if err := parseUniqueParam(scanner, &hasResource, &params.ResourceName, readRequiredName, "RELEASE", "resource"); err != nil {
				return nil, err
			}

		case "next":
			if err := parseUniqueParam(scanner, &hasNext, &params.NextName, readRequiredName, "RELEASE", "next"); err != nil {
				return nil, err
			}

		default:
			return nil, parseErr(scanner.LineNum(), ErrUnknownBlockParam, `unknown RELEASE parameter "`+string(param)+`"`)
		}
	}

	return nil, parseErr(scanner.LineNum(), ErrUnexpectedEOF, `expected "}" after RELEASE block`)
}

func parseAssignmentTarget(scanner *LineScanner, target []byte) (AssignmentTarget, error) {
	target = bytes.TrimSpace(target)
	if len(target) == 0 {
		return AssignmentTarget{}, parseErr(scanner.LineNum(), ErrIncorrectFormat, `expected assignment target`)
	}

	dot := bytes.IndexByte(target, '.')
	if dot == -1 {
		name, err := validateName(scanner, target)
		if err != nil {
			return AssignmentTarget{}, err
		}

		return AssignmentTarget{
			IsEntityField: false,
			VarName:       name,
		}, nil
	}

	if dot == 0 || dot == len(target)-1 {
		return AssignmentTarget{}, parseErr(scanner.LineNum(), ErrIncorrectFormat, `invalid entity field target "`+string(target)+`"`)
	}

	if bytes.IndexByte(target[dot+1:], '.') != -1 {
		return AssignmentTarget{}, parseErr(scanner.LineNum(), ErrIncorrectFormat, `invalid entity field target "`+string(target)+`"`)
	}

	entityName, err := validateName(scanner, target[:dot])
	if err != nil {
		return AssignmentTarget{}, err
	}

	fieldName, err := validateName(scanner, target[dot+1:])
	if err != nil {
		return AssignmentTarget{}, err
	}

	return AssignmentTarget{
		IsEntityField: true,
		EntityName:    entityName,
		FieldName:     fieldName,
	}, nil
}

func parseAssignment(scanner *LineScanner) (Assignment, error) {
	line := scanner.Line()
	if len(line) == 0 {
		return Assignment{}, parseErr(scanner.LineNum(), ErrIncorrectFormat, `expected assignment expression`)
	}

	eq := bytes.IndexByte(line, '=')
	if eq == -1 {
		return Assignment{}, parseErr(scanner.LineNum(), ErrIncorrectFormat, `expected "=" in assignment`)
	}

	targetRaw := bytes.TrimSpace(line[:eq])
	exprRaw := bytes.TrimSpace(line[eq+1:])

	if len(exprRaw) == 0 {
		return Assignment{}, parseErr(scanner.LineNum(), ErrIncorrectFormat, `expected assignment value`)
	}

	target, err := parseAssignmentTarget(scanner, targetRaw)
	if err != nil {
		return Assignment{}, err
	}

	return Assignment{
		Target: target,
		Expr:   Expr(string(exprRaw)),
	}, nil
}

func parseAssignParams(scanner *LineScanner) (*AssignParams, error) {
	params := &AssignParams{
		Assignments: make([]Assignment, 0),
	}

	hasNext := false

	for scanner.ScanNoEmpty() {
		param := scanner.Word()

		if bytes.Equal(param, []byte("}")) {
			if err := expectNoExtraAfterClosingBrace(scanner); err != nil {
				return nil, err
			}

			if !hasNext {
				return nil, parseErr(scanner.LineNum(), ErrIncorrectFormat, `missing required ASSIGN parameter "next"`)
			}

			return params, nil
		}

		switch string(param) {
		case "set":
			assignment, err := parseAssignment(scanner)
			if err != nil {
				return nil, err
			}

			params.Assignments = append(params.Assignments, assignment)

		case "next":
			if err := parseUniqueParam(scanner, &hasNext, &params.NextName, readRequiredName, "ASSIGN", "next"); err != nil {
				return nil, err
			}

		default:
			return nil, parseErr(scanner.LineNum(), ErrUnknownBlockParam, `unknown ASSIGN parameter "`+string(param)+`"`)
		}
	}

	return nil, parseErr(scanner.LineNum(), ErrUnexpectedEOF, `expected "}" after ASSIGN block`)
}

func splitColon(scanner *LineScanner, line []byte) ([]byte, []byte, error) {
	idx := bytes.IndexByte(line, ':')
	if idx == -1 {
		return nil, nil, parseErr(scanner.LineNum(), ErrIncorrectFormat, `expected ":"`)
	}

	left := bytes.TrimSpace(line[:idx])
	right := bytes.TrimSpace(line[idx+1:])

	if len(right) == 0 {
		return nil, nil, parseErr(scanner.LineNum(), ErrIncorrectFormat, `expected block name after ":"`)
	}

	if bytes.Contains(right, []byte(":")) {
		return nil, nil, parseErr(scanner.LineNum(), ErrIncorrectFormat, `unexpected second ":"`)
	}

	return left, right, nil
}

func parseBranchCase(scanner *LineScanner) (BranchCase, error) {
	line := scanner.Line()
	conditionRaw, nextRaw, err := splitColon(scanner, line)
	if err != nil {
		return BranchCase{}, err
	}

	if len(conditionRaw) == 0 {
		return BranchCase{}, parseErr(scanner.LineNum(), ErrIncorrectFormat, `expected branch condition`)
	}

	nextName, err := validateName(scanner, nextRaw)
	if err != nil {
		return BranchCase{}, err
	}

	return BranchCase{
		Condition: Expr(string(conditionRaw)),
		NextName:  nextName,
	}, nil
}

func parseBranchElse(scanner *LineScanner) (string, error) {
	line := scanner.Line()
	left, nextRaw, err := splitColon(scanner, line)
	if err != nil {
		return "", err
	}

	if len(left) != 0 {
		return "", parseErr(scanner.LineNum(), ErrIncorrectFormat, `unexpected condition before else target`)
	}

	nextName, err := validateName(scanner, nextRaw)
	if err != nil {
		return "", err
	}

	return nextName, nil
}

func parseBranchParams(scanner *LineScanner) (*BranchParams, error) {
	params := &BranchParams{
		Cases: make([]BranchCase, 0),
	}

	for scanner.ScanNoEmpty() {
		param := scanner.Word()

		if bytes.Equal(param, []byte("}")) {
			if err := expectNoExtraAfterClosingBrace(scanner); err != nil {
				return nil, err
			}

			if len(params.Cases) == 0 && !params.HasElse {
				return nil, parseErr(scanner.LineNum(), ErrIncorrectFormat, `missing BRANCH cases`)
			}

			return params, nil
		}

		switch string(param) {
		case "if":
			branchCase, err := parseBranchCase(scanner)
			if err != nil {
				return nil, err
			}

			params.Cases = append(params.Cases, branchCase)

		case "else":
			if params.HasElse {
				return nil, parseErr(scanner.LineNum(), ErrIncorrectFormat, `duplicate BRANCH parameter "else"`)
			}

			nextName, err := parseBranchElse(scanner)
			if err != nil {
				return nil, err
			}

			params.ElseName = nextName
			params.HasElse = true

		default:
			return nil, parseErr(scanner.LineNum(), ErrUnknownBlockParam, `unknown BRANCH parameter "`+string(param)+`"`)
		}
	}

	return nil, parseErr(scanner.LineNum(), ErrUnexpectedEOF, `expected "}" after BRANCH block`)
}
