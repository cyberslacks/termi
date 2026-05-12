package playbook

import (
	"bytes"
	"fmt"
	"text/template"
)

// Interpolate expands {{ .VAR_NAME }} template syntax in a command string.
func Interpolate(command string, vars map[string]string) (string, error) {
	tmpl, err := template.New("cmd").Option("missingkey=error").Parse(command)
	if err != nil {
		return "", fmt.Errorf("parse command template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return "", fmt.Errorf("interpolate command: %w", err)
	}
	return buf.String(), nil
}

// MergeVars builds a variable map from playbook defaults + caller overrides.
func MergeVars(defs []Variable, overrides map[string]string) (map[string]string, error) {
	result := make(map[string]string, len(defs))
	for _, v := range defs {
		result[v.Name] = v.Default
	}
	for k, val := range overrides {
		result[k] = val
	}
	// Check required vars
	for _, v := range defs {
		if v.Required && result[v.Name] == "" {
			return nil, fmt.Errorf("required variable %q is not set", v.Name)
		}
	}
	return result, nil
}
