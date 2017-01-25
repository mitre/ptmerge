package server

import (
	"fmt"
	"net/http"
	"github.com/gin-gonic/gin"
	"log"
	"github.com/mitre/ptmerge/merge"
)

// RegisterRoutes registers all routes needed to serve the patient matching service.
func RegisterRoutes(router *gin.Engine) {

	// Merging and confict resolution
	router.POST("/merge", mergeResources)
	router.POST("/merge/:merge_id/resolve/:op_outcome_id", resolve)
	router.POST("/merge/:merge_id/abort", abort)
	router.GET("/merge/:merge_id", getConflicts)

}

// Merges 2 FHIR bundles of patient resources given the URLs to both bundles.
func mergeResources(c *gin.Context) {
	source1 := c.Query("source1")
	source2 := c.Query("source2")

	fmt.Printf("Source 1: %s\n", source1)
	fmt.Printf("Source 2: %s\n", source2)

	c.String(http.StatusOK, "Merging records %s and %s\n", source1, source2)

	mergedResult := merge.Merger{}
	outcome, err := mergedResult.Merge(source1, source2)
	if err != nil {
		c.String(http.StatusCreated, "Merge conflicts for merge %s", mergedResult)
		log.Fatal(err)
	}
	c.JSON(http.StatusOK, outcome)
}

// Resolves a single merge confict given the OperationOutcome ID and the correct resource
// that resolves the merge.
func resolve(c *gin.Context) {
	mergeID := c.Param("merge_id")
	opOutcomeID := c.Param("op_outcome_id")
	c.String(http.StatusOK, "Resolving conflict %s for merge %s", opOutcomeID, mergeID)
}

// Aborts a merge given the merge's ID.
func abort(c *gin.Context) {
	mergeID := c.Param("merge_id")
	c.String(http.StatusOK, "Aborting merge %s", mergeID)
}

// Returns all unresolved merge conflicts for a given merge ID.
func getConflicts(c *gin.Context) {
	mergeID := c.Param("merge_id")
	c.String(http.StatusOK, "Merge conflicts for merge %s", mergeID)
}
