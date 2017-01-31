package testutil

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/intervention-engine/fhir/models"
)

// PostPatientBundle inserts a patient Bundle fixture into a mock FHIR server.
// Returns the fixture (Bundle) that was inserted for further use by unit tests.
func PostPatientBundle(fhirHost, fixturePath string) (bundle *models.Bundle, err error) {
	// Load and POST the fixture.
	res, err := postFixture(fhirHost, "Bundle", fixturePath)
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
	res, err := postFixture(fhirHost, "OperationOutcome", fixturePath)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	// Check the response.
	if res.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("Failed to POST OperationOutcome fixture %s", fixturePath)
	}

	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(oo)
	if err != nil {
		return nil, err
	}

	// Return the OperationOutcome for use by a test.
	return oo, nil
}

// postFixture inserts a fixture for any specified resource into a mock FHIR server.
// Expects a relative fixture path, e.g. "../fixtures/the_fixture_you_want.json".
// It is the caller's responsibility to close the response Body.
func postFixture(fhirHost, resourceName, fixturePath string) (res *http.Response, err error) {
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
