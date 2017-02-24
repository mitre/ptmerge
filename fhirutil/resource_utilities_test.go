package fhirutil

import (
	"net/http/httptest"
	"testing"

	"gopkg.in/mgo.v2/bson"

	"github.com/gin-gonic/gin"
	"github.com/intervention-engine/fhir/models"
	"github.com/intervention-engine/fhir/server"
	"github.com/mitre/ptmerge/testutil"
	"github.com/stretchr/testify/suite"
)

type FHIRUtilTestSuite struct {
	testutil.MongoSuite
	FHIRServer *httptest.Server
}

func TestFHIRUtilTestSuite(t *testing.T) {
	suite.Run(t, new(FHIRUtilTestSuite))
}

func (f *FHIRUtilTestSuite) SetupSuite() {
	// Set gin to release mode (less verbose output).
	gin.SetMode(gin.ReleaseMode)

	// Create a mock FHIR server to check the ptmerge service's outgoing requests. The first
	// call to s.DB() stands up the mock Mongo server, see testutil/mongo_suite.go for more.
	fhirEngine := gin.New()
	ms := server.NewMasterSession(f.DB().Session, "ptmerge-test")
	server.RegisterRoutes(fhirEngine, nil, server.NewMongoDataAccessLayer(ms, nil), server.Config{})
	f.FHIRServer = httptest.NewServer(fhirEngine)
}

func (f *FHIRUtilTestSuite) TearDownSuite() {
	f.FHIRServer.Close()
	// Clean up and remove all temporary files from the mocked database.
	// See testutil/mongo_suite.go for more.
	f.TearDownDBServer()
}

func (f *FHIRUtilTestSuite) TestGetResourceID() {
	expectedID := bson.NewObjectId().Hex()

	resource := &models.Condition{
		DomainResource: models.DomainResource{
			Resource: models.Resource{
				Id: expectedID,
			},
		},
	}
	f.Equal(expectedID, GetResourceID(resource))
}

func (f *FHIRUtilTestSuite) TestSetResourceID() {
	newID := bson.NewObjectId().Hex()
	resource := &models.Patient{}
	f.Empty(resource.Id)
	SetResourceID(resource, newID)
	f.NotEmpty(resource.Id)
	f.Equal(newID, resource.Id)
}

func (f *FHIRUtilTestSuite) TestGetResourceType() {
	typ := GetResourceType(&models.Condition{})
	f.Equal("Condition", typ)
}

func (f *FHIRUtilTestSuite) TestGetAndPostResource() {
	fixture, err := LoadResource("Bundle", "../fixtures/clint_abbot_bundle.json")
	f.NoError(err)
	put, err := PostResource(f.FHIRServer.URL, "Bundle", fixture)
	f.NoError(err)
	putBundle, ok := put.(*models.Bundle)
	f.True(ok)

	got, err := GetResource(f.FHIRServer.URL, "Bundle", putBundle.Id)
	f.NoError(err)
	gotBundle, ok := got.(*models.Bundle)
	f.True(ok)
	f.Equal(putBundle.Id, gotBundle.Id)
	f.Equal(len(putBundle.Entry), len(gotBundle.Entry))
}

func (f *FHIRUtilTestSuite) TestUpdateResource() {
	fixture, err := LoadResource("Patient", "../fixtures/patients/foo_bar.json")
	f.NoError(err)
	post, err := PostResource(f.FHIRServer.URL, "Patient", fixture)
	f.NoError(err)
	postPatient, ok := post.(*models.Patient)
	f.True(ok)

	postPatient.Name[0].Family = "Barson"

	up, err := UpdateResource(f.FHIRServer.URL, "Patient", postPatient)
	f.NoError(err)
	upPatient, ok := up.(*models.Patient)
	f.True(ok)
	f.Equal("Barson", upPatient.Name[0].Family)
}

func (f *FHIRUtilTestSuite) TestDeleteResource() {
	fixture, err := LoadResource("Bundle", "../fixtures/clint_abbot_bundle.json")
	f.NoError(err)
	put, err := PostResource(f.FHIRServer.URL, "Bundle", fixture)
	f.NoError(err)
	putBundle, ok := put.(*models.Bundle)
	f.True(ok)

	err = DeleteResource(f.FHIRServer.URL, "Bundle", putBundle.Id)
	f.NoError(err)
}

func (f *FHIRUtilTestSuite) TestLoadResource() {
	resource, err := LoadResource("Patient", "../fixtures/patients/foo_bar.json")
	f.NoError(err)
	patient, ok := resource.(*models.Patient)
	f.True(ok)
	f.Len(patient.Name, 1)
	f.Len(patient.Name[0].Given, 1)
	f.Equal("Foo", patient.Name[0].Given[0])
	f.Equal("Bar", patient.Name[0].Family)
}
