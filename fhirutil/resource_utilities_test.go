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
	server.RegisterRoutes(fhirEngine, nil, server.NewMongoDataAccessLayer(ms, nil, true), server.Config{})
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

	resource2 := resource
	newID = bson.NewObjectId().Hex()
	SetResourceID(resource2, newID)
	f.NotEmpty(resource2.Id)
	f.Equal(newID, resource2.Id)
}

func (f *FHIRUtilTestSuite) TestGetResourceType() {
	// For a DomainResource.
	typ := GetResourceType(&models.Condition{
		DomainResource: models.DomainResource{
			Resource: models.Resource{
				ResourceType: "Condition",
			},
		},
	})
	f.Equal("Condition", typ)

	// For a Resource.
	typ = GetResourceType(&models.Bundle{
		Resource: models.Resource{
			ResourceType: "Bundle",
		},
	})
	f.Equal("Bundle", typ)

	// For an unknown, we expect a blank resourceType.
	typ = GetResourceType(3)
	f.Equal("", typ)
}

func (f *FHIRUtilTestSuite) TestGetResourceByURL() {
	fixture, err := LoadResource("Bundle", "../fixtures/bundles/clint_abbott_bundle.json")
	f.NoError(err)
	put, err := PostResource(f.FHIRServer.URL, "Bundle", fixture)
	f.NoError(err)
	putBundle, ok := put.(*models.Bundle)
	f.True(ok)

	got, err := GetResourceByURL("Bundle", f.FHIRServer.URL+"/Bundle/"+putBundle.Id)
	f.NoError(err)
	gotBundle, ok := got.(*models.Bundle)
	f.True(ok)
	f.Equal(putBundle.Id, gotBundle.Id)
	f.Equal(len(putBundle.Entry), len(gotBundle.Entry))

	// Check the Patient Entry.
	for _, entry := range gotBundle.Entry {
		if GetResourceType(entry.Resource) == "Patient" {
			patient := entry.Resource.(*models.Patient)
			f.Equal("Clint", patient.Name[0].Given[0])
			f.Equal("Abbott", patient.Name[0].Family)
		}
	}
}

func (f *FHIRUtilTestSuite) TestGetResource() {
	fixture, err := LoadResource("OperationOutcome", "../fixtures/operation_outcomes/oo_0.json")
	f.NoError(err)
	put, err := PostResource(f.FHIRServer.URL, "OperationOutcome", fixture)
	f.NoError(err)
	oo, ok := put.(*models.OperationOutcome)
	f.True(ok)

	got, err := GetResource(f.FHIRServer.URL, "OperationOutcome", oo.Id)
	f.NoError(err)
	gotoo, ok := got.(*models.OperationOutcome)
	f.True(ok)
	f.Equal(oo.Id, gotoo.Id)
	f.Len(gotoo.Issue, 1)

	issue := gotoo.Issue[0]
	f.Equal("Patient:58b4265297bba9116152c7a3", issue.Diagnostics)
	f.Len(issue.Location, 1)
	f.Equal("foo.bar[0].x", issue.Location[0])
}

func (f *FHIRUtilTestSuite) TestPostResource() {
	fixture, err := LoadResource("Patient", "../fixtures/patients/foo_bar.json")
	f.NoError(err)
	put, err := PostResource(f.FHIRServer.URL, "Patient", fixture)
	f.NoError(err)
	putPatient, ok := put.(*models.Patient)
	f.True(ok)

	got, err := GetResource(f.FHIRServer.URL, "Patient", putPatient.Id)
	f.NoError(err)
	gotPatient, ok := got.(*models.Patient)
	f.True(ok)
	f.Equal(putPatient.Id, gotPatient.Id)
	f.Equal(putPatient.Name[0].Given[0], gotPatient.Name[0].Given[0])
	f.Equal(putPatient.Name[0].Family, gotPatient.Name[0].Family)
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

func (f *FHIRUtilTestSuite) TestDeleteResourceByURL() {
	fixture, err := LoadResource("Bundle", "../fixtures/bundles/clint_abbott_bundle.json")
	f.NoError(err)
	put, err := PostResource(f.FHIRServer.URL, "Bundle", &fixture)
	f.NoError(err)
	putBundle, ok := put.(*models.Bundle)
	f.True(ok)

	err = DeleteResourceByURL(f.FHIRServer.URL + "/Bundle/" + putBundle.Id)
	f.NoError(err)
}

func (f *FHIRUtilTestSuite) TestDeleteResource() {
	fixture, err := LoadResource("Bundle", "../fixtures/bundles/clint_abbott_bundle.json")
	f.NoError(err)
	put, err := PostResource(f.FHIRServer.URL, "Bundle", &fixture)
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

func (f *FHIRUtilTestSuite) TestLoadAndPostResource() {
	created, err := LoadAndPostResource(f.FHIRServer.URL, "Bundle", "../fixtures/bundles/lowell_abbott_bundle.json")
	f.NoError(err)
	bundle, ok := created.(*models.Bundle)
	f.True(ok)
	f.Len(bundle.Entry, 7)
}
