package merge

import (
	"reflect"

	"gopkg.in/mgo.v2/bson"

	"github.com/intervention-engine/fhir/models"
	"github.com/mitre/ptmerge/fhirutil"
)

// Detector provides tools for detecting all conflicts between 2 resources in a Match.
type Detector struct{}

// Conflicts identifies all conflicts in a Match, returning a target resource
// and any conflicts between Left and Right.
func (d *Detector) Conflicts(match *Match) (targetResource interface{}, conflict *models.OperationOutcome) {

	// Identify any conflicts between Left and Right.
	conflictPaths := d.findConflictPaths(match)

	// Create a new target. For simplicity, use the Left resource.
	target := match.Left

	// Give it a new ID.
	targetID := bson.NewObjectId().Hex()
	fhirutil.SetResourceID(target, targetID)

	if len(conflictPaths) > 0 {
		// Build an OperationOutcome detailing the conflicts.
		conflict = fhirutil.OperationOutcome(targetID, conflictPaths)
	}
	return target, conflict
}

// findConflictPaths finds all non-nil paths in both resources comprising a Match. It then identifies
// which paths have a conflict, and which paths do not.
func (d *Detector) findConflictPaths(match *Match) (conflictPaths []string) {

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

	// Find conflictPaths and noConflictPaths between the common paths.
	for _, cp := range commonPaths {
		// Unless the values exactly match, we count them as a conflict.
		if !d.compareValues(leftPaths[cp], rightPaths[cp]) {
			conflictPaths = append(conflictPaths, cp)
		}
	}

	// Left-only and right-only paths are automatically conflicts.
	conflictPaths = append(conflictPaths, leftOnlyPaths...)
	conflictPaths = append(conflictPaths, rightOnlyPaths...)
	return conflictPaths
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

	// This is only for models.FHIRDateTime objects, all other structs should have been traversed.
	case reflect.Struct:
		leftTime, ok := left.Interface().(models.FHIRDateTime)
		if !ok {
			return false
		}
		rightTime, ok := right.Interface().(models.FHIRDateTime)
		if !ok {
			return false
		}
		return leftTime.Time.Equal(rightTime.Time)

	default:
		return false
	}
}
