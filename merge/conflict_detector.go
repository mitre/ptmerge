package merge

import (
	"reflect"
	"time"

	"gopkg.in/mgo.v2/bson"

	"github.com/intervention-engine/fhir/models"
)

// Detector provides tools for detecting all conflicts between 2 resources in a Match.
type Detector struct{}

// AllConflicts identifies all conflicts between a set of Matches, returning an OperationOutcome for
// each match detailing the location(s) of conflicts between the resources in the Match.
func (d *Detector) AllConflicts(matches []Match) (conflicts []models.OperationOutcome, err error) {
	for _, match := range matches {
		conflict, err := d.findConflicts(&match)
		if err != nil {
			return nil, err
		}
		if conflict != nil {
			conflicts = append(conflicts, *conflict)
		}
	}
	return conflicts, nil
}

func (d *Detector) findConflicts(match *Match) (conflict *models.OperationOutcome, err error) {
	// First, find all non-nil paths in the left resource, and the values at those paths.
	leftPaths := make(PathMap)
	traverse(reflect.ValueOf(match.Left), leftPaths, "")

	// Then traverse the right.
	rightPaths := make(PathMap)
	traverse(reflect.ValueOf(match.Right), rightPaths, "")

	// Finally, compare paths and values. If a path exists in both resources we can compare
	// it to identify conflicts. If it only exists in one resource, there is automatically a conflict.
	commonPaths := intersection(leftPaths.Keys(), rightPaths.Keys())
	leftOnlyPaths := setDiff(leftPaths.Keys(), rightPaths.Keys())  // L not in R
	rightOnlyPaths := setDiff(rightPaths.Keys(), leftPaths.Keys()) // R not in L

	// Find conflicts between the common paths.
	locations := []string{}

	for _, cp := range commonPaths {
		// Unless the values are EXACTLY the same, we mark them as a conflict.
		if !d.compareValues(leftPaths[cp], rightPaths[cp]) {
			locations = append(locations, cp)
		}
	}

	// Left-only and right-only paths are automatically conflicts.
	locations = append(locations, leftOnlyPaths...)
	locations = append(locations, rightOnlyPaths...)

	// Build a new OperationOutcome detailing all conflicts for this match.
	if len(locations) > 0 {
		return d.createConflictOperationOutcome(locations), nil
	}
	// No conflicts found
	return nil, nil
}

// compareValues compares 2 reflected values obtained by traversing FHIR resources. The values
// must be of the same kind to do a comparison. compareValues should only be used to compare values
// collected by traverse(). Traverse ensures that only primitive go types (strings, bools, ints, etc.)
// are collected as valid paths for comparison.
func (d *Detector) compareValues(left, right reflect.Value) bool {
	if left.Kind() != right.Kind() {
		return false
	}

	switch left.Kind() {
	case reflect.String:
		return left.String() == right.String()

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return left.Uint() == right.Uint()

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return left.Int() == right.Int()

	case reflect.Float32, reflect.Float64:
		return left.Float() == right.Float()

	case reflect.Bool:
		return left.Bool() == right.Bool()

	// This is only for time.Time objects, all other structs should have been traversed.
	case reflect.Struct:
		leftTime, ok := left.Interface().(time.Time)
		if !ok {
			return false
		}
		rightTime, ok := right.Interface().(time.Time)
		if !ok {
			return false
		}
		return leftTime.Equal(rightTime)

	default:
		return false
	}
}

func (d *Detector) createConflictOperationOutcome(locations []string) *models.OperationOutcome {
	outcome := &models.OperationOutcome{
		DomainResource: models.DomainResource{
			Resource: models.Resource{
				Id:           bson.NewObjectId().Hex(),
				ResourceType: "OperationOutcome",
			},
		},
		Issue: []models.OperationOutcomeIssueComponent{
			models.OperationOutcomeIssueComponent{
				Severity: "information",
				Code:     "conflict",
				Location: locations,
			},
		},
	}

	return outcome
}
