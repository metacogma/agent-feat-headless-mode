package testsuite

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TestSuite struct {
	ID             primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	Title          string             `json:"title" bson:"title"`
	TestcasesCount int64              `json:"testcases_count" bson:"testcases_count"`
	CreatedBy      string             `json:"created_by" bson:"created_by"`
	CreatedOn      *time.Time         `json:"created_on" bson:"created_on"`
}
