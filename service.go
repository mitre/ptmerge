package main

import (
	"encoding/json"
	"log"
	"net/http"
	"github.com/gorilla/mux"
	"time"
	"fmt"
	"bytes"
)

var myClient = &http.Client{Timeout: 10 * time.Second}

// FHIR server config
var fhirHost = "http://localhost"
var fhirPort = "3001"

type FhirGenerated struct {
	ResourceType string `json:"resourceType"`
	//ID string `json:"-"`	//id
	Text struct {
			     Status string `json:"status"`
			     Div string `json:"div"`
		     } `json:"text"`
	Extension []struct {
		URL string `json:"url"`
		ValueCodeableConcept struct {
			    Coding []struct {
				    System string `json:"system"`
				    Code string `json:"code"`
				    Display string `json:"display"`
			    } `json:"coding"`
			    Text string `json:"text"`
		    } `json:"valueCodeableConcept,omitempty"`
		ValueAddress struct {
			    City string `json:"city"`
			    State string `json:"state"`
		    } `json:"valueAddress,omitempty"`
		ValueString string `json:"valueString,omitempty"`
	} `json:"extension"`
	Identifier []struct {
		System string `json:"system"`
		Value string `json:"value"`
		Type struct {
			       Coding []struct {
				       System string `json:"system"`
				       Code string `json:"code"`
			       } `json:"coding"`
		       } `json:"type,omitempty"`
	} `json:"identifier"`
	Name []struct {
		Use string `json:"use"`
		Family []string `json:"family"`
		Given []string `json:"given"`
		Prefix []string `json:"prefix"`
	} `json:"name"`
	Telecom []struct {
		System string `json:"system"`
		Value string `json:"value"`
		Use string `json:"use"`
	} `json:"telecom"`
	Gender string `json:"gender"`
	BirthDate string `json:"birthDate"`
	DeceasedDateTime string `json:"deceasedDateTime"`
	Address []struct {
		Line []string `json:"line"`
		City string `json:"city"`
		State string `json:"state"`
		PostalCode string `json:"postalCode"`
	} `json:"address"`
	MaritalStatus struct {
			     Coding []struct {
				     System string `json:"system"`
				     Code string `json:"code"`
			     } `json:"coding"`
		     } `json:"maritalStatus"`
	MultipleBirthBoolean bool `json:"multipleBirthBoolean"`
	Photo []struct {
		ContentType string `json:"contentType"`
		Data string `json:"data"`
		Title string `json:"title"`
	} `json:"photo"`
}

func getJson(url string, target interface{}) error {
	r, err := myClient.Get(url)

	if err != nil {
		return err
	}
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(&target)

}

func Merge(w http.ResponseWriter, req *http.Request) {

	var id1 = req.FormValue("source1");
	var id2 = req.FormValue("source2");
	source1 := new(FhirGenerated)
	source2 := new(FhirGenerated)
	getJson(fhirHost + ":" + fhirPort + "/Patient/" + id1, source1)
	getJson(fhirHost + ":" + fhirPort + "/Patient/" + id2, source2)

	// Encode POST bundle
	// Currently using an ID-stripped source1 instance to mock a new Patient resource
	buf := bytes.NewBuffer(nil)
	enc := json.NewEncoder(buf)
	jstr := enc.Encode(source1)
	if jstr != nil {
		log.Fatal(jstr)
	}
	req, err := http.NewRequest("POST", fhirHost + ":" + fhirPort + "/Patient", buf)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := myClient.Do(req)
	if err != nil {
		fmt.Println("Could not reach server ", fhirHost + ":" + fhirPort)
	}
	fmt.Println(resp.Body)
	targetBundle := new(FhirGenerated)
	json.NewDecoder(resp.Body).Decode(&targetBundle)

}

func main() {
	router := mux.NewRouter()
	router.HandleFunc("/merge", Merge).Methods("POST")

	log.Fatal(http.ListenAndServe(":12346", router))
}