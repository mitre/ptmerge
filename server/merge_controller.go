package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/intervention-engine/fhir/models"
	"github.com/mitre/ptmerge/merge"
	"gopkg.in/mgo.v2"
)

// MergeController manages the resource handlers for a Merge. MergeController
// is also responsible for maintaining the state of a Merge using the mongo
// database.
type MergeController struct {
	session  *mgo.Session
	fhirHost string
}

// NewMergeController returns a pointer to a newly initialized MergeController.
func NewMergeController(session *mgo.Session, fhirHost string) *MergeController {
	return &MergeController{
		session:  session,
		fhirHost: fhirHost,
	}
}

// Merges 2 FHIR bundles of patient resources given the URLs to both bundles.
func (m *MergeController) merge(c *gin.Context) {
	source1 := c.Query("source1")
	source2 := c.Query("source2")

	merger := merge.NewMerger(m.fhirHost)

	// Explicitly not collecting the return values from this stub.
	merger.Merge(source1, source2)
	c.String(http.StatusOK, "Merging records %s and %s", source1, source2)
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
