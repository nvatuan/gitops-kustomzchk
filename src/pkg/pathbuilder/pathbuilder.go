package pathbuilder

import (
	"fmt"
	"regexp"
	"strings"
)

// PathBuilder interpolates variables into build path templates
type PathBuilder struct {
	Template  string              // e.g., "/path/$SERVICE/clusters/$CLUSTER/$ENV"
	Variables map[string][]string // e.g., {"SERVICE": ["my-app"], "CLUSTER": ["alpha","beta"], ...}
}

// PathCombination represents a single interpolated path with its variable values
type PathCombination struct {
	Path       string            // Full interpolated path (relative to root)
	Values     map[string]string // Variable values used
	OverlayKey string            // Key for reports (e.g., "alpha/stg")
}

// variablePattern matches [VARIABLE_NAME] patterns (alphanumeric and underscores)
// Using brackets instead of $ to avoid bash variable expansion issues
var variablePattern = regexp.MustCompile(`\[([A-Za-z_][A-Za-z0-9_]*)\]`)

// NewPathBuilder creates a new PathBuilder from template and values strings
func NewPathBuilder(template, valuesStr string) (*PathBuilder, error) {
	variables, err := ParseValues(valuesStr)
	if err != nil {
		return nil, err
	}
	return &PathBuilder{
		Template:  template,
		Variables: variables,
	}, nil
}

// ParseTemplate extracts $VARIABLE names from the template
func ParseTemplate(template string) []string {
	matches := variablePattern.FindAllStringSubmatch(template, -1)
	seen := make(map[string]bool)
	var vars []string
	for _, match := range matches {
		if len(match) > 1 && !seen[match[1]] {
			seen[match[1]] = true
			vars = append(vars, match[1])
		}
	}
	return vars
}

// ParseValues parses "KEY=v1,v2;KEY2=v3" format into a map
func ParseValues(valuesStr string) (map[string][]string, error) {
	result := make(map[string][]string)
	if valuesStr == "" {
		return result, nil
	}

	tokens := strings.Split(valuesStr, ";")
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}

		parts := strings.SplitN(token, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid token format: %q, expected KEY=value1,value2", token)
		}

		key := strings.TrimSpace(parts[0])
		if key == "" {
			return nil, fmt.Errorf("empty key in token: %q", token)
		}

		valuesStr := strings.TrimSpace(parts[1])
		if valuesStr == "" {
			return nil, fmt.Errorf("empty value for key: %q", key)
		}

		values := strings.Split(valuesStr, ",")
		for i, v := range values {
			values[i] = strings.TrimSpace(v)
		}
		result[key] = values
	}

	return result, nil
}

// Validate checks that all $VARs in template have corresponding values
func (pb *PathBuilder) Validate() error {
	if pb.Template == "" {
		return fmt.Errorf("template cannot be empty")
	}

	templateVars := ParseTemplate(pb.Template)
	for _, varName := range templateVars {
		values, exists := pb.Variables[varName]
		if !exists {
			return fmt.Errorf("variable $%s in template has no values defined", varName)
		}
		if len(values) == 0 {
			return fmt.Errorf("variable $%s has empty values", varName)
		}
	}

	return nil
}

// InterpolatePath performs single interpolation with given values
func (pb *PathBuilder) InterpolatePath(values map[string]string) (string, error) {
	result := pb.Template
	for varName, value := range values {
		result = strings.ReplaceAll(result, "["+varName+"]", value)
	}

	// Check if any unresolved variables remain
	if variablePattern.MatchString(result) {
		unresolved := variablePattern.FindAllString(result, -1)
		return "", fmt.Errorf("unresolved variables in path: %v", unresolved)
	}

	return result, nil
}

// GenerateAllPaths generates all path combinations using Cartesian product
func (pb *PathBuilder) GenerateAllPaths() ([]PathCombination, error) {
	if err := pb.Validate(); err != nil {
		return nil, err
	}

	templateVars := ParseTemplate(pb.Template)
	if len(templateVars) == 0 {
		// No variables, return template as-is
		return []PathCombination{{
			Path:       pb.Template,
			Values:     map[string]string{},
			OverlayKey: pb.Template,
		}}, nil
	}

	// Generate Cartesian product
	combinations := pb.cartesianProduct(templateVars)

	var results []PathCombination
	for _, combo := range combinations {
		path, err := pb.InterpolatePath(combo)
		if err != nil {
			return nil, err
		}

		overlayKey := pb.generateOverlayKey(templateVars, combo)

		results = append(results, PathCombination{
			Path:       path,
			Values:     combo,
			OverlayKey: overlayKey,
		})
	}

	return results, nil
}

// cartesianProduct generates all combinations of variable values
// Variables are processed in the order they appear in the template
func (pb *PathBuilder) cartesianProduct(varNames []string) []map[string]string {
	if len(varNames) == 0 {
		return []map[string]string{{}}
	}

	// Use varNames as-is to preserve template order
	var results []map[string]string
	pb.generateCombinations(varNames, 0, make(map[string]string), &results)
	return results
}

func (pb *PathBuilder) generateCombinations(varNames []string, index int, current map[string]string, results *[]map[string]string) {
	if index == len(varNames) {
		// Make a copy of current map
		combo := make(map[string]string)
		for k, v := range current {
			combo[k] = v
		}
		*results = append(*results, combo)
		return
	}

	varName := varNames[index]
	values := pb.Variables[varName]
	for _, value := range values {
		current[varName] = value
		pb.generateCombinations(varNames, index+1, current, results)
	}
	delete(current, varName)
}

// generateOverlayKey creates a key for reports based on variable values
// Uses variable values in template order joined by "/"
func (pb *PathBuilder) generateOverlayKey(varNames []string, values map[string]string) string {
	// Use template order (varNames is already in template order)
	var parts []string
	for _, varName := range varNames {
		if value, ok := values[varName]; ok {
			parts = append(parts, value)
		}
	}

	return strings.Join(parts, "/")
}

// GetRelativePaths returns all relative paths for sparse checkout
func (pb *PathBuilder) GetRelativePaths() ([]string, error) {
	combos, err := pb.GenerateAllPaths()
	if err != nil {
		return nil, err
	}

	paths := make([]string, len(combos))
	for i, combo := range combos {
		paths[i] = combo.Path
	}
	return paths, nil
}

