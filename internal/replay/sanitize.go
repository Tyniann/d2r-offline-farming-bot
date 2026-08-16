package replay

import (
	"encoding/json"
	"fmt"
	"regexp"
)

const redactedValue = "[redacted]"

var (
	forbiddenTraceKey = regexp.MustCompile(`(?i)(?:^|_)(?:raw|pointer|address|module_base|process_handle|save|path|token|secret|password|authorization|api_key)(?:_|$)`)
	windowsUserPath   = regexp.MustCompile(`(?i)[a-z]:[\\/]+users[\\/]+[^\\/\s\"']+(?:[\\/][^\s\"']*)?`)
	secretAssignment  = regexp.MustCompile(`(?i)(token|secret|password|authorization|api[_-]?key)\s*[:=]\s*[^\s,;]+`)
)

// SanitizeMap returns a deep JSON-compatible copy with sensitive keys and
// values redacted. Unsupported values become a stable type marker.
func SanitizeMap(value map[string]any) map[string]any {
	if len(value) == 0 {
		return nil
	}
	clean := make(map[string]any, len(value))
	for key, item := range value {
		if forbiddenTraceKey.MatchString(key) {
			clean[key] = redactedValue
			continue
		}
		clean[key] = sanitizeValue(item)
	}
	return clean
}

func sanitizeValue(value any) any {
	switch typed := value.(type) {
	case nil, bool, float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return typed
	case string:
		return sanitizeText(typed)
	case map[string]any:
		return SanitizeMap(typed)
	case []any:
		clean := make([]any, len(typed))
		for index := range typed {
			clean[index] = sanitizeValue(typed[index])
		}
		return clean
	default:
		encoded, err := json.Marshal(value)
		if err != nil {
			return fmt.Sprintf("[unsupported:%T]", value)
		}
		var decoded any
		if err := json.Unmarshal(encoded, &decoded); err != nil {
			return fmt.Sprintf("[unsupported:%T]", value)
		}
		return sanitizeValue(decoded)
	}
}

func sanitizeText(value string) string {
	value = windowsUserPath.ReplaceAllString(value, redactedValue)
	value = secretAssignment.ReplaceAllString(value, redactedValue)
	return value
}

// ValidateSafeJSON rejects any bundle that still contains prohibited schema
// keys or obvious user paths/secrets after sanitization.
func ValidateSafeJSON(encoded []byte) error {
	var value any
	if err := json.Unmarshal(encoded, &value); err != nil {
		return fmt.Errorf("decode runtime trace safety view: %w", err)
	}
	return validateSafeValue(value, "$")
}

func validateSafeValue(value any, location string) error {
	switch typed := value.(type) {
	case map[string]any:
		for key, item := range typed {
			if forbiddenTraceKey.MatchString(key) && item != redactedValue {
				return fmt.Errorf("runtime trace contains prohibited field %s.%s", location, key)
			}
			if err := validateSafeValue(item, location+"."+key); err != nil {
				return err
			}
		}
	case []any:
		for index, item := range typed {
			if err := validateSafeValue(item, fmt.Sprintf("%s[%d]", location, index)); err != nil {
				return err
			}
		}
	case string:
		if windowsUserPath.MatchString(typed) || secretAssignment.MatchString(typed) {
			return fmt.Errorf("runtime trace contains sensitive text at %s", location)
		}
	}
	return nil
}
