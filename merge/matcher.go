package merge

import (
	"fmt"

	"github.com/intervention-engine/fhir/models"
)

type Matcher struct{}

func (m *Matcher) Match(leftResources []interface{}, rightResources []interface{}, matchStrategy MatchStrategy) (matches []Match, noMatches []interface{}, err error) {
	isMatch := false
	//rightResource = interface{}

	for i := 0; i < len(leftResources); i++ {
		for k := 0; i < len(rightResources); k++ {
			//rightResource = rightResources[k]
			if MatchPatient(leftResources[i], rightResources[k]) {
				isMatch = true
				break
			}
		}

		if isMatch {
			//matches = append(matches, Match{leftResources[i], rightResource})
		} else {
			noMatches = append(noMatches, leftResources[i])
		}
	}
	return matches, noMatches, err
}

func MatchPatient(left interface{}, right interface{}) bool {
	source1 := left.(*models.Patient)
	source2 := right.(*models.Patient)

	fmt.Println(source1.Gender + ", " + source2.Gender)
	return true
}
