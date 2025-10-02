package integrations

type IntegrationModel struct {
	Integrations Integrations `json:"integrations,omitempty" bson:"integrations"`
}

type Integrations struct {
	TestLabs TestLabs `json:"test_labs,omitempty" bson:"test_labs,omitempty"`
	Edc      Edc      `json:"edc,omitempty" bson:"edc,omitempty"`
}

type TestLabs struct {
	BrowserStack BrowserStack `json:"browser_stack,omitempty" bson:"browser_stack,omitempty"`
	SauceLabs    SauceLabs    `json:"sauce_labs,omitempty" bson:"sauce_labs,omitempty"`
	LambdaTest   LambdaTest   `json:"lambda_test,omitempty" bson:"lambda_test,omitempty"`
}

type BrowserStack struct {
	Enabled   bool   `json:"enabled,omitempty" bson:"enabled,omitempty"`
	Username  string `json:"username,omitempty" bson:"username,omitempty"`
	AccessKey string `json:"access_key,omitempty" bson:"access_key,omitempty"`
}

type SauceLabs struct {
	Enabled   bool   `json:"enabled,omitempty" bson:"enabled,omitempty"`
	Username  string `json:"username,omitempty" bson:"username,omitempty"`
	AccessKey string `json:"access_key,omitempty" bson:"access_key,omitempty"`
}

type LambdaTest struct {
	Enabled   bool   `json:"enabled,omitempty" bson:"enabled,omitempty"`
	Username  string `json:"username,omitempty" bson:"username,omitempty"`
	AccessKey string `json:"access_key,omitempty" bson:"access_key,omitempty"`
}

type Edc struct {
	VeevaVault VeevaVault `json:"veeva_vault,omitempty" bson:"veeva_vault,omitempty"`
}

type VeevaVault struct {
	Version       string `json:"version,omitempty" bson:"version,omitempty"`
	Dns           string `json:"dns,omitempty" bson:"dns,omitempty"`
	Study_name    string `json:"study_name,omitempty" bson:"study_name,omitempty"`
	Study_country string `json:"study_country,omitempty" bson:"study_country,omitempty"`
	Site_name     string `json:"site_name,omitempty" bson:"site_name,omitempty"`
	Subject_name  string `json:"subject_name,omitempty" bson:"subject_name,omitempty"`
	User_name     string `json:"user_name,omitempty" bson:"user_name,omitempty"`
	Password      string `json:"password,omitempty" bson:"password,omitempty"`
	LoginUrl      string `json:"login_url,omitempty" bson:"login_url,omitempty"`
}
