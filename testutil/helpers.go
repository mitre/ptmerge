package testutil

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"gopkg.in/mgo.v2/bson"

	"io/ioutil"

	"github.com/intervention-engine/fhir/models"
)

// PostPatientBundle inserts a patient Bundle fixture into a mock FHIR server.
// Returns the fixture (Bundle) that was inserted for further use by unit tests.
func PostPatientBundle(fhirHost, fixturePath string) (bundle *models.Bundle, err error) {
	// Load and POST the fixture.
	res, err := PostFixture(fhirHost, "Bundle", fixturePath)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	// Check the response.
	if res.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("Failed to POST patient Bundle fixture %s", fixturePath)
	}

	decoder := json.NewDecoder(res.Body)
	responseBundle := &models.Bundle{}
	err = decoder.Decode(responseBundle)
	if err != nil {
		return nil, err
	}

	// Return the Bundle for use by a test.
	return responseBundle, nil
}

// PostOperationOutcome inserts an OperationOutcome fixture into a mock FHIR server.
// Returns the OperationOutcome that was inserted for further use by unit tests.
func PostOperationOutcome(fhirHost, fixturePath string) (oo *models.OperationOutcome, err error) {
	// Load and POST the fixture.
	res, err := PostFixture(fhirHost, "OperationOutcome", fixturePath)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	// Check the response.
	if res.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("Failed to POST OperationOutcome fixture %s", fixturePath)
	}

	oo = &models.OperationOutcome{}
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(oo)
	if err != nil {
		return nil, err
	}

	// Return the OperationOutcome for use by a test.
	return oo, nil
}

// PostFixture inserts a fixture for any specified resource into a mock FHIR server.
// Expects a relative fixture path, e.g. "../fixtures/the_fixture_you_want.json".
// It is the caller's responsibility to close the response Body.
func PostFixture(fhirHost, resourceName, fixturePath string) (res *http.Response, err error) {
	data, err := os.Open(fixturePath)
	if err != nil {
		return nil, err
	}
	defer data.Close()
	res, err = http.Post(fhirHost+"/"+resourceName, "application/json", data)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// CreateMockConflictBundle creates a sample Bundle with n OperationOutcomes
// representing merge conflicts.
func CreateMockConflictBundle(n int) *models.Bundle {
	total := uint32(n)
	bundle := &models.Bundle{
		Total: &total,
		Entry: make([]models.BundleEntryComponent, n),
	}

	for i := 0; i < n; i++ {
		oo := createOpOutcome("information", "informational", "MSG_CREATED", "New resource created")
		oo.Issue[0].Location[0] = "/Bundle/Patient/Name/Given" // This likely isn't proper XPath syntax

		bundle.Entry[i] = models.BundleEntryComponent{
			Resource: oo,
		}
	}
	return bundle
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

func CreateMockPatientObject(jsonFilePath string) *models.Patient {

	patient := new(models.Patient)
	reader, err := ioutil.ReadFile(jsonFilePath)
	if err != nil {
		panic("Could not read patient json from fixture")
	}

	err = json.Unmarshal(reader, &patient)

	if err != nil {
		panic("Could not unmarshal json from file")
	}

	return patient

}

func CreateMockEncounterObject(jsonFilePath string) *models.Encounter {

	encounter := new(models.Encounter)
	reader, err := ioutil.ReadFile(jsonFilePath)
	if err != nil {
		panic("Could not read encounter json from fixture")
	}

	err = json.Unmarshal(reader, &encounter)

	if err != nil {
		panic("Could not unmarshal json from file")
	}

	return encounter

}
