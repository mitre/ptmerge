package merge

import "github.com/intervention-engine/fhir/models"

type ConflictDetector struct{}

func (c *ConflictDetector) Conflicts(match Match, conflictStrategy ConflictStrategy) (conflict *models.OperationOutcome, err error) {
	return
}
