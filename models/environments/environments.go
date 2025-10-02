package environments

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"agent/errors"
)

type Variables map[string]string

type Environment struct {
	ID        primitive.ObjectID `json:"_id" bson:"_id,omitempty"`
	Name      string             `json:"name" bson:"name,omitempty"`
	CreatedBy string             `json:"created_by" bson:"created_by,omitempty"`
	CreatedAt time.Time          `json:"created_at" bson:"created_at,omitempty"`
	UpdatedBy string             `json:"updated_by" bson:"updated_by,omitempty"`
	UpdatedAt time.Time          `json:"updated_at" bson:"updated_at,omitempty"`
	Variables Variables          `json:"variables" bson:"variables"`
}

func (t *Environment) Validate() error {
	ve := errors.ValidationErrs()
	if t.Name == "" {
		ve.Add("name", "cannot be empty")
	}
	if len(t.Variables) == 0 {
		ve.Add("variables", "cannot be empty")
	}
	return ve.Err()
}
