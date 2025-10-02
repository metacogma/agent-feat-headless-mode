package edcdetails

type EDCDetails struct {
	Site           string   `json:"site" bson:"site"`
	SubjectNumbers []string `json:"subject_numbers" bson:"subject_numbers"`
}
