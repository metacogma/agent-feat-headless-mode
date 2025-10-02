package replacements

import (
	"github.com/samber/lo"

	"agent/errors"
)

type Replacement struct {
	Type      string `bson:"type" json:"type"`
	Value     string `bson:"value" json:"value"`
	ValueType string `bson:"value_type" json:"value_type"`
	Key       string `bson:"key" json:"key"`
}

func (t *Replacement) Validate() error {
	ve := errors.ValidationErrs()
	if t.Type == "" {
		ve.Add("type", "cannot be empty")
	}
	if !lo.Contains([]string{"identifier", "value", "text"}, t.Type) {
		ve.Add("type", "invalid value")
	}
	if t.ValueType == "" {
		ve.Add("Value Type", "cannot be empty")
	}
	if !lo.Contains([]string{"raw", "environment", "data_profile", "element"}, t.ValueType) {
		ve.Add("Value Type", "cannot be empty")
	}
	return ve.Err()
}
