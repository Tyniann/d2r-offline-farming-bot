package app

import (
	"fmt"
	"strconv"
	"strings"
)

type inputTestActionKind string

const (
	inputTestBelt        inputTestActionKind = "belt"
	inputTestPortal      inputTestActionKind = "portal"
	inputTestSkill       inputTestActionKind = "skill"
	inputTestCenterClick inputTestActionKind = "center_click"
	inputTestClick       inputTestActionKind = "click"
)

type inputTestAction struct {
	kind inputTestActionKind
	slot int
	x    int
	y    int
}

func (a inputTestAction) String() string {
	switch a.kind {
	case inputTestBelt:
		return fmt.Sprintf("belt:%d", a.slot)
	case inputTestPortal:
		return "portal"
	case inputTestSkill:
		return fmt.Sprintf("skill:%d", a.slot)
	case inputTestCenterClick:
		return "center-click"
	case inputTestClick:
		return fmt.Sprintf("click:%d,%d", a.x, a.y)
	default:
		return string(a.kind)
	}
}

func parseInputTestSpec(spec string) ([]inputTestAction, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, fmt.Errorf("input test spec is empty")
	}

	parts := splitInputTestTokens(spec)
	actions := make([]inputTestAction, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("input test spec contains empty segment in %q", spec)
		}
		action, err := parseInputTestAction(part)
		if err != nil {
			return nil, err
		}
		actions = append(actions, action)
	}
	return actions, nil
}

// splitInputTestTokens splits a comma-separated action list while keeping click:X,Y coordinates intact.
func splitInputTestTokens(spec string) []string {
	raw := strings.Split(spec, ",")
	tokens := make([]string, 0, len(raw))
	for i := 0; i < len(raw); i++ {
		part := strings.TrimSpace(raw[i])
		name, arg, hasArg := strings.Cut(part, ":")
		if name == "click" && hasArg && !strings.Contains(arg, ",") && i+1 < len(raw) {
			part = part + "," + strings.TrimSpace(raw[i+1])
			i++
		}
		tokens = append(tokens, part)
	}
	return tokens
}

func parseInputTestAction(token string) (inputTestAction, error) {
	name, arg, hasArg := strings.Cut(token, ":")
	name = strings.TrimSpace(name)
	switch name {
	case "belt", "potion":
		if !hasArg {
			return inputTestAction{}, fmt.Errorf("input test action %q requires slot (e.g. belt:1)", token)
		}
		slot, err := strconv.Atoi(strings.TrimSpace(arg))
		if err != nil {
			return inputTestAction{}, fmt.Errorf("input test action %q: invalid slot: %w", token, err)
		}
		if slot < 1 || slot > 4 {
			return inputTestAction{}, fmt.Errorf("input test action %q: belt/potion slot must be 1..4", token)
		}
		return inputTestAction{kind: inputTestBelt, slot: slot}, nil
	case "portal":
		if hasArg {
			return inputTestAction{}, fmt.Errorf("input test action %q does not take arguments", token)
		}
		return inputTestAction{kind: inputTestPortal}, nil
	case "skill":
		if !hasArg {
			return inputTestAction{}, fmt.Errorf("input test action %q requires slot (e.g. skill:1)", token)
		}
		slot, err := strconv.Atoi(strings.TrimSpace(arg))
		if err != nil {
			return inputTestAction{}, fmt.Errorf("input test action %q: invalid slot: %w", token, err)
		}
		if slot < 1 || slot > 8 {
			return inputTestAction{}, fmt.Errorf("input test action %q: skill slot must be 1..8", token)
		}
		return inputTestAction{kind: inputTestSkill, slot: slot}, nil
	case "center-click":
		if hasArg {
			return inputTestAction{}, fmt.Errorf("input test action %q does not take arguments", token)
		}
		return inputTestAction{kind: inputTestCenterClick}, nil
	case "click":
		if !hasArg {
			return inputTestAction{}, fmt.Errorf("input test action %q requires coordinates (e.g. click:640,360)", token)
		}
		coords := strings.Split(arg, ",")
		if len(coords) != 2 {
			return inputTestAction{}, fmt.Errorf("input test action %q: expected click:X,Y", token)
		}
		x, err := strconv.Atoi(strings.TrimSpace(coords[0]))
		if err != nil {
			return inputTestAction{}, fmt.Errorf("input test action %q: invalid X: %w", token, err)
		}
		y, err := strconv.Atoi(strings.TrimSpace(coords[1]))
		if err != nil {
			return inputTestAction{}, fmt.Errorf("input test action %q: invalid Y: %w", token, err)
		}
		return inputTestAction{kind: inputTestClick, x: x, y: y}, nil
	default:
		return inputTestAction{}, fmt.Errorf(
			"unknown input test action %q; allowed examples: belt:1, potion:1, portal, skill:1, center-click, click:640,360",
			token,
		)
	}
}
