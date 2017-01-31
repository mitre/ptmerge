package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/intervention-engine/fhir/models"
	"github.com/mitre/ptmerge/merge"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

// MergeController manages the resource handlers for a Merge. MergeController
// is also responsible for maintaining the state of a Merge using the mongo
// database.
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

// Merges 2 FHIR bundles of patient resources given the URLs to both bundles.
func (m *MergeController) merge(c *gin.Context) {
	var err error
	worker := m.session.Copy()
	defer worker.Close()

	source1 := c.Query("source1")
	source2 := c.Query("source2")

	if source1 == "" || source2 == "" {
		c.String(http.StatusBadRequest, "URL(s) referencing bundles to merge were not provided")
		return
	}

	merger := merge.NewMerger(m.fhirHost)
	targetID, outcome, err := merger.Merge(source1, source2)

	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	if targetID == "" {
		// The merge had no conflicts, just return the merged bundle.
		c.JSON(http.StatusOK, outcome)
		return
	}

	// Check outcome to get a list of merge conflict IDs, if any.
	conflictIDs := []string{}
	for _, entry := range outcome.Entry {
		if entryIsOperationOutcome(entry) {
			conflictIDs = append(conflictIDs, getConflictID(entry))
		}
	}

	// Some conflicts exist, create a new record in mongo to manage this merge's state.
	mergeID := bson.NewObjectId().Hex()
	err = worker.DB(m.dbname).C("merges").Insert(&MergeState{
		MergeID:        mergeID,
		TargetBundleID: targetID,
		ConflictIDs:    conflictIDs,
	})

	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Return the bundle of conflicts to resolve. The mergeID is passed in the Location header.
	c.Header("Location", mergeID)
	c.JSON(http.StatusCreated, outcome)
}

// Resolves a single merge confict given the OperationOutcome ID and the correct resource
// that resolves the merge.
func (m *MergeController) resolve(c *gin.Context) {
	var resource models.Resource
	mergeID := c.Param("merge_id")
	opOutcomeID := c.Param("op_outcome_id")
	c.BindJSON(&resource)

	merger := merge.NewMerger(m.fhirHost)

	// Explicitly not collecting the return values from this stub.
	merger.ResolveConflict(mergeID, opOutcomeID, resource)
	c.String(http.StatusOK, "Resolving conflict %s for merge %s", opOutcomeID, mergeID)
}

// Aborts a merge given the merge's ID.
func (m *MergeController) abort(c *gin.Context) {
	mergeID := c.Param("merge_id")

	merger := merge.NewMerger(m.fhirHost)

	// Explicitly not collecting the return values from this stub.
	merger.Abort(mergeID)
	c.String(http.StatusOK, "Aborting merge %s", mergeID)
}

// Returns all unresolved merge conflicts for a given merge ID.
func (m *MergeController) getConflicts(c *gin.Context) {
	mergeID := c.Param("merge_id")
	c.String(http.StatusOK, "Merge conflicts for merge %s", mergeID)
}

func entryIsOperationOutcome(entry models.BundleEntryComponent) bool {
	_, ok := entry.Resource.(*models.OperationOutcome)
	return ok
}

func getConflictID(entry models.BundleEntryComponent) string {
	return entry.Resource.(*models.OperationOutcome).Id
}
