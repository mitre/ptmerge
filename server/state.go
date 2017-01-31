package server

// MergeState represents the current state of a merge as it is stored in mongo.
type MergeState struct {
	MergeID        string   `bson:"_id" json:"id,omitempty"`
	TargetBundleID string   `bson:"target" json:"target,omitempty"`
	ConflictIDs    []string `bson:"conflicts" json:"conflicts,omitempty"`
}
