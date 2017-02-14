package merge

type MatchingStrategy interface {
	// SupportedResourceType returns the name of the resource that this strategy supports.
	SupportedResourceType() string
	// Match returns true if the left resource matches the right resource.
	Match(left interface{}, right interface{}) (isMatch bool, err error)
}
