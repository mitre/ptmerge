package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"io/ioutil"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/suite"
)

type ServerTestSuite struct {
	suite.Suite
	Engine *gin.Engine
	Server *httptest.Server
}

func TestServerTestSuite(t *testing.T) {
	suite.Run(t, new(ServerTestSuite))
}

func (s *ServerTestSuite) SetupSuite() {
	// set gin to release mode (less verbose output)
	gin.SetMode(gin.ReleaseMode)

	// build routes for testing
	s.Engine = gin.New()
	RegisterRoutes(s.Engine)

	// create HTTP test server
	s.Server = httptest.NewServer(s.Engine)
}

func (s *ServerTestSuite) TeardownSuite() {
	s.Server.Close()
}

func (s *ServerTestSuite) TestInitiateMerge() {
	source1 := "http://www.example.com/fhir/Patient/12345"
	source2 := "http://www.example.com/fhir/Patient/67890"
	req := s.Server.URL + "/merge?source1=" + url.QueryEscape(source1) + "&source2=" + url.QueryEscape(source2)

	res, err := http.Post(req, "", nil)
	defer res.Body.Close()
	body, _ := ioutil.ReadAll(res.Body)

	s.Nil(err)
	s.Equal(200, res.StatusCode)
	s.Equal(fmt.Sprintf("Merging records %s and %s", source1, source2), string(body))
}

func (s *ServerTestSuite) TestResolveConflict() {
	mergeID := "12345"
	conflictID := "67890"
	req := s.Server.URL + "/merge/" + mergeID + "/resolve/" + conflictID

	res, err := http.Post(req, "", nil)
	defer res.Body.Close()
	body, _ := ioutil.ReadAll(res.Body)

	s.Nil(err)
	s.Equal(200, res.StatusCode)
	s.Equal(fmt.Sprintf("Resolving conflict %s for merge %s", conflictID, mergeID), string(body))
}

func (s *ServerTestSuite) TestAbortMerge() {
	mergeID := "12345"
	req := s.Server.URL + "/merge/" + mergeID + "/abort"

	res, err := http.Post(req, "", nil)
	defer res.Body.Close()
	body, _ := ioutil.ReadAll(res.Body)

	s.Nil(err)
	s.Equal(200, res.StatusCode)
	s.Equal(fmt.Sprintf("Aborting merge %s", mergeID), string(body))
}

func (s *ServerTestSuite) TestGetConflicts() {
	mergeID := "12345"
	req := s.Server.URL + "/merge/" + mergeID

	res, err := http.Get(req)
	defer res.Body.Close()
	body, _ := ioutil.ReadAll(res.Body)

	s.Nil(err)
	s.Equal(200, res.StatusCode)
	s.Equal(fmt.Sprintf("Merge conflicts for merge %s", mergeID), string(body))
}
