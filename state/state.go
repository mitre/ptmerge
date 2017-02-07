package state

import "time"

// Merges represents all metadata for all merges.
type Merges struct {
	Timestamp time.Time    `json:"timestamp,omitempty"`
	Merges    []MergeState `json:"merges"`
}

// Merge represents the metadata for a single merge.
type Merge struct {
	Timestamp time.Time  `json:"timestamp,omitempty"`
	Merge     MergeState `json:"merge,omitempty"`
}

// MergeState represents the current state of a merge as it is stored in mongo.
// This is stored in the "merges" collection.
type MergeState struct {
	MergeID   string      `bson:"_id,omitempty" json:"id,omitempty"`
	Completed bool        `bson:"completed" json:"completed"`
	TargetURL string      `bson:"target,omitempty" json:"target,omitempty"`
	Conflicts ConflictMap `bson:"conflicts,omitempty" json:"conflicts,omitempty"`
}

// ConflictState represents the current state of a single merge conflict as it is
// store in mongo. This is embedded in the MergeState object as a ConflictMap.
type ConflictState struct {
	URL      string `bson:"url,omitempty" json:"url,omitempty"`
	Resolved bool   `bson:"resolved" json:"resolved"`
}

// ConflictMap is a map containing one or more ConflictStates. The key to each
// ConflictState is that conflict's ID.
type ConflictMap map[string]ConflictState

// Keys returns all keys (conflict IDs) in the map.
func (c ConflictMap) Keys() []string {
	keys := make([]string, len(c))
	i := 0
	for k := range c {
		keys[i] = k
		i++
	}
	return keys
}
