package stats

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

var attrKeyPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// ParseAttrs parses attributes from CLI flags and ENV.
// CLI flags take precedence over ENV.
// ENV format: "key1=val1,key2=val2"
func ParseAttrs(cliAttrs []string, envAttr string) map[string]string {
	attrs := make(map[string]string)

	// Parse ENV first
	if envAttr != "" {
		for _, pair := range strings.Split(envAttr, ",") {
			pair = strings.TrimSpace(pair)
			if pair == "" {
				continue
			}
			k, v, ok := strings.Cut(pair, "=")
			if ok {
				k = strings.TrimSpace(k)
				v = strings.TrimSpace(v)
				if k != "" && v != "" {
					attrs[k] = v
				}
			}
		}
	}

	// Override with CLI flags
	for _, pair := range cliAttrs {
		k, v, ok := strings.Cut(pair, "=")
		if ok {
			k = strings.TrimSpace(k)
			v = strings.TrimSpace(v)
			if k != "" && v != "" {
				attrs[k] = v
			}
		}
	}

	if len(attrs) == 0 {
		return nil
	}
	return attrs
}

// ValidateAttr validates a single attribute key and value.
func ValidateAttr(key, value string) error {
	if key == "" {
		return fmt.Errorf("attribute key must not be empty")
	}
	if value == "" {
		return fmt.Errorf("attribute value must not be empty")
	}
	if !attrKeyPattern.MatchString(key) {
		return fmt.Errorf("attribute key must match [a-zA-Z0-9_-], got %q", key)
	}
	return nil
}

// ValidateAttrs validates all attributes.
func ValidateAttrs(attrs map[string]string) error {
	for k, v := range attrs {
		if err := ValidateAttr(k, v); err != nil {
			return err
		}
	}
	return nil
}

// GetXSQLAttrEnv returns the value of XSQL_ATTR environment variable.
func GetXSQLAttrEnv() string {
	return os.Getenv("XSQL_ATTR")
}
