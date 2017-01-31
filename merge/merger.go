package merge

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"gopkg.in/mgo.v2/bson"

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
func (m *Merger) Merge(source1, source2 string) (mergeID string, outcome *models.Bundle, err error) {
	return mockMerge(source1, source2)
}

// ResolveConflict attempts to resolve a single merge conflict. If the conflict
// resolution is successful and no more conflicts exist, the merged FHIR Bundle is
// returned. If additional conflicts still exist or the conflict resolution was not
// successful, a FHIR Bundle of OperationOutcomes is returned detailing the remaining
// merge conflicts.
func (m *Merger) ResolveConflict(mergeID, opOutcomeID string, updatedResource interface{}) (outcome *models.Bundle, err error) {
	return mockResolveConflict(mergeID, opOutcomeID, updatedResource)
}

// Abort aborts a merge in progress. The partially merged Bundle and any outstanding
// OperationOutcomes are deleted. A Bundle with a successful OperationOutcome is returned
// to the client in response.
func (m *Merger) Abort(mergeID string) (outcome *models.Bundle, err error) {
	return
}

// ========================================================================= //
// MOCKS                                                                     //
// ========================================================================= //
// I mocked up the expected behavior of these different functions so I could write
// The unit tests for them. We can swap out and delete the mocks as soon as we have
// real Merger operations fleshed out. Some of the helper functions here may also
// be useful in the real functions.

func mockMerge(source1, source2 string) (mergeID string, outcome *models.Bundle, err error) {

	res, err := http.Get(source1)
	defer res.Body.Close()

	if err != nil {
		return "", nil, err
	}

	fmt.Println("Status: ", res.StatusCode)

	if res.StatusCode == 404 {
		err = fmt.Errorf("Resource %s not found", source1)
		return "", nil, err
	}

	if strings.Compare(source1, source2) == 0 {
		// Mocked up for testing. If the 2 resources are the same return that bundle.
		decoder := json.NewDecoder(res.Body)
		mergedBundle := &models.Bundle{}
		err = decoder.Decode(mergedBundle)
		if err != nil {
			return "", nil, err
		}
		// The mergeID is nil for a successful merge since there is no reason to save state.
		return "", mergedBundle, nil
	}

	// Mocked up for testing. If the 2 sources are not the same, return a
	// bundle with mock OperationOutcomes detailing conflicts.
	return bson.NewObjectId().Hex(), createMockConflictBundle(), nil
}

func mockResolveConflict(mergeID, opOutcomeID string, updatedResource interface{}) (outcome *models.Bundle, err error) {
	return
}

func createMockConflictBundle() *models.Bundle {

	conflict1 := createOpOutcome("information", "informational", "MSG_CREATED", "New resource created")
	conflict1.Issue[0].Location[0] = "/Bundle/Patient/Name/Given" // This likely isn't proper XPath syntax

	conflict2 := createOpOutcome("information", "informational", "MSG_CREATED", "New resource created")
	conflict2.Issue[0].Location[0] = "/Bundle/Patient/Name/Family"

	total := uint32(2)

	return &models.Bundle{
		Total: &total,
		Entry: []models.BundleEntryComponent{
			models.BundleEntryComponent{
				Resource: conflict1,
			},
			models.BundleEntryComponent{
				Resource: conflict2,
			},
		},
	}
}

func createOpOutcome(severity, code, detailsCode, detailsDisplay string) *models.OperationOutcome {
	outcome := &models.OperationOutcome{
		DomainResource: models.DomainResource{
			Resource: models.Resource{
				Id:           bson.NewObjectId().Hex(),
				ResourceType: "OperationOutcome",
			},
		},
		Issue: []models.OperationOutcomeIssueComponent{
			models.OperationOutcomeIssueComponent{
				Severity: severity,
				Code:     code,
				Location: make([]string, 1),
			},
		},
	}

	if detailsCode != "" {
		outcome.Issue[0].Details = &models.CodeableConcept{
			Coding: []models.Coding{
				models.Coding{
					Code:    detailsCode,
					System:  "http://hl7.org/fhir/ValueSet/operation-outcome",
					Display: detailsDisplay},
			},
			Text: detailsDisplay,
		}
	}

	return outcome
}
