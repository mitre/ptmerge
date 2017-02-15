package merge

import (
	"fmt"

	"github.com/intervention-engine/fhir/models"
)

type MatchingStrategy interface {
	// SupportedResourceType returns the name of the resource that this strategy supports.
	SupportedResourceType() string
	// Match returns true if the left resource matches the right resource.
	Match(left interface{}, right interface{}) (isMatch bool, err error)
}

// ========================================================================= //
// PATIENT MATCHING STRATEGY                                                 //
// ========================================================================= //

type PatientMatchingStrategy struct{}

func (p *PatientMatchingStrategy) SupportedResourceType() string {
	return "Patient"
}

func (p *PatientMatchingStrategy) Match(left interface{}, right interface{}) (isMatch bool, err error) {

	leftResource, ok := left.(*models.Patient)
	if !ok {
		return false, fmt.Errorf("Left resource was not a Patient resource")
	}
	rightResource, ok := right.(*models.Patient)
	if !ok {
		return false, fmt.Errorf("Right resource was not a Patient resource")
	}

	// Define the threshold at which we can declare two resources to be a match
	matchThreshold := .9
	matchCtr := 0.0
	const matchCriteriaTotal = 6.0

	// Property comparison
	if len(leftResource.Name) > 0 && len(rightResource.Name) > 0 {
		if leftResource.Name[0].Given[0] == rightResource.Name[0].Given[0] && leftResource.Name[0].Family[0] == rightResource.Name[0].Family[0] {
			matchCtr++
		}
	}

	if leftResource.Active != nil && rightResource.Active != nil {
		if leftResource.Active == rightResource.Active {
			matchCtr++
		}
	}

	if leftResource.Gender != "" && rightResource.Gender != "" {
		if leftResource.Gender == rightResource.Gender {
			matchCtr++
		}
	}

	if leftResource.BirthDate != nil && rightResource.BirthDate != nil {
		if leftResource.BirthDate.Time == rightResource.BirthDate.Time {
			matchCtr++
		}
	}

	if leftResource.DeceasedBoolean != nil && rightResource.DeceasedBoolean != nil {
		if leftResource.DeceasedBoolean == rightResource.DeceasedBoolean {
			matchCtr++
		}
	}

	if leftResource.MaritalStatus != nil && rightResource.MaritalStatus != nil {
		if leftResource.MaritalStatus.Text == rightResource.MaritalStatus.Text {
			matchCtr++
		}
	}

	if len(leftResource.Telecom) > 0 && len(rightResource.Telecom) > 0 {
		if len(leftResource.Telecom) == len(rightResource.Telecom) {
			for i := 0; i < len(leftResource.Telecom); i++ {
				if leftResource.Telecom[i] == rightResource.Telecom[i] {
					matchCtr++
				}
			}
		}
	}

	if len(leftResource.Address) > 0 && len(rightResource.Address) > 0 {
		if len(leftResource.Address) == len(rightResource.Address) {
			addMatchCtr := 0.0
			addMatchCriteriaTotal := 8.0
			for i := 0; i < len(leftResource.Address); i++ {
				if leftResource.Address[i].Use == rightResource.Address[i].Use {
					addMatchCtr++
				}

				if leftResource.Address[i].Type == rightResource.Address[i].Type {
					addMatchCtr++
				}

				if leftResource.Address[i].Text == rightResource.Address[i].Text {
					addMatchCtr++
				}

				if leftResource.Address[i].City == rightResource.Address[i].City {
					addMatchCtr++
				}

				if leftResource.Address[i].District == rightResource.Address[i].District {
					addMatchCtr++
				}

				if leftResource.Address[i].State == rightResource.Address[i].State {
					addMatchCtr++
				}

				if leftResource.Address[i].PostalCode == rightResource.Address[i].PostalCode {
					addMatchCtr++
				}

				if leftResource.Address[i].Country == rightResource.Address[i].Country {
					addMatchCtr++
				}
			}
			if IsMatchFromCriteria(matchThreshold, addMatchCtr, addMatchCriteriaTotal) {
				matchCtr++
			}
		}
	}

	if leftResource.MultipleBirthBoolean != nil && rightResource.MultipleBirthBoolean != nil {
		if leftResource.MultipleBirthBoolean == rightResource.MultipleBirthBoolean {
			matchCtr++
		}
	}

	if leftResource.MultipleBirthInteger != nil && rightResource.MultipleBirthInteger != nil {
		if leftResource.MultipleBirthInteger == rightResource.MultipleBirthInteger {
			matchCtr++
		}
	}

	if leftResource.Animal != nil && rightResource.Animal != nil {
		if leftResource.Animal == rightResource.Animal {
			matchCtr++
		}
	}

	if len(leftResource.Communication) > 0 && len(rightResource.Communication) > 0 {
		if len(leftResource.Communication) == len(rightResource.Communication) {
			commMatchCtr := 0.0
			commMatchCriteriaTotal := 2.0
			for i := 0; i < len(leftResource.Communication); i++ {
				if leftResource.Communication[i].Language == rightResource.Communication[i].Language {
					commMatchCtr++
				}
				if leftResource.Communication[i].Preferred == rightResource.Communication[i].Preferred {
					commMatchCtr++
				}
			}
			if IsMatchFromCriteria(matchThreshold, commMatchCtr, commMatchCriteriaTotal) {
				matchCtr++
			}
		}
	}

	if len(leftResource.Contact) > 0 && len(rightResource.Contact) > 0 {
		if len(leftResource.Contact) == len(rightResource.Contact) {
			contactMatchCtr := 0.0
			contactCriteriaTotal := 6.0

			for i := 0; i < len(leftResource.Contact); i++ {
				if leftResource.Contact[i].Relationship[0].Text == rightResource.Contact[i].Relationship[0].Text {
					contactMatchCtr++
				}
				if leftResource.Contact[i].Name == rightResource.Contact[i].Name {
					contactMatchCtr++
				}
				if len(leftResource.Telecom) == len(rightResource.Telecom) {
					for i := 0; i < len(leftResource.Telecom); i++ {
						if leftResource.Telecom[i] == rightResource.Telecom[i] {
							contactMatchCtr++
						}
					}
				}
				if leftResource.Contact[i].Address == rightResource.Contact[i].Address {
					contactMatchCtr++
				}
				if leftResource.Contact[i].Gender == rightResource.Contact[i].Gender {
					contactMatchCtr++
				}
				if leftResource.Contact[i].Organization == rightResource.Contact[i].Organization {
					contactMatchCtr++
				}
			}
			if IsMatchFromCriteria(matchThreshold, contactMatchCtr, contactCriteriaTotal) {
				matchCtr++
			}
		}
	}

	if leftResource.ManagingOrganization != nil && rightResource.ManagingOrganization != nil {
		if leftResource.ManagingOrganization == rightResource.ManagingOrganization {
			matchCtr++
		}
	}

	isMatch = IsMatchFromCriteria(matchThreshold, matchCtr, matchCriteriaTotal)
	return isMatch, err
}

func IsMatchFromCriteria(threshold, score, totalCriteria float64) bool {
	resultAvg := score / totalCriteria
	fmt.Println(resultAvg)
	return resultAvg >= threshold
}
