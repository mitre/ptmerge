package server

import "github.com/gin-gonic/gin"

// PTMergeServer contains the router and database connection needed to server the
// patient merging service.
type PTMergeServer struct {
	Engine *gin.Engine
}

// NewServer returns a newly initialized PTMergeServer.
func NewServer() *PTMergeServer {
	return &PTMergeServer{
		Engine: gin.Default(),
	}
}

// Run sets up the routing and starts the server.
func (p *PTMergeServer) Run() {
	RegisterRoutes(p.Engine)
	p.Engine.Run(":5000")
}
