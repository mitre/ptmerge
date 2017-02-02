package merge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"gopkg.in/mgo.v2/bson"

	"github.com/intervention-engine/fhir/models"
	"github.com/mitre/ptmerge/testutil"
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
func (m *Merger) Merge(source1ID, source2ID string) (mergeID string, outcome *models.Bundle, err error) {

	var isMatched bool

	source1 := new(models.Bundle)
	source2 := new(models.Bundle)

	//Unmarshal JSON to source objects
	populateBundle(m.fhirHost+"/Bundle/"+source1ID, source1)
	populateBundle(m.fhirHost+"/Bundle/"+source2ID, source2)

	//Target bundle
	createdTarget := new(models.Bundle)

	//var birthdate models.FHIRDateTime
	given1, family1 := getKeyData(source1)
	given2, family2 := getKeyData(source2)

	//Compare key data of both sources
	if given1 == given2 && family1 == family2 {
		isMatched = true
	} else {
		isMatched = false
	}

	createdTarget.ResourceType = "Bundle"

	newRequest := new(models.BundleEntryRequestComponent)
	newRequest.Method = "POST"

	//Decide which type of Resource to send to FHIR server in the Bundle
	if !isMatched {

		newName := make([]models.HumanName, 1)
		newGivenName := make([]string, 1)
		newGivenName[0] = given1
		newFamilyName := make([]string, 1)
		newFamilyName[0] = family1
		newName[0].Given = newGivenName
		newName[0].Family = newFamilyName
		newRequest.Url = "Patient"

		newResource := new(models.Patient)
		newResource.Name = newName
		newEntry := make([]models.BundleEntryComponent, 1)
		newEntry[0].Resource = newResource
		newEntry[0].Request = newRequest
		createdTarget.Entry = newEntry

	} else {
		//newResource := new(models.OperationOutcome)
		newEntry := make([]models.BundleEntryComponent, 1)
		//newEntry[0].Resource = newResource
		newEntry[0].Request = newRequest
		//createdTarget.Resource = newResource
		createdTarget.Entry = newEntry
	}

	createdTarget.Type = "transaction"

	bytestr, err := json.Marshal(createdTarget)

	resp, err := http.Post(m.fhirHost+"/Bundle", "application/json", bytes.NewBuffer(bytestr))
	if err != nil {

		fmt.Println("Could not reach server ", m.fhirHost)
	}
	targetBundle := new(models.Bundle)
	json.NewDecoder(resp.Body).Decode(&targetBundle)
	mergeID = targetBundle.Id
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

func getKeyData(source *models.Bundle) (given, family string) {

	// Find Patient entry for both records and compare key data
	for _, entry := range source.Entry {
		switch t := entry.Resource.(type) {

		case *models.Patient:
			resource := entry.Resource.(*models.Patient)
			given = resource.Name[0].Given[0]
			family = resource.Name[0].Family[0]
			//birthdate := resource.BirthDate

			return given, family
		case *models.OperationOutcome:
			fmt.Printf("Resource %s is an OperationOutcome!", t.Id)
		}
	}
	return given, family
}

func populateBundle(url string, target *models.Bundle) {

	res, err := http.Get(url)
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		panic(err)
	}

	decoder := json.NewDecoder(res.Body)

	err = decoder.Decode(&target)
	if err != nil {
		panic(err)
	}
	return
}
