# ptmerge [![Build Status](https://travis-ci.org/mitre/ptmerge.svg?branch=master)](https://travis-ci.org/mitre/ptmerge)
A service for merging two HL7 FHIR patient records. Can be coupled with the [`ptmerge-frontend`](https://github.com/mitre/ptmerge-frontend).

## Dependencies

1. **MongoDB** - This project requires MongoDB 3.2.\* or higher. To install MongoDB, refer to the
[MongoDB installation guide](http://docs.mongodb.org/manual/installation/).
2. **FHIR Server** - ptmerge also requires a running host FHIR server. To install and start a go-based FHIR server, refer to the [GoFHIR Installation Instructions](https://github.com/synthetichealth/gofhir).

## Environment

This project currently uses Go 1.8 and is built using the Go toolchain.

To install Go, follow the instructions found at the [Go Website](http://golang.org/doc/install).

Following standard Go practices, you should clone this project to:

```
$GOPATH/src/github.com/mitre/ptmerge
```

Assuming your working directory is `$GOPATH/src/github.com/mitre`, the git command will look like:

```
git clone https://github.com/mitre/ptmerge.git
```

This project uses [Glide](https://github.com/Masterminds/glide) to manage dependencies. You can install glide using `go get`:

```
go get github.com/Masterminds/glide
```

Mac users can also install Glide using homebrew:

```
brew install glide
```

Once Glide is installed, install all of the project's dependencies:

```
glide install
```

To run all of the tests for this project, run:

```
go test $(glide novendor)
```

in this directory.


## Running the Application

To start the application, simply run ptmerge.go:

```
go run ptmerge.go
```

The ptmerge command also has several configuration options, with reasonable defaults:

```
Usage of ./ptmerge:
  -dbhost string
    	The Mongo database used to host the ptmerge service (default "localhost:27017")
  -dbname string
    	The name of the Mongo database (default "ptmerge")
  -debug
    	Run the ptmerge service in debug mode (more verbose output)
  -fhirhost string
    	The FHIR server used to host the ptmerge service (default "http://localhost:3001")

```

## License
Copyright 2017 The MITRE Corporation

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

MITRE Public Release: 16-3601
