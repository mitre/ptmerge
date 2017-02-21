package merge

type ConflictStrategy interface {
	// SupportedResourceType returns the name of the resource that this strategy supports.
	SupportedResourceType() string
	// FindConflicts a slice of conflict locations between the two resources in a Match.
	FindConflicts(match Match) (locations []string, err error)
}
