package server

import (
	"github.com/gin-gonic/gin"
	"gopkg.in/mgo.v2"
)

// RegisterRoutes registers all routes needed to serve the patient matching service.
func RegisterRoutes(router *gin.Engine, session *mgo.Session, dbname string, fhirHost string) {

	mc := NewMergeController(session, dbname, fhirHost)

	// Merging and confict resolution
	router.POST("/merge", mc.merge)
	router.POST("/merge/:merge_id/resolve/:op_outcome_id", mc.resolve)
	router.POST("/merge/:merge_id/abort", mc.abort)

	// Convenience methods
	router.GET("/merge/:merge_id", mc.getConflicts)
}
