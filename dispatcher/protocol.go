package main

import (
	"bufio"
	"fmt"
	"strings"
)

func scanNonEmptyLine(scanner *bufio.Scanner) (string, bool) {
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			return line, true
		}
	}

	return "", false
}

func parseHello(line string) (string, error) {
	fields := strings.Fields(line)
	if len(fields) != 2 || fields[0] != "HELLO" {
		return "", fmt.Errorf("expected HELLO <model_hash>, got %q", line)
	}
	if len(fields[1]) != 64 || !isHex(fields[1]) {
		return "", fmt.Errorf("invalid HELLO model hash %q", fields[1])
	}
	return fields[1], nil
}

func parseStatLine(line string) (map[string]string, error) {
	raw := strings.TrimSpace(strings.TrimPrefix(line, "STAT"))
	if raw == "" {
		return map[string]string{}, nil
	}

	result := make(map[string]string)
	parts := strings.Split(raw, ";")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		name, value, ok := strings.Cut(part, "=")
		if !ok {
			return nil, fmt.Errorf("bad metric pair %q", part)
		}

		name = strings.TrimSpace(name)
		value = strings.TrimSpace(value)

		if name == "" {
			return nil, fmt.Errorf("empty metric name in %q", part)
		}

		result[name] = value
	}

	return result, nil
}

func parseVarOverrides(values []string) ([]SetVar, error) {
	result := make([]SetVar, 0, len(values))

	for _, raw := range values {
		name, value, ok := strings.Cut(raw, "=")
		if !ok {
			return nil, fmt.Errorf("bad set %q, expected name=value", raw)
		}

		name = strings.TrimSpace(name)
		value = strings.TrimSpace(value)

		if name == "" {
			return nil, fmt.Errorf("empty variable name in set %q", raw)
		}
		if value == "" {
			return nil, fmt.Errorf("empty variable value in set %q", raw)
		}

		result = append(result, SetVar{Name: name, Value: value})
	}

	return result, nil
}

func parseStopRule(raw string) (*StopRule, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	fields := strings.Fields(raw)
	if len(fields) != 3 {
		return nil, fmt.Errorf("bad stop-rule %q, expected '<left> <op> <right>'", raw)
	}

	switch fields[1] {
	case "==", "!=", ">", ">=", "<", "<=":
	default:
		return nil, fmt.Errorf("bad stop-rule operator %q", fields[1])
	}

	return &StopRule{
		Left:  fields[0],
		Op:    fields[1],
		Right: fields[2],
	}, nil
}

func normalizeMetrics(raw string) []string {
	seen := map[string]bool{
		metricClock: true,
	}

	result := []string{metricClock}

	for _, part := range strings.Split(raw, ";") {
		name := strings.TrimSpace(part)
		if name == "" {
			continue
		}

		if seen[name] {
			continue
		}

		seen[name] = true
		result = append(result, name)
	}

	return result
}
