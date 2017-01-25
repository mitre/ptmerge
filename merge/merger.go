package merge

import (
	"github.com/intervention-engine/fhir/models"
	"time"
	"net/http"
	"encoding/json"
	"os"
	"bytes"
	"log"
	"fmt"
)

// Merger is the top-level interface used to merge resources and resolve conflicts.
type Merger struct{

}

var myClient = &http.Client{Timeout: 10 * time.Second}

// Merge attempts to merge two FHIR Bundles containing patient records. If a merge
// is successful a new FHIR Bundle containing the merged patient record is returned.
// If a merge fails, a FHIR Bundle containing one or more OperationOutcomes is
// returned detailing the merge conflicts.
func (m *Merger) Merge(source1ID, source2ID string) (outcome *models.Bundle, err error) {

	FHIRHost := os.Getenv("FHIRHost")
	source1 := new(models.Bundle)
	source2 := new(models.Bundle)

	getJson(FHIRHost + "/Patient/" + source1ID, source1)
	getJson(FHIRHost + "/Patient/" + source2ID, source2)

	// Encode POST bundle
	// Currently using an ID-stripped source1 instance to mock a new Patient resource
	buf := bytes.NewBuffer(nil)
	enc := json.NewEncoder(buf)
	jstr := enc.Encode(source1)
	if jstr != nil {
		log.Fatal(jstr)
	}
	req, err := http.NewRequest("POST", FHIRHost + "/Patient", buf)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := myClient.Do(req)
	if err != nil {
		fmt.Println("Could not reach server ", FHIRHost)
	}
	targetBundle := new(models.Bundle)
	json.NewDecoder(resp.Body).Decode(&targetBundle)
	return targetBundle, err
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
// OperationOutcomes are deleted. A successful OperationOutcome is returned to the
// client in response.
func (m *Merger) Abort(mergeID string) (outcome *models.OperationOutcome, err error) {
	return
}

func getJson(url string, target interface{}) error {
	r, err := myClient.Get(url)

	if err != nil {
		return err
	}
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(&target)
}
