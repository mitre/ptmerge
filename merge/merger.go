package merge

import "github.com/intervention-engine/fhir/models"

// Merger is the top-level interface used to merge resources and resolve conflicts.
// The Merger is solely responsible for communicating with the FHIR host and managing
// resources on that host.
type Merger struct {
	fhirHost string
}

// NewMerger returns a pointer to a newly initialized Merger with a known FHIR host.
func NewMerger(fhirHost string) *Merger {
	return &Merger{
		fhirHost: fhirHost,
	}
}

// Merge attempts to merge two FHIR Bundles containing patient records. If a merge
// is successful a new FHIR Bundle containing the merged patient record is returned.
// If a merge fails, a FHIR Bundle containing one or more OperationOutcomes is
// returned detailing the merge conflicts.
func (m *Merger) Merge(source1ID, source2ID string) (mergeID string, outcome *models.Bundle, err error) {
	return
}

// ResolveConflict attempts to resolve a single merge conflict. If the conflict
// resolution is successful and no more conflicts exist, the merged FHIR Bundle is
// returned. If additional conflicts still exist or the conflict resolution was not
// successful, a FHIR Bundle of OperationOutcomes is returned detailing the remaining
// merge conflicts.
func (m *Merger) ResolveConflict(mergeID, opOutcomeID string, updatedResource interface{}) (outcome *models.Bundle, err error) {
	return
}

// Abort aborts a merge in progress. The partially merged Bundle and any outstanding
// OperationOutcomes are deleted. A Bundle with a successful OperationOutcome is returned
// to the client in response.
func (m *Merger) Abort(mergeID string) (outcome *models.Bundle, err error) {
	return
}
