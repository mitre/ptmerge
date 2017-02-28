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
	TargetURL string      `bson:"targetBundle,omitempty" json:"targetBundle,omitempty"`
	Conflicts ConflictMap `bson:"conflicts,omitempty" json:"conflicts,omitempty"`
	Completed bool        `bson:"completed" json:"completed"`
}

// ConflictMap is a map containing one or more ConflictStates. The key to each
// ConflictState is that conflict's ID.
type ConflictMap map[string]*ConflictState

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

// RemainingConflicts returns a slice of IDs for the remaining unresolved conflicts.
func (c ConflictMap) RemainingConflicts() []string {
	remaining := []string{}
	for k := range c {
		if !c[k].Resolved {
			remaining = append(remaining, k)
		}
	}
	return remaining
}

// ResolvedConflicts returns a slice of IDs for the resolved conflicts.
func (c ConflictMap) ResolvedConflicts() []string {
	resolved := []string{}
	for k := range c {
		if c[k].Resolved {
			resolved = append(resolved, k)
		}
	}
	return resolved
}

// ConflictState represents the current state of a single merge conflict as it is
// store in mongo. This is embedded in the MergeState object as a ConflictMap.
type ConflictState struct {
	OperationOutcomeURL string         `bson:"operationOutcome,omitempty" json:"operationOutcome,omitempty"`
	TargetResource      TargetResource `bson:"targetResource,omitempty" json:"targetResource,omitempty"`
	Resolved            bool           `bson:"resolved" json:"resolved"`
}

// TargetResource represents a single resource in a target bundle.
type TargetResource struct {
	ResourceID   string `bson:"id" json:"id"`
	ResourceType string `bson:"type" json:"type"`
}
