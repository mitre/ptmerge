package merge

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/intervention-engine/fhir/models"
)

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
func (m *Merger) Merge(source1, source2 string) (outcome *models.Bundle, targetURL string, err error) {
	return
}

// ResolveConflict attempts to resolve a single merge conflict. If the conflict
// resolution is successful and no more conflicts exist, the merged FHIR Bundle is
// returned. If additional conflicts still exist or the conflict resolution was not
// successful, a FHIR Bundle of OperationOutcomes is returned detailing the remaining
// merge conflicts.
func (m *Merger) ResolveConflict(targetURL, opOutcomeURL string, updatedResource interface{}) (outcome *models.Bundle, err error) {
	return
}

// Abort aborts a merge in progress by deleting all resources related to the merge.
func (m *Merger) Abort(resourceURLs []string) (err error) {
	for _, url := range resourceURLs {
		err = DeleteResource(url)
		if err != nil {
			return err
		}
	}
	return nil
}

// GetConflicts obtains a bundle of conflict OperationOutcomes from the host FHIR server.
// These conflicts may or may not be resolved.
func (m *Merger) GetConflicts(conflictURLs []string) (conflicts *models.Bundle, err error) {
	conflicts = &models.Bundle{}
	total := uint32(len(conflictURLs))
	conflicts.Total = &total
	conflicts.Type = "transaction-response"
	conflicts.Entry = make([]models.BundleEntryComponent, total)

	for i, url := range conflictURLs {
		entry := models.BundleEntryComponent{}
		conflict, err := GetResource("OperationOutcome", url)
		if err != nil {
			return nil, err
		}
		entry.Resource = conflict
		conflicts.Entry[i] = entry
	}
	return conflicts, nil
}

// GetTarget obtains the target bundle used by a merge session.
func (m *Merger) GetTarget(targetURL string) (target *models.Bundle, err error) {
	resource, err := GetResource("Bundle", targetURL)
	if err != nil {
		return nil, err
	}
	target, ok := resource.(*models.Bundle)
	if !ok {
		return nil, fmt.Errorf("Target %s was not a valid FHIR bundle", targetURL)
	}
	target.Type = "collection"
	total := uint32(len(target.Entry))
	target.Total = &total
	return target, nil
}

// ========================================================================= //
// HELPER FUNCTIONS                                                          //
// ========================================================================= //

// GetResource gets a FHIR resource of a specified type from the fully qualified
// resourceURL provided.
func GetResource(resourceType, resourceURL string) (resource interface{}, err error) {
	// Make the request.
	res, err := http.Get(resourceURL)
	if err != nil {
		return nil, err
	}

	if res.StatusCode == 404 {
		return nil, fmt.Errorf("Resource %s not found", resourceURL)
	}

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("An unexpected error occured while requesting resource %s", resourceURL)
	}

	// Unmarshal the resource returned.
	resource = models.NewStructForResourceName(resourceType)
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(body, resource)
	if err != nil {
		return nil, err
	}
	return resource, nil
}

// DeleteResource deletes a resource on a FHIR server when provided with the fully
// qualified URL referencing that resource.
func DeleteResource(resourceURL string) error {
	req, err := http.NewRequest("DELETE", resourceURL, nil)
	if err != nil {
		return err
	}

	deleteResp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	if deleteResp.StatusCode != 204 {
		return fmt.Errorf("Resource %s was not deleted", resourceURL)
	}
	return nil
}
