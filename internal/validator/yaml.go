package validator

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
)

// ValidateYAMLFile reads a file and checks if its content is valid YAML.
// It returns an error if the file cannot be read or if the YAML is invalid.
func ValidateYAMLFile(filePath string) error {
	// Read the file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file '%s': %w", filePath, err)
	}

	// Attempt to unmarshal the YAML content.
	// We unmarshal into an interface{} because we only care about syntax, not structure.
	var out interface{}
	err = yaml.Unmarshal(content, &out)
	if err != nil {
		// yaml.v3 provides good error messages, often including line numbers
		return fmt.Errorf("invalid YAML in '%s': %w", filePath, err)
	}

	// If unmarshal was successful, the YAML is valid
	return nil
}
