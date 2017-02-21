package merge

// Match pairs up to FHIR resources that "match". These resources should be of the same
// resource type (e.g. Patient).
type Match struct {
	ResourceType string
	Left         interface{}
	Right        interface{}
}
