package server

import (
	"github.com/gin-gonic/gin"
	"gopkg.in/mgo.v2"
)

// RegisterRoutes registers all routes needed to serve the patient merging service.
func RegisterRoutes(router *gin.Engine, session *mgo.Session, dbname string, fhirHost string) {

	mc := NewMergeController(session, dbname, fhirHost)

	// Merging and confict resolution
	router.POST("/merge", mc.Merge)
	router.POST("/merge/:merge_id/resolve/:conflict_id", mc.Resolve)
	router.POST("/merge/:merge_id/abort", mc.Abort)

	// Convenience routes
	router.GET("/merge", mc.AllMerges)
	router.GET("/merge/:merge_id", mc.GetMerge)
	router.GET("/merge/:merge_id/conflicts", mc.GetRemainingConflicts)
	router.GET("/merge/:merge_id/resolved", mc.GetResolvedConflicts)
	router.GET("/merge/:merge_id/target", mc.GetTarget)
}
