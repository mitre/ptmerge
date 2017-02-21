package merge

import (
	"fmt"
	"reflect"
	"strings"

	"gopkg.in/mgo.v2/bson"

	"github.com/intervention-engine/fhir/models"
)

// ConflictStrategies maps a custom conflict detection strategy to a specific resource type.
// For example, "Patient": &PatientConflictStrategy{}. To be used by Detector.FindConflicts()
// any new custom conflict detection strategy must be registered here.
var ConflictStrategies = map[string]ConflictStrategy{}

// Detector provides tools for identifying all conflicts between 2 resources in a Match.
type Detector struct{}

// FindConflicts identifies all conflicts between a set of Matches, returning an OperationOutcome for
// each match detailing the location(s) of conflicts between the resources in the Match.
func (d *Detector) FindConflicts(matches []Match) (conflicts []models.OperationOutcome, err error) {

	// First, iterate through all of the matches to determine what conflicts, if any exist.
	for _, match := range matches {
		strategy, ok := ConflictStrategies[match.ResourceType]
		if !ok {
			// No known conflict strategy exists for this resource type. To be safe, we mark every
			// non-nil field in either resource as a conflict and leave it up to the user to resolve them.
			locations := []string{}
			value := reflect.ValueOf(match.Left)
			traverse(&locations, value, value.Type().Name())
			conflicts = append(conflicts, *createConflictOperationOutcome(locations))
			continue
		}

		locations, err := strategy.FindConflicts(match)
		if err != nil {
			return nil, err
		}

		// If there is no error and locations is empty, then the match was a "perfect" match, with no conflicts.
		if len(locations) > 0 {
			conflicts = append(conflicts, *createConflictOperationOutcome(locations))
		}
	}
	return conflicts, nil
}

// traverse recursively iterates through all non-nil fields in the resource, identifying the JSON paths
// to the non-nil fields. These paths are then used to identify the location of any conflicts.
func traverse(paths *[]string, value reflect.Value, path string) {
	switch value.Kind() {
	case reflect.Ptr, reflect.Interface:
		// To get the actual value of the object or interface being pointed to we use Elem().
		val := value.Elem()
		// Check if the pointer or interface is nil.
		if !val.IsValid() {
			return
		}
		// Traverse the object that's being pointed to.
		traverse(paths, val, path)

	case reflect.Struct:
		// We don't traverse into time objects.
		if value.Type().Name() == "Time" {
			*paths = append(*paths, path)
			return
		}

		// Traverse all non-nil fields in the struct, building up their json paths.
		for i := 0; i < value.NumField(); i++ {
			jsonPath := value.Type().Field(i).Tag.Get("json")
			// jsonPath will be empty for inline resourced (e.g. DomainResource).
			if jsonPath != "" {
				prefix := ""
				// The path is empty if we're currently traversing the top-level object (e.g. Patient).
				if path != "" {
					prefix = path + "."
				}
				// Splits into the name of the field (e.g. "gender") and the "omitempty" flag.
				parts := strings.SplitN(jsonPath, ",", 2)
				traverse(paths, value.Field(i), prefix+parts[0])
			} else {
				// This was an inline resource, so we shouldn't add it to the path. Just traverse
				// it's fields instead.
				traverse(paths, value.Field(i), path)
			}
		}

	case reflect.Slice:
		// Traverse all elements in the slice.
		for i := 0; i < value.Len(); i++ {
			traverse(paths, value.Index(i), path+fmt.Sprintf("[%d]", i))
		}

	case reflect.String:
		// Check that the string isn't nil.
		val := value.String()
		if val != "" {
			*paths = append(*paths, path)
		}

	default:
		// These are all of the other primitive types (e.g. int, float, bool).
		*paths = append(*paths, path)
	}
}

func createConflictOperationOutcome(locations []string) *models.OperationOutcome {
	outcome := &models.OperationOutcome{
		DomainResource: models.DomainResource{
			Resource: models.Resource{
				Id:           bson.NewObjectId().Hex(),
				ResourceType: "OperationOutcome",
			},
		},
		Issue: []models.OperationOutcomeIssueComponent{
			models.OperationOutcomeIssueComponent{
				Severity:    "information",
				Code:        "conflict",
				Location:    locations,
				Diagnostics: "",
			},
		},
	}

	return outcome
}
