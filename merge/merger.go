package merge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/intervention-engine/fhir/models"
	"github.com/mitre/ptmerge/testutil"
	"gopkg.in/mgo.v2/bson"
	"log"
	"net/http"
	"strings"
	"time"
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

var myClient = &http.Client{Timeout: 10 * time.Second}

// Merge attempts to merge two FHIR Bundles containing patient records. If a merge
// is successful a new FHIR Bundle containing the merged patient record is returned.
// If a merge fails, a FHIR Bundle containing one or more OperationOutcomes is
// returned detailing the merge conflicts.

func (m *Merger) Merge(source1ID, source2ID string) (mergeID string, outcome *models.Bundle, err error) {

	source1 := new(models.Bundle)
	source2 := new(models.Bundle)
	mergeID = ""

	getJson(m.fhirHost+"/Patient/"+source1ID, source1)
	getJson(m.fhirHost+"/Patient/"+source2ID, source2)

	target := new(models.Bundle)
	// Encode POST bundle
	buf := bytes.NewBuffer(nil)
	enc := json.NewEncoder(buf)
	jstr := enc.Encode(target)
	if jstr != nil {
		log.Fatal(jstr)
	}

	req, err := http.NewRequest("POST", m.fhirHost+"/Patient", buf)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := myClient.Do(req)
	if err != nil {

		fmt.Println("Could not reach server ", m.fhirHost)
	}
	targetBundle := new(models.Bundle)
	json.NewDecoder(resp.Body).Decode(&targetBundle)
	return mergeID, targetBundle, err

}

// ResolveConflict attempts to resolve a single merge conflict. If the conflict
// resolution is successful and no more conflicts exist, the merged FHIR Bundle is
// returned. If additional conflicts still exist or the conflict resolution was not
// successful, a FHIR Bundle of OperationOutcomes is returned detailing the remaining
// merge conflicts.
func (m *Merger) ResolveConflict(targetBundle, opOutcome string, updatedResource interface{}) (outcome *models.Bundle, err error) {
	return mockResolveConflict(targetBundle, opOutcome, updatedResource)
}

// Abort aborts a merge in progress by deleting all resources related to the merge.
func (m *Merger) Abort(resourceURIs []string) (err error) {
	for _, uri := range resourceURIs {
		err = deleteResource(uri)
		if err != nil {
			return err
		}
	}
	return nil
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
	if err != nil {
		return "", nil, err
	}
	defer res.Body.Close()

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
	return bson.NewObjectId().Hex(), testutil.CreateMockConflictBundle(2), nil
}

func mockResolveConflict(targetBundle, opOutcome string, updatedResource interface{}) (outcome *models.Bundle, err error) {

	switch updatedResource.(type) {
	case *models.Patient:
		// This is mocked to be a merge with one conflict. "Resolve"
		// it and delete the OperationOutcome.
		err = deleteResource(opOutcome)
		if err != nil {
			return nil, err
		}

		// Get the final merged bundle.
		res, err := http.Get(targetBundle)
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()

		decoder := json.NewDecoder(res.Body)
		mergedBundle := &models.Bundle{}
		err = decoder.Decode(mergedBundle)
		if err != nil {
			return nil, err
		}

		// Now delete that merged bundle before returning it.
		err = deleteResource(targetBundle)
		if err != nil {
			return nil, err
		}
		return mergedBundle, nil

	case *models.Encounter:
		// This is mocked to be a merge with two conflicts. "Resolve"
		// one and leave the other.
		err = deleteResource(opOutcome)
		if err != nil {
			return nil, err
		}

		// Return a dummy bundle with one conflict.
		return testutil.CreateMockConflictBundle(1), nil

	default:
		return nil, fmt.Errorf("Unknown resource %v", updatedResource)
	}
}

// deleteResource deletes a resource on a FHIR server when provided
// with the fully qualified URI referencing that resource.
func deleteResource(resourceURI string) error {
	req, err := http.NewRequest("DELETE", resourceURI, nil)
	if err != nil {
		return err
	}

	deleteResp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	if deleteResp.StatusCode != 204 {
		return fmt.Errorf("Resource %s was not deleted", resourceURI)
	}
	return nil
}

func getJson(url string, target interface{}) error {
	r, err := myClient.Get(url)

	if err != nil {
		return err
	}
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(&target)
}
