package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gopkg.in/mgo.v2"
)

// MergeController manages the resource handles for a Merge.
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
	c.String(http.StatusOK, "Merging records %s and %s", source1, source2)
}

// Resolves a single merge confict given the OperationOutcome ID and the correct resource
// that resolves the merge.
func (m *MergeController) resolve(c *gin.Context) {
	mergeID := c.Param("merge_id")
	opOutcomeID := c.Param("op_outcome_id")
	c.String(http.StatusOK, "Resolving conflict %s for merge %s", opOutcomeID, mergeID)
}

// Aborts a merge given the merge's ID.
func (m *MergeController) abort(c *gin.Context) {
	mergeID := c.Param("merge_id")
	c.String(http.StatusOK, "Aborting merge %s", mergeID)
}

// Returns all unresolved merge conflicts for a given merge ID.
func (m *MergeController) getConflicts(c *gin.Context) {
	mergeID := c.Param("merge_id")
	c.String(http.StatusOK, "Merge conflicts for merge %s", mergeID)
}
