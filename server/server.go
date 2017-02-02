package server

import (
	"github.com/gin-gonic/gin"
	mgo "gopkg.in/mgo.v2"
	"log"
	"net/http"
	"os"
)

// PTMergeServer contains the router and database connection needed to serve the
// patient merging service.
type PTMergeServer struct {
	Engine       *gin.Engine
	FHIRHost     string
	DatabaseHost string
	DatabaseName string
	Session      *mgo.Session
}

// NewServer returns a newly initialized PTMergeServer.
func NewServer(fhirhost, dbhost, dbname string, debug bool) *PTMergeServer {
	if debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	return &PTMergeServer{
		Engine:       gin.Default(), // includes the default logging and recovery middleware
		FHIRHost:     fhirhost,
		DatabaseHost: dbhost,
		DatabaseName: dbname,
		Session:      nil,
	}
}

// Run sets up the routing, database, and FHIR server connections, then starts the server.
func (p *PTMergeServer) Run() {
	var err error
	log.Println("Starting ptmerge service...")

	// setup the host database connection
	log.Println("Connecting to mongodb...")
	session, err := mgo.Dial(p.DatabaseHost) // has a 1-minute timeout
	if err != nil {
		log.Printf("Failed to connect to mongodb at %s\n", p.DatabaseHost)
		os.Exit(1)
	}
	log.Printf("Connected to mongodb at %s\n", p.DatabaseHost)

	// this master database session should be copied using session.Copy()
	// before making requests to the database. This protects the connection
	// to mongo if for any reason a database operation times out.
	p.Session = session
	defer p.Session.Close()

	// ping the host FHIR server to make sure it's running
	log.Println("Connecting to host FHIR server...")
	_, err = http.Get(p.FHIRHost + "/metadata")
	if err != nil {
		log.Printf("Host FHIR server unavailable. Could not reach %s\n", p.FHIRHost)
		os.Exit(1)
	}
	log.Printf("Connected to host FHIR server at %s\n", p.FHIRHost)
	os.Setenv("FHIRHost", p.FHIRHost)

	// register ptmerge service routes
	RegisterRoutes(p.Engine, p.Session, p.DatabaseName, p.FHIRHost)
	log.Println("Started ptmerge service!")

	p.Engine.Run(":5000")
}
