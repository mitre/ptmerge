package fhirutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"

	"github.com/intervention-engine/fhir/models"
	"gopkg.in/mgo.v2/bson"
)

// GetResourceID returns the string equivalent of a FHIR resource ID.
func GetResourceID(resource interface{}) string {
	return reflect.ValueOf(resource).Elem().FieldByName("Id").String()
}

// SetResourceID sets or updates the Id for a FHIR resource.
func SetResourceID(resource interface{}, newID string) {
	reflect.ValueOf(resource).Elem().FieldByName("Id").SetString(newID)
}

// GetResourceType returns the string equivalent of a FHIR resource type.
func GetResourceType(resource interface{}) string {
	val := reflect.ValueOf(resource)
	if val.Kind() == reflect.Ptr || val.Kind() == reflect.Interface {
		return val.Elem().FieldByName("ResourceType").String()
	}
	return ""
}

// GetResourceByURL GETs a FHIR resource from it's specified URL.
func GetResourceByURL(resourceType, resourceURL string) (resource interface{}, err error) {
	// Make the request.
	res, err := http.Get(resourceURL)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

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
	err = json.Unmarshal(body, &resource)
	if err != nil {
		return nil, err
	}
	return resource, nil
}

// GetResource GETs a FHIR resource of a specified resourceType from the host provided.
func GetResource(host, resourceType, resourceID string) (resource interface{}, err error) {
	// Make the request.
	res, err := http.Get(host + "/" + resourceType + "/" + resourceID)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode == 404 {
		return nil, fmt.Errorf("Resource %s:%s not found", resourceType, resourceID)
	}

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("An unexpected error occured while requesting resource %s:%s", resourceType, resourceID)
	}

	// Unmarshal the resource returned.
	resource = models.NewStructForResourceName(resourceType)
	body, err = ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(body, &resource)
	if err != nil {
		return nil, err
	}
	return resource, nil
}

// PostResource POSTs a FHIR resource of a specified resourceType to the host provided.
func PostResource(host, resourceType string, resource interface{}) (created interface{}, err error) {
	data, err := json.Marshal(resource)
	if err != nil {
		return nil, err
	}

	res, err := http.Post(host+"/"+resourceType, "application/fhir+json", bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("Failed to create resource %s", resourceType)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	created = models.NewStructForResourceName(resourceType)
	err = json.Unmarshal(body, &created)
	if err != nil {
		return nil, err
	}
	return created, nil
}

// UpdateResource PUTs a FHIR resource of a specified resourceType on the host provided, updating the resource.
func UpdateResource(host, resourceType string, resource interface{}) (updatedResource interface{}, err error) {
	// Marshal the updated resource.
	data, err := json.Marshal(resource)
	if err != nil {
		return nil, err
	}

	resourceID := GetResourceID(resource)

	req, err := http.NewRequest("PUT", host+"/"+resourceType+"/"+resourceID, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/fhir+json")
	if err != nil {
		return nil, err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Failed to update resource %s:%s", resourceType, resourceID)
	}

	// Unmarshal the resource returned.
	updatedResource = models.NewStructForResourceName(resourceType)
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(body, &updatedResource)
	if err != nil {
		return nil, err
	}
	return updatedResource, nil
}

// DeleteResourceByURL DELETEs a FHIR resource at the specified URL.
func DeleteResourceByURL(resourceURL string) error {
	req, err := http.NewRequest("DELETE", resourceURL, nil)
	if err != nil {
		return err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNoContent {
		return fmt.Errorf("Resource %s was not deleted", resourceURL)
	}
	return nil
}

// DeleteResource DELETEs a FHIR resource of a specified resourceType on the host provided.
func DeleteResource(host, resourceType, resourceID string) error {
	req, err := http.NewRequest("DELETE", host+"/"+resourceType+"/"+resourceID, nil)
	if err != nil {
		return err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNoContent {
		return fmt.Errorf("Resource %s:%s was not deleted", resourceType, resourceID)
	}
	return nil
}

// OperationOutcome creates a new OperatioOutcome detailing all conflicts
// in the target resource, identified by its targetResourceID.
func OperationOutcome(targetResourceType, targetResourceID string, conflictPaths []string) (oo *models.OperationOutcome) {
	oo = &models.OperationOutcome{
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
				// The target resource type and ID are stored as additional diagnostic information.
				Diagnostics: targetResourceType + ":" + targetResourceID,
				// Paths in the resource with conflicts, as a JSON path rather than XPath.
				Location: conflictPaths,
			},
		},
	}
	return oo
}

// TransactionBundle creates a new Bundle of resources for
// transaction with the host FHIR server.
func TransactionBundle(resources []interface{}) (bundle *models.Bundle) {
	total := uint32(len(resources))

	bundle = &models.Bundle{
		Resource: models.Resource{
			Id: bson.NewObjectId().Hex(),
		},
		Type:  "transaction",
		Total: &total,
		Entry: make([]models.BundleEntryComponent, total),
	}

	for i := 0; i < int(total); i++ {
		bundle.Entry[i] = models.BundleEntryComponent{
			Resource: resources[i],
			Request: &models.BundleEntryRequestComponent{
				Method: "POST",
				Url:    "/" + GetResourceType(resources[i]),
			},
		}
	}
	return bundle
}

// ResponseBundle returns a new response bundle. Status codes may be either 200 Ok
// when responding with a bundle that didn't require creating any host resources,
// or 201 Created if host resources were also created (e.g. OperationOutcomes).
func ResponseBundle(statusCode string, resources []interface{}) (bundle *models.Bundle) {
	total := uint32(len(resources))

	bundle = &models.Bundle{
		Resource: models.Resource{
			Id: bson.NewObjectId().Hex(),
		},
		Type:  "transaction-response",
		Total: &total,
		Entry: make([]models.BundleEntryComponent, total),
	}

	for i := 0; i < int(total); i++ {
		bundle.Entry[i] = models.BundleEntryComponent{
			Resource: resources[i],
			Response: &models.BundleEntryResponseComponent{
				Status: statusCode,
			},
		}
	}
	return bundle
}

// LoadResource returns a resource-appropriate struct for the unmarshaled file.
func LoadResource(resourceType, filepath string) (resource interface{}, err error) {
	data, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	resource = models.NewStructForResourceName(resourceType)
	if resource == nil {
		return nil, fmt.Errorf("Unknown resource type '%s'", resourceType)
	}
	err = json.Unmarshal(data, &resource)
	if err != nil {
		return nil, err
	}
	return resource, nil
}

// LoadAndPostResource loads a resource from a fixture and immediately POSTs it,
// returning the resource that was created.
func LoadAndPostResource(host, resourceType, filepath string) (created interface{}, err error) {
	data, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	var url string
	if resourceType == "" {
		// This is a batch operation
		url = host
		resourceType = "Bundle"
	} else {
		url = host + "/" + resourceType
	}

	res, err := http.Post(url, "application/fhir+json", bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated && res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Failed to create resource %s", resourceType)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	created = models.NewStructForResourceName(resourceType)
	err = json.Unmarshal(body, &created)
	if err != nil {
		return nil, err
	}
	return created, nil
}
