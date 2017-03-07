package server

import (
	"github.com/gin-gonic/gin"
	"gopkg.in/mgo.v2"
)

// RegisterRoutes registers all routes needed to serve the patient merging service.
func RegisterRoutes(router *gin.Engine, session *mgo.Session, dbname string, fhirHost string) {

	mc := NewMergeController(session, dbname, fhirHost)

	// Merge operations.
	router.POST("/merge", mc.Merge)
	router.POST("/merge/:merge_id/resolve/:conflict_id", mc.Resolve)
	router.POST("/merge/:merge_id/abort", mc.DeleteMerge)

	// Merge target management.
	router.GET("/merge/:merge_id/target", mc.GetTarget)
	router.POST("/merge/:merge_id/target/resources/:resource_id", mc.UpdateTargetResource)
	router.DELETE("/merge/:merge_id/target/resources/:resource_id", mc.DeleteTargetResource)

	// Merge conflict management.
	router.GET("/merge/:merge_id/conflicts", mc.GetRemainingConflicts)
	router.GET("/merge/:merge_id/resolved", mc.GetResolvedConflicts)
	router.DELETE("/merge/:merge_id/conflicts/:conflict_id", mc.DeleteConflict)

	// Merge metadata.
	router.GET("/merge", mc.AllMerges)
	router.GET("/merge/:merge_id", mc.GetMerge)
}
