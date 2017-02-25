package merge

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/intervention-engine/fhir/server"
	"github.com/mitre/ptmerge/testutil"
	"github.com/stretchr/testify/suite"
)

type MergerTestSuite struct {
	testutil.MongoSuite
	FHIRServer *httptest.Server
}

func TestMergerTestSuite(t *testing.T) {
	suite.Run(t, new(MergerTestSuite))
}

func (m *MergerTestSuite) SetupSuite() {
	// Set gin to release mode (less verbose output).
	gin.SetMode(gin.ReleaseMode)

	// Create a mock FHIR server to check the ptmerge service's outgoing requests. The first
	// call to s.DB() stands up the mock Mongo server, see testutil/mongo_suite.go for more.
	fhirEngine := gin.New()
	ms := server.NewMasterSession(m.DB().Session, "ptmerge-test")
	server.RegisterRoutes(fhirEngine, nil, server.NewMongoDataAccessLayer(ms, nil), server.Config{})
	m.FHIRServer = httptest.NewServer(fhirEngine)
}

func (m *MergerTestSuite) TearDownSuite() {
	m.FHIRServer.Close()
	// Clean up and remove all temporary files from the mocked database.
	// See testutil/mongo_suite.go for more.
	m.TearDownDBServer()
}

func (m *MergerTestSuite) TearDownTest() {
	// Cleanup any saved merge states.
	m.DB().C("merges").DropCollection()
}

// ========================================================================= //
// TEST MERGE                                                                //
// ========================================================================= //

func (m *MergerTestSuite) TestMerge() {

}

// ========================================================================= //
// TEST RESOLVE CONFLICT                                                     //
// ========================================================================= //

func (m *MergerTestSuite) TestResolveConflict() {

}
