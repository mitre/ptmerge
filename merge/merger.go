package merge

import (
	"fmt"

	"gopkg.in/mgo.v2/bson"

	"github.com/intervention-engine/fhir/models"
	"github.com/mitre/ptmerge/fhirutil"
)

// Merger is the top-level interface used to merge resources and resolve conflicts.
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
func (m *Merger) Merge(source1, source2 string) (outcome *models.Bundle, targetURL string, err error) {
	// Get the bundles from the host FHIR server.
	resource1, err := fhirutil.GetResourceByURL("Bundle", source1)
	if err != nil {
		return nil, "", err
	}
	bundle1, ok := resource1.(*models.Bundle)
	if !ok {
		return nil, "", fmt.Errorf("Source 1 (%s) was not a valid bundle", source1)
	}

	resource2, err := fhirutil.GetResourceByURL("Bundle", source2)
	if err != nil {
		return nil, "", err
	}
	bundle2, ok := resource2.(*models.Bundle)
	if !ok {
		return nil, "", fmt.Errorf("Source 2 (%s) was not a valid bundle", source2)
	}

	// Start by matching all resources in each bundle.
	matcher := new(Matcher)
	matches, unmatchables, err := matcher.Match(bundle1, bundle2)
	if err != nil {
		return nil, "", err
	}

	// Then identify conflicts between the match pairs. This process creates 2 things:
	// 1. targetResources for a targetBundle
	// 2. OperationOutcomes (oos) representing conflicts in a targetResource
	// len(oos) <= len(targetResources) depending on what resources have conflicts
	detector := new(Detector)
	targetResources := make([]interface{}, 0, len(matches))
	opOutcomes := make([]models.OperationOutcome, 0, len(matches))

	for _, match := range matches {
		targetResource, conflictOpOutcome := detector.Conflicts(&match)
		if conflictOpOutcome != nil {
			opOutcomes = append(opOutcomes, *conflictOpOutcome)
		}
		targetResources = append(targetResources, targetResource)
	}

	// Unmatchables get new IDs for the target bundle.
	for _, umatch := range unmatchables {
		fhirutil.SetResourceID(umatch, bson.NewObjectId().Hex())
	}

	if len(opOutcomes) == 0 {
		// The merge had no conflicts, so just returned the merged bundle.
		responseBundle := fhirutil.ResponseBundle("200", append(targetResources, unmatchables...))
		return responseBundle, "", nil
	}

	// This merge had one or more conflicts, so we'll be preparing for a new
	// merge session by POSTing the target bundle.
	targetBundle := fhirutil.TransactionBundle(append(targetResources, unmatchables...))
	createdTarget, err := fhirutil.PostResource(m.fhirHost, "Bundle", targetBundle)
	if err != nil {
		return nil, "", err
	}
	targetURL = m.fhirHost + "/Bundle/" + fhirutil.GetResourceID(createdTarget)

	// POST all of the OperationOutcomes too.
	createdOpOutcomes := make([]interface{}, len(opOutcomes))
	for i, oo := range opOutcomes {
		created, err := fhirutil.PostResource(m.fhirHost, "OperationOutcome", oo)
		if err != nil {
			// This is a tricky state where the target was created but the merge operation failed.
			// Deleting the target to be safe. The error for DeleteResourceByURL is not checked
			// since we're already in an error state.
			fhirutil.DeleteResourceByURL(targetURL)
			return nil, "", err
		}
		createdOpOutcomes[i] = created
	}

	// Return the bundle of OperationOutcomes.
	responseBundle := fhirutil.ResponseBundle("201", createdOpOutcomes)
	return responseBundle, targetURL, nil
}

// ResolveConflict attempts to resolve a single merge conflict. If the conflict
// resolution is successful and no more conflicts exist, the merged FHIR Bundle is
// returned. If additional conflicts still exist or the conflict resolution was not
// successful, a FHIR Bundle of OperationOutcomes is returned detailing the remaining
// merge conflicts.
func (m *Merger) ResolveConflict(targetBundleURL, targetResourceID string, updatedResource interface{}) error {
	// Get the merge target.
	target, err := fhirutil.GetResourceByURL("Bundle", targetBundleURL)
	if err != nil {
		return err
	}
	targetBundle := target.(*models.Bundle)

	// Find the targetResource of this conflict in the bundle.
	targetResourceIdx := -1
	for i, entry := range targetBundle.Entry {
		if fhirutil.GetResourceID(entry.Resource) == targetResourceID {
			targetResourceIdx = i
			break
		}
	}

	if targetResourceIdx == -1 {
		// The target resource was not found.
		return fmt.Errorf("Target resource %s not found in target bundle %s", targetResourceID, targetBundleURL)
	}

	// Update the target resource with the one provided.
	targetBundle.Entry[targetResourceIdx].Resource = updatedResource

	// PUT the updated bundle.
	_, err = fhirutil.UpdateResource(m.fhirHost, "Bundle", targetBundle)
	if err != nil {
		return err
	}

	// No error means the conflict was resolved and the bundle was updated successfully.
	return nil
}
