package main

import "github.com/mitre/ptmerge/server"

func main() {
	server := server.NewServer()
	server.Run()
}
