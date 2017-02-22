package merge

import (
	"reflect"

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
		if !compareValues(leftPaths[cp], rightPaths[cp]) {
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
