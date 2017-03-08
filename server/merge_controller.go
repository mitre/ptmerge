package server

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"

	"github.com/gin-gonic/gin"
	"github.com/intervention-engine/fhir/models"
	"github.com/mitre/ptmerge/fhirutil"
	"github.com/mitre/ptmerge/merge"
	"github.com/mitre/ptmerge/state"
)

// MergeController manages the resource handlers for a Merge operation.
type MergeController struct {
	session  *mgo.Session
	dbname   string
	fhirHost string
}

// NewMergeController returns a pointer to a newly initialized MergeController.
func NewMergeController(session *mgo.Session, dbname string, fhirHost string) *MergeController {
	return &MergeController{
		session:  session,
		dbname:   dbname,
		fhirHost: fhirHost,
	}
}

// ========================================================================= //
// MERGE                                                                     //
// ========================================================================= //

// Merge attempts to merge 2 FHIR bundles of patient resources given the URLs to both bundles.
func (m *MergeController) Merge(c *gin.Context) {
	var err error
	worker := m.session.Copy()
	defer worker.Close()

	source1 := c.Query("source1")
	source2 := c.Query("source2")

	if source1 == "" || source2 == "" {
		c.String(http.StatusBadRequest, "URL(s) referencing source bundles were not provided")
		return
	}

	merger := merge.NewMerger(m.fhirHost)
	outcome, targetURL, err := merger.Merge(source1, source2)

	if err != nil {
		if err == merge.ErrNoPatientResource || err == merge.ErrDuplicatePatientResource {
			c.String(http.StatusBadRequest, err.Error())
			return
		}
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	if targetURL == "" {
		// The merge had no conflicts, just return the merged bundle.
		c.JSON(http.StatusOK, outcome)
		return
	}

	// Check outcome to get a list of merge conflicts.
	conflictMap := make(state.ConflictMap)
	for _, entry := range outcome.Entry {
		oo, ok := entry.Resource.(*models.OperationOutcome)
		if !ok {
			c.String(http.StatusInternalServerError, "Malformed merge conflict")
			return
		}
		if len(oo.Issue) != 1 {
			c.String(http.StatusInternalServerError, "Malformed merge conflict: bad Issue")
			return
		}
		if oo.Issue[0].Diagnostics == "" {
			c.String(http.StatusInternalServerError, "Malformed merge conflict: bad Diagnostic information")
			return
		}

		conflictID := oo.Id
		parts := strings.SplitN(oo.Issue[0].Diagnostics, ":", 2)

		conflictMap[conflictID] = &state.ConflictState{
			OperationOutcomeURL: m.fhirHost + "/OperationOutcome/" + conflictID,
			TargetResource: state.TargetResource{
				ResourceType: parts[0],
				ResourceID:   parts[1],
			},
		}
	}

	// Some conflicts exist, create a new record in mongo to manage this merge's state.
	mergeID := bson.NewObjectId().Hex()
	err = worker.DB(m.dbname).C("merges").Insert(&state.MergeState{
		MergeID:   mergeID,
		Completed: false,
		TargetURL: targetURL,
		Conflicts: conflictMap,
	})

	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Return the bundle of conflicts to resolve. The mergeID is passed in the Location header.
	c.Header("Location", mergeID)
	c.JSON(http.StatusCreated, outcome)
}

// ========================================================================= //
// RESOLVE CONFLICT                                                          //
// ========================================================================= //

// Resolve attempts to resolve a single merge confict given the mergeID, conflictID,
// and the complete resource that resolve the conflict.
func (m *MergeController) Resolve(c *gin.Context) {
	var err error
	worker := m.session.Copy()
	defer worker.Close()

	mergeID := c.Param("merge_id")
	conflictID := c.Param("conflict_id")

	// Retrieve the merge state and conflicts from mongo.
	var mergeState state.MergeState
	err = worker.DB(m.dbname).C("merges").Find(bson.M{"_id": mergeID}).One(&mergeState)
	if err != nil {
		if err == mgo.ErrNotFound {
			c.String(http.StatusNotFound, "Merge %s not found", mergeID)
			return
		}
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Check that the merge is incomplete.
	if mergeState.Completed {
		c.String(http.StatusBadRequest, "Merge %s is complete, no remaining conflicts to resolve", mergeID)
		return
	}

	// Check that the conflictID exists and is part of this merge.
	conflict, found := mergeState.Conflicts[conflictID]
	if !found {
		c.String(http.StatusNotFound, "Merge conflict %s not found for merge %s", conflictID, mergeID)
		return
	}

	// Check that the conflict wasn't already resolved.
	if conflict.Resolved {
		c.String(http.StatusBadRequest, "Merge conflict %s was already resolved for merge %s", conflictID, mergeID)
		return
	}

	// Extract the resource from the request body.
	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Now we can unmarshal the body into the proper resource struct.
	updatedResource := models.NewStructForResourceName(conflict.TargetResource.ResourceType)
	err = json.Unmarshal(body, &updatedResource)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Attempt to resolve the conflict with this updatedResource.
	merger := merge.NewMerger(m.fhirHost)
	err = merger.ResolveConflict(mergeState.TargetURL, conflict.TargetResource.ResourceID, updatedResource)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// No error means the conflict was resolved, so update the merge state.
	mergeState.Conflicts[conflictID].Resolved = true
	err = worker.DB(m.dbname).C("merges").UpdateId(mergeID, bson.M{"$set": mergeState})
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Check if there were still other unresolved conflicts.
	numRemaining := len(mergeState.Conflicts.RemainingConflicts())
	if numRemaining == 0 {
		// No conflicts remaining, mark the merge as "completed" and return the target Bundle.
		err = worker.DB(m.dbname).C("merges").UpdateId(mergeID, bson.M{"$set": bson.M{"completed": true}})
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		targetBundle, err := fhirutil.GetResourceByURL("Bundle", mergeState.TargetURL)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		c.JSON(http.StatusOK, targetBundle)
		return
	}

	// At least one conflict remaining, return an bundle of conflicts.
	remainingConflicts := make([]interface{}, numRemaining)
	for i, id := range mergeState.Conflicts.RemainingConflicts() {
		oo, err := fhirutil.GetResourceByURL("OperationOutcome", mergeState.Conflicts[id].OperationOutcomeURL)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		remainingConflicts[i] = oo
	}
	c.JSON(http.StatusOK, fhirutil.ResponseBundle("200", remainingConflicts))
}

// ========================================================================= //
// DELETE MERGE                                                              //
// ========================================================================= //

// DeleteMerge terminates an in-progress merge given the mergeID.
func (m *MergeController) DeleteMerge(c *gin.Context) {
	var err error
	worker := m.session.Copy()
	defer worker.Close()

	mergeID := c.Param("merge_id")

	// Get the merge state from mongo.
	var mergeState state.MergeState
	err = worker.DB(m.dbname).C("merges").Find(bson.M{"_id": mergeID}).One(&mergeState)
	if err != nil {
		if err == mgo.ErrNotFound {
			c.String(http.StatusNotFound, "Merge %s not found", mergeID)
			return
		}
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	// Delete all conflicts.
	for _, key := range mergeState.Conflicts.Keys() {
		err = fhirutil.DeleteResourceByURL(mergeState.Conflicts[key].OperationOutcomeURL)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
	}

	// Delete the target.
	err = fhirutil.DeleteResourceByURL(mergeState.TargetURL)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Now wipe the saved merge state.
	err = worker.DB(m.dbname).C("merges").RemoveId(mergeID)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// 204 response explicitly has no body.
	c.Data(http.StatusNoContent, "", nil)
}

// ========================================================================= //
// MERGE TARGET MANAGEMENT                                                   //
// ========================================================================= //

// GetTarget returns the (partially complete) merge target given a mergeID.
func (m *MergeController) GetTarget(c *gin.Context) {
	var err error
	worker := m.session.Copy()
	defer worker.Close()

	mergeID := c.Param("merge_id")

	// Get the merge state from mongo.
	var mergeState state.MergeState
	err = worker.DB(m.dbname).C("merges").Find(bson.M{"_id": mergeID}).One(&mergeState)
	if err != nil {
		if err == mgo.ErrNotFound {
			c.String(http.StatusNotFound, "Merge %s not found", mergeID)
			return
		}
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Get the target from the host FHIR server.
	targetBundle, err := fhirutil.GetResourceByURL("Bundle", mergeState.TargetURL)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, targetBundle)
}

// UpdateTargetResource allows manual update of a resource in the target bundle.
// The updated resource should be in the POST body.
func (m *MergeController) UpdateTargetResource(c *gin.Context) {
	var err error
	worker := m.session.Copy()
	defer worker.Close()

	mergeID := c.Param("merge_id")
	targetResourceID := c.Param("resource_id")

	// Get the merge state from mongo.
	var mergeState state.MergeState
	err = worker.DB(m.dbname).C("merges").Find(bson.M{"_id": mergeID}).One(&mergeState)
	if err != nil {
		if err == mgo.ErrNotFound {
			c.String(http.StatusNotFound, "Merge %s not found", mergeID)
			return
		}
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Get the resource from the request body.
	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Unmarshal into a map[string]interface{} to get the resourceType.

	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	resourceType := fhirutil.JSONGetResourceType(body)
	if resourceType == "" {
		c.String(http.StatusInternalServerError, "Could not identify resourceType of updated resource")
		return
	}

	// Now we can unmarshal the body into the proper resource struct.
	updatedResource := models.NewStructForResourceName(resourceType)
	err = json.Unmarshal(body, &updatedResource)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Update the target resource.
	merger := merge.NewMerger(m.fhirHost)
	err = merger.UpdateTargetResource(mergeState.TargetURL, targetResourceID, updatedResource)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Respond with the updated resource.
	c.JSON(http.StatusOK, updatedResource)
}

// DeleteTargetResource allows manual update of a resource in the target bundle.
// The updated resource should be in the POST body.
func (m *MergeController) DeleteTargetResource(c *gin.Context) {
	var err error
	worker := m.session.Copy()
	defer worker.Close()

	mergeID := c.Param("merge_id")
	targetResourceID := c.Param("resource_id")

	// Get the merge state from mongo.
	var mergeState state.MergeState
	err = worker.DB(m.dbname).C("merges").Find(bson.M{"_id": mergeID}).One(&mergeState)
	if err != nil {
		if err == mgo.ErrNotFound {
			c.String(http.StatusNotFound, "Merge %s not found", mergeID)
			return
		}
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	merger := merge.NewMerger(m.fhirHost)
	err = merger.DeleteTargetResource(mergeState.TargetURL, targetResourceID)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Respond with 204 no content.
	c.Data(http.StatusNoContent, "", nil)
}

// ========================================================================= //
// CONFLICT MANAGEMENT                                                       //
// ========================================================================= //

// GetRemainingConflicts returns all unresolved merge conflicts for a given mergeID.
func (m *MergeController) GetRemainingConflicts(c *gin.Context) {
	var err error
	worker := m.session.Copy()
	defer worker.Close()

	mergeID := c.Param("merge_id")

	// Get the merge state from mongo.
	var mergeState state.MergeState
	err = worker.DB(m.dbname).C("merges").Find(bson.M{"_id": mergeID}).One(&mergeState)
	if err != nil {
		if err == mgo.ErrNotFound {
			c.String(http.StatusNotFound, "Merge %s not found", mergeID)
			return
		}
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Extract the URLs to all unresolved conflicts.
	numRemaining := len(mergeState.Conflicts.RemainingConflicts())
	conflicts := make([]interface{}, numRemaining)
	for i, id := range mergeState.Conflicts.RemainingConflicts() {
		conflict, err := fhirutil.GetResource(m.fhirHost, "OperationOutcome", id)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		conflicts[i] = conflict
	}

	c.JSON(http.StatusOK, fhirutil.ResponseBundle("200", conflicts))
}

// GetResolvedConflicts returns all resolved merge conflicts for a given mergeID.
func (m *MergeController) GetResolvedConflicts(c *gin.Context) {
	var err error
	worker := m.session.Copy()
	defer worker.Close()

	mergeID := c.Param("merge_id")

	// Get the merge state from mongo.
	var mergeState state.MergeState
	err = worker.DB(m.dbname).C("merges").Find(bson.M{"_id": mergeID}).One(&mergeState)
	if err != nil {
		if err == mgo.ErrNotFound {
			c.String(http.StatusNotFound, "Merge %s not found", mergeID)
			return
		}
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Extract the URLs to all unresolved conflicts.
	numResolved := len(mergeState.Conflicts.ResolvedConflicts())
	resolved := make([]interface{}, numResolved)
	for i, id := range mergeState.Conflicts.ResolvedConflicts() {
		r, err := fhirutil.GetResource(m.fhirHost, "OperationOutcome", id)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		resolved[i] = r
	}

	c.JSON(http.StatusOK, fhirutil.ResponseBundle("200", resolved))
}

// DeleteConflict removes a conflict from the merge, including its target resource.
func (m *MergeController) DeleteConflict(c *gin.Context) {
	var err error
	worker := m.session.Copy()
	defer worker.Close()

	mergeID := c.Param("merge_id")
	conflictID := c.Param("conflict_id")

	// Get the merge state from mongo.
	var mergeState state.MergeState
	err = worker.DB(m.dbname).C("merges").Find(bson.M{"_id": mergeID}).One(&mergeState)
	if err != nil {
		if err == mgo.ErrNotFound {
			c.String(http.StatusNotFound, "Merge %s not found", mergeID)
			return
		}
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Check that the merge is incomplete.
	if mergeState.Completed {
		c.String(http.StatusBadRequest, "Merge %s is complete, no remaining conflicts to resolve", mergeID)
		return
	}

	// Check that the conflictID exists and is part of this merge.
	conflict, found := mergeState.Conflicts[conflictID]
	if !found {
		c.String(http.StatusNotFound, "Merge conflict %s not found for merge %s", conflictID, mergeID)
		return
	}

	// Check that the conflict wasn't already resolved.
	if conflict.Resolved {
		c.String(http.StatusBadRequest, "Merge conflict %s was already resolved for merge %s", conflictID, mergeID)
		return
	}

	// Delete this conflict from the target.
	merger := merge.NewMerger(m.fhirHost)
	err = merger.DeleteTargetResource(mergeState.TargetURL, conflict.TargetResource.ResourceID)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// No error mean success, delete the conflict OperationOutcome.
	err = fhirutil.DeleteResourceByURL(conflict.OperationOutcomeURL)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Remove the conflict from the merge state.
	delete(mergeState.Conflicts, conflictID)

	// Save the updated state.
	err = worker.DB(m.dbname).C("merges").UpdateId(mergeID, bson.M{"$set": mergeState})
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Respond with 204 no content.
	c.Data(http.StatusNoContent, "", nil)
}

// ========================================================================= //
// MERGE METADATA                                                            //
// ========================================================================= //

// AllMerges returns the metadata for all merges we have a record of.
func (m *MergeController) AllMerges(c *gin.Context) {
	var err error
	worker := m.session.Copy()
	defer worker.Close()

	// Get all merges from mongo.
	var merges []state.MergeState
	err = worker.DB(m.dbname).C("merges").Find(nil).All(&merges)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Package up the merges metadata.
	meta := &state.Merges{
		Timestamp: time.Now(),
		Merges:    merges,
	}

	c.JSON(http.StatusOK, meta)
}

// GetMerge returns the metadata for a single merge.
func (m *MergeController) GetMerge(c *gin.Context) {
	var err error
	worker := m.session.Copy()
	defer worker.Close()

	mergeID := c.Param("merge_id")

	// Get the merge state from mongo.
	var mergeState state.MergeState
	err = worker.DB(m.dbname).C("merges").Find(bson.M{"_id": mergeID}).One(&mergeState)
	if err != nil {
		if err == mgo.ErrNotFound {
			c.String(http.StatusNotFound, "Merge %s not found", mergeID)
			return
		}
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Package up the merge metadata.
	meta := &state.Merge{
		Timestamp: time.Now(),
		Merge:     mergeState,
	}

	c.JSON(http.StatusOK, meta)
}
