package server

// MergeState represents the current state of a merge as it is stored in mongo.
// This is stored in the "merges" collection.
type MergeState struct {
	MergeID      string      `bson:"_id" json:"id,omitempty"`
	TargetBundle string      `bson:"target" json:"target_bundle,omitempty"`
	Conflicts    ConflictMap `bson:"conflicts" json:"conflicts,omitempty"`
}

// ConflictMap maps a conflictID to a FHIR URI, e.g.:
// "conflicts": {
//     "5890c15c97bba9f2d4a2b471": "http://example.com/fhir/OperationOutcome/5890c15c97bba9f2d4a2b471"
// }
// This ensures complete unambiguity when determining where to find a merge conflict.
type ConflictMap map[string]string

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
