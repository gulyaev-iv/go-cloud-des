package generator

func (e *emitter) emitTransportDeclarations() {
	e.printf("type metricFunc struct {\n")
	e.printf("name string\n")
	e.printf("value func() string\n")
	e.printf("}\n\n")

	e.printf("var (\n")
	e.printf("protoOut *bufio.Writer\n")
	e.printf("metrics []metricFunc\n")
	e.printf("metricStep float64\n")
	e.printf("hasMetricStep bool\n")
	e.printf("nextMetricAt float64\n")
	e.printf("stopRule func() bool\n")
	e.printf(")\n\n")
}

func (e *emitter) emitTransportFunctions() {
	e.emitProtocolIOFunctions()
	e.emitCommandFunctions()
	e.emitSetFunctions()
	e.emitMetricFunctions()
	e.emitStopRuleFunctions()
}

func (e *emitter) emitProtocolIOFunctions() {
	e.printf("func sendLine(format string, args ...any) {\n")
	e.printf("if protoOut == nil { return }\n")
	e.printf("_, _ = fmt.Fprintf(protoOut, format+%q, args...)\n", "\n")
	e.printf("_ = protoOut.Flush()\n")
	e.printf("}\n\n")

	e.printf("func sendErr(code string, message string) {\n")
	e.printf("message = strings.ReplaceAll(message, %q, %q)\n", "\n", " ")
	e.printf("sendLine(%q, code, message)\n", "ERR %s %s")
	e.printf("}\n\n")

	e.printf("func configureExperiment() bool {\n")
	e.printf("protoOut = bufio.NewWriter(os.Stdout)\n")
	e.printf("scanner := bufio.NewScanner(os.Stdin)\n")
	e.printf("sendLine(%q, modelHash)\n", "HELLO %s")
	e.printf("for scanner.Scan() {\n")
	e.printf("line := strings.TrimSpace(scanner.Text())\n")
	e.printf("if line == %q { continue }\n", "")
	e.printf("if handleCommand(line) { return true }\n")
	e.printf("}\n")
	e.printf("if err := scanner.Err(); err != nil { sendErr(%q, err.Error()) }\n", "read_error")
	e.printf("return false\n")
	e.printf("}\n\n")
}

func (e *emitter) emitCommandFunctions() {
	e.printf("func handleCommand(line string) bool {\n")
	e.printf("fields := strings.Fields(line)\n")
	e.printf("if len(fields) == 0 { return false }\n")
	e.printf("switch fields[0] {\n")

	e.printf("case %q:\n", "RULE")
	e.printf("if len(fields) != 5 || fields[1] != %q { sendErr(%q, %q); return false }\n", "STOP", "bad_rule", "expected RULE STOP <left> <op> <right>")
	e.printf("if !configureStopRule(fields[2], fields[3], fields[4]) { return false }\n")

	e.printf("case %q:\n", "METRIC")
	e.printf("raw := strings.TrimSpace(strings.TrimPrefix(line, %q))\n", "METRIC")
	e.printf("if raw == %q { metrics = nil; return false }\n", "")
	e.printf("parts := strings.Split(raw, %q)\n", ";")
	e.printf("nextMetrics := make([]metricFunc, 0, len(parts))\n")
	e.printf("for _, part := range parts {\n")
	e.printf("name := strings.TrimSpace(part)\n")
	e.printf("if name == %q { continue }\n", "")
	e.printf("metric, ok := resolveMetric(name)\n")
	e.printf("if !ok { sendErr(%q, name); return false }\n", "unknown_metric")
	e.printf("nextMetrics = append(nextMetrics, metric)\n")
	e.printf("}\n")
	e.printf("metrics = nextMetrics\n")

	e.printf("case %q:\n", "METRIC_STEP")
	e.printf("if len(fields) != 2 { sendErr(%q, %q); return false }\n", "bad_metric_step", "expected METRIC_STEP <model_time_step>")
	e.printf("value, err := strconv.ParseFloat(fields[1], 64)\n")
	e.printf("if err != nil || value <= 0 { sendErr(%q, fields[1]); return false }\n", "bad_metric_step")
	e.printf("metricStep = value\n")
	e.printf("hasMetricStep = true\n")
	e.printf("nextMetricAt = metricStep\n")

	e.printf("case %q:\n", "START")
	e.printf("if len(fields) != 1 { sendErr(%q, %q); return false }\n", "bad_start", "START does not accept arguments")
	e.printf("sendLine(%q)\n", "READY")
	e.printf("return true\n")

	e.printf("case %q:\n", "SET")
	e.printf("if len(fields) == 3 {\n")
	e.printf("if !setVar(fields[1], fields[2]) { return false }\n")
	e.printf("} else if len(fields) == 4 && fields[2] == %q {\n", "=")
	e.printf("if !setVar(fields[1], fields[3]) { return false }\n")
	e.printf("} else {\n")
	e.printf("sendErr(%q, %q)\n", "bad_set", "expected SET <var_name> <value> or SET <var_name> = <value>")
	e.printf("return false\n")
	e.printf("}\n")

	e.printf("default:\n")
	e.printf("sendErr(%q, fields[0])\n", "unknown_command")

	e.printf("}\n")
	e.printf("return false\n")
	e.printf("}\n\n")
}

func (e *emitter) emitSetFunctions() {
	e.printf("func setVar(name string, raw string) bool {\n")
	e.printf("switch name {\n")

	for _, v := range e.model.Vars {
		e.emitSetVarCase(v)
	}

	e.printf("default:\n")
	e.printf("sendErr(%q, name)\n", "unknown_set_target")
	e.printf("return false\n")
	e.printf("}\n")
	e.printf("return true\n")
	e.printf("}\n\n")
}

func (e *emitter) emitSetVarCase(v *VarDesc) {
	e.printf("case %q:\n", v.Name)

	switch v.Type {
	case ValueFloat64:
		e.printf("value, err := strconv.ParseFloat(raw, 64)\n")
		e.printf("if err != nil { sendErr(%q, name+%q+raw); return false }\n", "bad_set_value", "=")
		e.printf("%s = value\n", varGoName(v))

	case ValueInt64:
		e.printf("value, err := strconv.ParseInt(raw, 10, 64)\n")
		e.printf("if err != nil { sendErr(%q, name+%q+raw); return false }\n", "bad_set_value", "=")
		e.printf("%s = value\n", varGoName(v))

	case ValueUint64:
		e.printf("value, err := strconv.ParseUint(raw, 10, 64)\n")
		e.printf("if err != nil { sendErr(%q, name+%q+raw); return false }\n", "bad_set_value", "=")
		e.printf("%s = value\n", varGoName(v))

	case ValueBool:
		e.printf("value, err := strconv.ParseBool(raw)\n")
		e.printf("if err != nil { sendErr(%q, name+%q+raw); return false }\n", "bad_set_value", "=")
		e.printf("%s = value\n", varGoName(v))

	default:
		e.printf("sendErr(%q, name)\n", "bad_set_target_type")
		e.printf("return false\n")
	}
}

func (e *emitter) emitMetricFunctions() {
	e.printf("func resolveMetric(name string) (metricFunc, bool) {\n")
	e.printf("switch name {\n")

	e.emitMetricCase("Simulation.clock", "clock", ValueFloat64)

	for _, v := range e.model.Vars {
		e.emitMetricCase(v.Name, varGoName(v), v.Type)
	}

	for _, block := range e.model.Blocks {
		for _, field := range transportBlockMetricFields(block) {
			e.emitMetricCase(block.Name+"."+field, blockGoName(block)+"."+field, ValueUint64)
		}
	}

	for _, resource := range e.model.Resources {
		for _, field := range transportResourceMetricFields() {
			e.emitMetricCase(resource.Name+"."+field, resourceGoName(resource)+"."+field, ValueUint64)
		}
	}

	e.printf("default:\n")
	e.printf("return metricFunc{}, false\n")
	e.printf("}\n")
	e.printf("}\n\n")

	e.printf("func sendStat() {\n")
	e.printf("if len(metrics) == 0 { return }\n")
	e.printf("parts := make([]string, 0, len(metrics))\n")
	e.printf("for _, metric := range metrics {\n")
	e.printf("parts = append(parts, metric.name+%q+metric.value())\n", "=")
	e.printf("}\n")
	e.printf("sendLine(%q, strings.Join(parts, %q))\n", "STAT %s", ";")
	e.printf("}\n\n")

	e.printf("func maybeSendMetricStep() {\n")
	e.printf("if !hasMetricStep || len(metrics) == 0 { return }\n")
	e.printf("if clock < nextMetricAt { return }\n")
	e.printf("sendStat()\n")
	e.printf("for nextMetricAt <= clock { nextMetricAt += metricStep }\n")
	e.printf("}\n\n")
}

func (e *emitter) emitMetricCase(name string, goExpr string, valueType ValueType) {
	e.printf("case %q:\n", name)
	e.printf("return metricFunc{name: name, value: func() string { return %s }}, true\n", transportFormatValueExpr(goExpr, valueType))
}

func (e *emitter) emitStopRuleFunctions() {
	e.printf("func configureStopRule(leftName string, op string, rightName string) bool {\n")
	e.printf("left, ok := resolveNumericOperand(leftName)\n")
	e.printf("if !ok { sendErr(%q, leftName); return false }\n", "bad_rule_left")
	e.printf("right, ok := resolveNumericOperand(rightName)\n")
	e.printf("if !ok { sendErr(%q, rightName); return false }\n", "bad_rule_right")
	e.printf("switch op {\n")
	e.printf("case %q:\n", "==")
	e.printf("stopRule = func() bool { return left() == right() }\n")
	e.printf("case %q:\n", "!=")
	e.printf("stopRule = func() bool { return left() != right() }\n")
	e.printf("case %q:\n", ">")
	e.printf("stopRule = func() bool { return left() > right() }\n")
	e.printf("case %q:\n", ">=")
	e.printf("stopRule = func() bool { return left() >= right() }\n")
	e.printf("case %q:\n", "<")
	e.printf("stopRule = func() bool { return left() < right() }\n")
	e.printf("case %q:\n", "<=")
	e.printf("stopRule = func() bool { return left() <= right() }\n")
	e.printf("default:\n")
	e.printf("sendErr(%q, op)\n", "bad_rule_op")
	e.printf("return false\n")
	e.printf("}\n")
	e.printf("return true\n")
	e.printf("}\n\n")

	e.printf("func resolveNumericOperand(name string) (func() float64, bool) {\n")
	e.printf("switch name {\n")

	e.emitNumericOperandCase("Simulation.clock", "clock", ValueFloat64)

	for _, v := range e.model.Vars {
		e.emitNumericOperandCase(v.Name, varGoName(v), v.Type)
	}

	for _, block := range e.model.Blocks {
		for _, field := range transportBlockMetricFields(block) {
			e.emitNumericOperandCase(block.Name+"."+field, blockGoName(block)+"."+field, ValueUint64)
		}
	}

	for _, resource := range e.model.Resources {
		for _, field := range transportResourceMetricFields() {
			e.emitNumericOperandCase(resource.Name+"."+field, resourceGoName(resource)+"."+field, ValueUint64)
		}
	}

	e.printf("}\n")
	e.printf("value, err := strconv.ParseFloat(name, 64)\n")
	e.printf("if err != nil { return nil, false }\n")
	e.printf("return func() float64 { return value }, true\n")
	e.printf("}\n\n")
}

func (e *emitter) emitNumericOperandCase(name string, goExpr string, valueType ValueType) {
	expr, ok := transportNumericValueExpr(goExpr, valueType)
	if !ok {
		return
	}

	e.printf("case %q:\n", name)
	e.printf("return func() float64 { return %s }, true\n", expr)
}

func transportFormatValueExpr(goExpr string, valueType ValueType) string {
	switch valueType {
	case ValueFloat64:
		return "strconv.FormatFloat(" + goExpr + ", 'f', -1, 64)"
	case ValueInt64:
		return "strconv.FormatInt(" + goExpr + ", 10)"
	case ValueUint64:
		return "strconv.FormatUint(" + goExpr + ", 10)"
	case ValueBool:
		return "strconv.FormatBool(" + goExpr + ")"
	default:
		return `""`
	}
}

func transportNumericValueExpr(goExpr string, valueType ValueType) (string, bool) {
	switch valueType {
	case ValueFloat64:
		return goExpr, true
	case ValueInt64, ValueUint64:
		return "float64(" + goExpr + ")", true
	default:
		return "", false
	}
}

func transportBlockMetricFields(block *BlockDesc) []string {
	switch block.Params.(type) {
	case *CreateParams:
		return []string{"createdN", "exitN", "dropN"}
	case *QueueParams:
		return []string{"enterN", "exitN", "dropN", "timeoutN", "len", "maxLen"}
	case *DelayParams:
		return []string{"enterN", "exitN", "busy", "maxBusy"}
	case *SeizeParams:
		return []string{"enterN", "exitN"}
	case *ReleaseParams:
		return []string{"enterN", "exitN"}
	case *AssignParams:
		return []string{"enterN", "exitN"}
	case *BranchParams:
		return []string{"enterN", "exitN"}
	case *TerminateParams:
		return []string{"enterN"}
	default:
		return nil
	}
}

func transportResourceMetricFields() []string {
	return []string{"capacity", "busy", "maxBusy", "seizeN", "releaseN"}
}
