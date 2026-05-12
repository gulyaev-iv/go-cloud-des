package generator

import (
	"fmt"
	"strings"
)

type emittedExpr struct {
	Go     string
	Type   ValueType
	Entity *EntityDesc
}

type emitExprMode struct {
	Context          string
	AllowEntityField bool
	HasExpected      bool
	Expected         ValueType
	AllowEmpty       bool
}

type exprOperand struct {
	Type         ValueType
	UntypedNum   bool
	FloatLiteral bool
}

type resolvedExprName struct {
	Go     string
	Type   ValueType
	Entity *EntityDesc
}

type exprFlags struct {
	hasArithmetic bool
	hasComparison bool
	hasBoolOp     bool
}

func (e *emitter) emitNoEntityTypedExpr(expr Expr, expected ValueType, context string) string {
	result := e.translateExpr(expr, emitExprMode{
		Context:          context,
		AllowEntityField: false,
		HasExpected:      true,
		Expected:         expected,
	})
	return result.Go
}

func (e *emitter) emitRuntimeTypedExpr(expr Expr, expected ValueType, context string) (string, *EntityDesc) {
	result := e.translateExpr(expr, emitExprMode{
		Context:          context,
		AllowEntityField: true,
		HasExpected:      true,
		Expected:         expected,
	})
	return result.Go, result.Entity
}

func (e *emitter) translateExpr(expr Expr, mode emitExprMode) emittedExpr {
	raw := strings.TrimSpace(string(expr))
	if raw == "" {
		if mode.AllowEmpty {
			return emittedExpr{Go: "", Type: mode.Expected}
		}

		e.exprFail(mode, raw, "empty expression")
		return emittedExpr{Go: exprZeroValue(mode)}
	}

	result := e.translateExprRaw(raw, mode)
	if mode.HasExpected && !exprTypesCompatible(result.Type, mode.Expected) {
		e.exprFail(
			mode,
			raw,
			"expected %s, got %s. Use explicit cast if this conversion is intentional",
			valueTypeName(mode.Expected),
			valueTypeName(result.Type),
		)
		return emittedExpr{Go: exprZeroValue(mode), Type: mode.Expected}
	}

	return result
}

func (e *emitter) translateExprRaw(raw string, mode emitExprMode) emittedExpr {
	var out strings.Builder
	var operands []exprOperand
	var flags exprFlags
	var exprEntity *EntityDesc

	for i := 0; i < len(raw); {
		c := raw[i]

		if isIdentStart(c) {
			name, next, ok := e.readExprName(raw, i, mode)
			if !ok {
				return emittedExpr{Go: exprZeroValue(mode)}
			}

			openIdx := nextNonSpace(raw, next)
			if openIdx < len(raw) && raw[openIdx] == '(' {
				call, callNext, ok := e.translateExprCall(raw, name, openIdx, mode)
				if !ok {
					return emittedExpr{Go: exprZeroValue(mode)}
				}

				out.WriteString(call.Go)
				operands = append(operands, exprOperand{Type: call.Type})
				exprEntity = e.mergeExprEntity(raw, mode, exprEntity, call.Entity)
				i = callNext
				continue
			}

			resolved := e.resolveExprName(raw, name, mode)
			out.WriteString(resolved.Go)
			operands = append(operands, exprOperand{Type: resolved.Type})
			exprEntity = e.mergeExprEntity(raw, mode, exprEntity, resolved.Entity)

			i = next
			continue
		}

		if isDigit(c) {
			number, next, operand, ok := e.readExprNumber(raw, i, mode)
			if !ok {
				return emittedExpr{Go: exprZeroValue(mode)}
			}

			out.WriteString(number)
			operands = append(operands, operand)
			i = next
			continue
		}

		if op, size, ok := readExprOperator(raw, i); ok {
			if !validateExprOperatorSpacing(raw, i, size) {
				e.exprFail(mode, raw, "operator %q must be separated by spaces", op)
				return emittedExpr{Go: exprZeroValue(mode)}
			}

			switch {
			case isArithmeticOperator(op):
				flags.hasArithmetic = true
			case isComparisonOperator(op):
				flags.hasComparison = true
			case op == "&&" || op == "||" || op == "!":
				flags.hasBoolOp = true
			}

			out.WriteString(op)
			i += size
			continue
		}

		if isOperatorChar(c) {
			e.exprFail(mode, raw, "unknown or incomplete operator near %q", string(c))
			return emittedExpr{Go: exprZeroValue(mode)}
		}

		out.WriteByte(c)
		i++
	}

	resultType, ok := e.inferExprType(raw, mode, operands, flags)
	if !ok {
		return emittedExpr{Go: exprZeroValue(mode)}
	}

	return emittedExpr{
		Go:     out.String(),
		Type:   resultType,
		Entity: exprEntity,
	}
}

func (e *emitter) translateExprCall(raw string, name string, openIdx int, mode emitExprMode) (emittedExpr, int, bool) {
	closeIdx, ok := findMatchingParen(raw, openIdx)
	if !ok {
		e.exprFail(mode, raw, "unclosed function call %q", name)
		return emittedExpr{}, openIdx, false
	}

	innerRaw := raw[openIdx+1 : closeIdx]

	if targetType, ok := castValueType(name); ok {
		if !validateExprArgCount(innerRaw, 1) {
			e.exprFail(mode, raw, "cast %q expects exactly one argument", name)
			return emittedExpr{}, openIdx, false
		}

		arg := e.translateExpr(Expr(innerRaw), emitExprMode{
			Context:          mode.Context + " argument of " + name,
			AllowEntityField: mode.AllowEntityField,
			AllowEmpty:       false,
		})

		return emittedExpr{
			Go:     name + "(" + arg.Go + ")",
			Type:   targetType,
			Entity: arg.Entity,
		}, closeIdx + 1, true
	}

	if goName, argCount, ok := distributionFunc(name); ok {
		if !validateExprArgCount(innerRaw, argCount) {
			e.exprFail(mode, raw, "distribution %q expects %d arguments", name, argCount)
			return emittedExpr{}, openIdx, false
		}

		args := e.translateExpr(Expr(innerRaw), emitExprMode{
			Context:          mode.Context + " arguments of " + name,
			AllowEntityField: mode.AllowEntityField,
			AllowEmpty:       argCount == 0,
		})

		return emittedExpr{
			Go:     goName + "(" + args.Go + ")",
			Type:   ValueFloat64,
			Entity: args.Entity,
		}, closeIdx + 1, true
	}

	e.exprFail(mode, raw, "unknown function %q", name)
	return emittedExpr{}, openIdx, false
}

func (e *emitter) readExprName(raw string, start int, mode emitExprMode) (string, int, bool) {
	i := start

	if i >= len(raw) || !isIdentStart(raw[i]) {
		e.exprFail(mode, raw, "expected identifier")
		return "", start, false
	}

	i++
	for i < len(raw) && isIdentPart(raw[i]) {
		i++
	}

	if i < len(raw) && raw[i] == '.' {
		i++

		if i >= len(raw) || !isIdentStart(raw[i]) {
			e.exprFail(mode, raw, "expected identifier after dot")
			return "", start, false
		}

		i++
		for i < len(raw) && isIdentPart(raw[i]) {
			i++
		}

		if i < len(raw) && raw[i] == '.' {
			e.exprFail(mode, raw, "qualified name must contain exactly one dot")
			return "", start, false
		}
	}

	return raw[start:i], i, true
}

func (e *emitter) readExprNumber(raw string, start int, mode emitExprMode) (string, int, exprOperand, bool) {
	i := start

	for i < len(raw) && isDigit(raw[i]) {
		i++
	}

	isFloat := false
	if i < len(raw) && raw[i] == '.' {
		isFloat = true
		i++

		if i >= len(raw) || !isDigit(raw[i]) {
			e.exprFail(mode, raw, "expected digit after decimal point")
			return "", start, exprOperand{}, false
		}

		for i < len(raw) && isDigit(raw[i]) {
			i++
		}
	}

	number := raw[start:i]
	if isFloat {
		return number, i, exprOperand{
			Type:         ValueFloat64,
			UntypedNum:   true,
			FloatLiteral: true,
		}, true
	}

	return number, i, exprOperand{
		Type:       ValueInt64,
		UntypedNum: true,
	}, true
}

func (e *emitter) resolveExprName(raw string, name string, mode emitExprMode) resolvedExprName {
	if name == "true" || name == "false" {
		return resolvedExprName{Go: name, Type: ValueBool}
	}

	if v, ok := e.model.VarByName[name]; ok {
		return resolvedExprName{Go: varGoName(v), Type: v.Type}
	}

	if _, _, ok := distributionFunc(name); ok {
		e.exprFail(mode, raw, "distribution %q must be called as a function", name)
		return resolvedExprName{Go: "0", Type: ValueFloat64}
	}

	if _, ok := castValueType(name); ok {
		e.exprFail(mode, raw, "cast %q must be called as a function", name)
		return resolvedExprName{Go: "0", Type: ValueInt64}
	}

	parts := strings.Split(name, ".")
	if len(parts) == 2 {
		left := parts[0]
		right := parts[1]

		if left == "Simulation" && right == "clock" {
			return resolvedExprName{Go: "clock", Type: ValueFloat64}
		}

		if entity, ok := e.model.EntityByName[left]; ok {
			if !mode.AllowEntityField {
				e.exprFail(mode, raw, "entity field %q is not allowed here", name)
				return resolvedExprName{Go: "0", Type: ValueInt64}
			}

			field, ok := entity.FieldByName[right]
			if !ok {
				e.exprFail(mode, raw, "unknown entity field %q", name)
				return resolvedExprName{Go: "0", Type: ValueInt64}
			}

			return resolvedExprName{
				Go:     entityPoolName(entity) + ".Entities[ref.Idx]." + entityFieldName(field),
				Type:   field.Type,
				Entity: entity,
			}
		}

		if block, ok := e.model.BlockByName[left]; ok {
			if !hasMetricField(transportBlockMetricFields(block), right) {
				e.exprFail(mode, raw, "unknown block metric %q", name)
				return resolvedExprName{Go: "0", Type: ValueInt64}
			}

			return resolvedExprName{
				Go:   blockGoName(block) + "." + right,
				Type: ValueUint64,
			}
		}

		if resource, ok := e.model.ResourceByName[left]; ok {
			if !hasMetricField(transportResourceMetricFields(), right) {
				e.exprFail(mode, raw, "unknown resource metric %q", name)
				return resolvedExprName{Go: "0", Type: ValueInt64}
			}

			return resolvedExprName{
				Go:   resourceGoName(resource) + "." + right,
				Type: ValueUint64,
			}
		}

		e.exprFail(mode, raw, "unknown qualified name %q", name)
		return resolvedExprName{Go: "0", Type: ValueInt64}
	}

	if name == "clock" {
		e.exprFail(mode, raw, "use Simulation.clock instead of clock")
		return resolvedExprName{Go: "0", Type: ValueFloat64}
	}

	e.exprFail(mode, raw, "unknown identifier %q", name)
	return resolvedExprName{Go: "0", Type: ValueInt64}
}

func (e *emitter) inferExprType(raw string, mode emitExprMode, operands []exprOperand, flags exprFlags) (ValueType, bool) {
	if len(operands) == 0 {
		e.exprFail(mode, raw, "expression does not contain operands")
		return ValueInt64, false
	}

	if flags.hasComparison || flags.hasBoolOp {
		return ValueBool, true
	}

	hasBool := false
	hasConcrete := false
	hasFloatLiteral := false
	concreteType := ValueType(0)

	for _, operand := range operands {
		if operand.Type == ValueBool {
			hasBool = true
			continue
		}

		if operand.FloatLiteral {
			hasFloatLiteral = true
		}

		if operand.UntypedNum {
			continue
		}

		if !isNumericValueType(operand.Type) {
			e.exprFail(mode, raw, "unsupported operand type %s", valueTypeName(operand.Type))
			return ValueInt64, false
		}

		if !hasConcrete {
			hasConcrete = true
			concreteType = operand.Type
			continue
		}

		if concreteType != operand.Type {
			e.exprFail(mode, raw, "mixed numeric types require explicit cast")
			return ValueInt64, false
		}
	}

	if hasBool {
		if len(operands) == 1 {
			return ValueBool, true
		}

		e.exprFail(mode, raw, "bool operand cannot be used as numeric expression")
		return ValueBool, false
	}

	if mode.HasExpected && isNumericValueType(mode.Expected) {
		if hasFloatLiteral && mode.Expected != ValueFloat64 {
			e.exprFail(mode, raw, "float literal cannot be used as %s without explicit cast", valueTypeName(mode.Expected))
			return mode.Expected, false
		}

		if hasConcrete && concreteType != mode.Expected {
			e.exprFail(
				mode,
				raw,
				"expected %s, got %s. Use explicit cast",
				valueTypeName(mode.Expected),
				valueTypeName(concreteType),
			)
			return mode.Expected, false
		}

		return mode.Expected, true
	}

	if hasConcrete {
		return concreteType, true
	}

	if hasFloatLiteral {
		return ValueFloat64, true
	}

	return ValueInt64, true
}

func (e *emitter) mergeExprEntity(raw string, mode emitExprMode, current *EntityDesc, next *EntityDesc) *EntityDesc {
	if next == nil {
		return current
	}

	if current == nil {
		return next
	}

	if current != next {
		e.exprFail(
			mode,
			raw,
			"expression references fields of multiple entity types: %q and %q",
			current.Name,
			next.Name,
		)
		return current
	}

	return current
}

func readExprOperator(raw string, pos int) (string, int, bool) {
	if pos+2 <= len(raw) {
		switch raw[pos : pos+2] {
		case "&&", "||", "==", "!=", ">=", "<=":
			return raw[pos : pos+2], 2, true
		}
	}

	switch raw[pos] {
	case '+', '-', '*', '/', '>', '<', '!':
		return raw[pos : pos+1], 1, true
	default:
		return "", 0, false
	}
}

func validateExprOperatorSpacing(raw string, pos int, size int) bool {
	if pos > 0 {
		prev := raw[pos-1]
		if !isSpace(prev) && prev != '(' && prev != ',' {
			return false
		}
	}

	end := pos + size
	if end >= len(raw) {
		return false
	}

	next := raw[end]
	return isSpace(next) || next == '('
}

func findMatchingParen(raw string, openIdx int) (int, bool) {
	if openIdx >= len(raw) || raw[openIdx] != '(' {
		return 0, false
	}

	depth := 0
	for i := openIdx; i < len(raw); i++ {
		switch raw[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i, true
			}
		}
	}

	return 0, false
}

func validateExprArgCount(raw string, expected int) bool {
	raw = strings.TrimSpace(raw)
	if expected == 0 {
		return raw == ""
	}

	if raw == "" {
		return false
	}

	count := 1
	depth := 0

	for i := 0; i < len(raw); i++ {
		switch raw[i] {
		case '(':
			depth++
		case ')':
			if depth == 0 {
				return false
			}
			depth--
		case ',':
			if depth == 0 {
				count++
			}
		}
	}

	return count == expected
}

func nextNonSpace(raw string, pos int) int {
	for pos < len(raw) && isSpace(raw[pos]) {
		pos++
	}
	return pos
}

func isOperatorChar(c byte) bool {
	switch c {
	case '+', '-', '*', '/', '>', '<', '!', '=', '&', '|':
		return true
	default:
		return false
	}
}

func isArithmeticOperator(op string) bool {
	return op == "+" || op == "-" || op == "*" || op == "/"
}

func isComparisonOperator(op string) bool {
	switch op {
	case "==", "!=", ">", ">=", "<", "<=":
		return true
	default:
		return false
	}
}

func isNumericValueType(t ValueType) bool {
	return t == ValueFloat64 || t == ValueInt64 || t == ValueUint64
}

func exprTypesCompatible(actual ValueType, expected ValueType) bool {
	if actual == expected {
		return true
	}

	return false
}

func castValueType(name string) (ValueType, bool) {
	switch name {
	case "float64":
		return ValueFloat64, true
	case "int64":
		return ValueInt64, true
	case "uint64":
		return ValueUint64, true
	default:
		return 0, false
	}
}

func distributionFunc(name string) (string, int, bool) {
	switch name {
	case "uniform":
		return "rng.Uniform", 2, true
	case "triangular":
		return "rng.Triangular", 3, true
	case "exponential":
		return "rng.Exponential", 1, true
	case "normal":
		return "rng.Normal", 2, true
	default:
		return "", 0, false
	}
}

func hasMetricField(fields []string, field string) bool {
	for _, candidate := range fields {
		if candidate == field {
			return true
		}
	}
	return false
}

func valueTypeName(t ValueType) string {
	switch t {
	case ValueFloat64:
		return "float64"
	case ValueInt64:
		return "int64"
	case ValueUint64:
		return "uint64"
	case ValueBool:
		return "bool"
	default:
		return "unknown"
	}
}

func exprZeroValue(mode emitExprMode) string {
	if mode.HasExpected {
		switch mode.Expected {
		case ValueFloat64:
			return "0.0"
		case ValueBool:
			return "false"
		default:
			return "0"
		}
	}

	return "0"
}

func (e *emitter) exprFail(mode emitExprMode, raw string, format string, args ...any) {
	context := mode.Context
	if context == "" {
		context = "expression"
	}

	msg := fmt.Sprintf(format, args...)
	if raw == "" {
		e.fail("%s: %s", context, msg)
		return
	}

	e.fail("%s: %s in expression %q", context, msg, raw)
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n'
}
