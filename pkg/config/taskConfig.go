package config

import (
	"encoding/json"
	"fmt"
)

type TaskConfig struct {
	Enabled  bool
	Schedule string
	Metadata map[string]interface{}
}

func (tc *TaskConfig) ConvertMetadataToStruct(target interface{}) error {
	if tc.Metadata == nil {
		return nil // No metadata to convert, target remains with zero values
	}

	// Convert map to JSON and then to struct
	// This handles type conversions automatically
	jsonData, err := json.Marshal(tc.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata to JSON: %w", err)
	}

	if err := json.Unmarshal(jsonData, target); err != nil {
		return fmt.Errorf("failed to unmarshal JSON to struct: %w", err)
	}

	return nil
}
