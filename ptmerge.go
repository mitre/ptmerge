package main

import (
	"flag"

	"github.com/mitre/ptmerge/server"
)

func main() {
	// command line flags
	fhirhost := flag.String("fhirhost", "http://localhost:3001", "The FHIR server used to host the ptmerge service")
	dbhost := flag.String("db", "localhost:27017", "The Mongo database used to host the ptmerge service")
	dbname := flag.String("dbname", "ptmerge", "The name of the Mongo database")
	debug := flag.Bool("debug", false, "Run the ptmerge service in debug mode (more verbose output)")
	flag.Parse()

	server := server.NewServer(*fhirhost, *dbhost, *dbname, *debug)
	server.Run()
}
