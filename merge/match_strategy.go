package merge

import "github.com/intervention-engine/fhir/models"
import "fmt"

type MatchStrategy interface {
	Match(left interface{}, right interface{}) (isMatch bool, err error)
}

type PatientMatchStrategy struct{}

func (p *PatientMatchStrategy) Match(left interface{}, right interface{}) (isMatch bool, err error) {

	leftResource, ok := left.(*models.Patient)
	if !ok {
		fmt.Println("Incorrect format for Patient resource")
	}
	rightResource, ok := right.(*models.Patient)
	if !ok {
		fmt.Println("Incorrect format for Patient resource")
	}

	// Define the threshold at which we can declare two resources to be a match
	matchThreshold := .9
	matchCtr := 0.0
	const matchCriteriaTotal = 6.0

	// Property comparison
	if leftResource.Name[0].Given[0] == rightResource.Name[0].Given[0] && leftResource.Name[0].Family[0] == rightResource.Name[0].Family[0] {
		matchCtr++
		fmt.Println("Name: match")
	}

	if leftResource.Active == rightResource.Active {
		matchCtr++
		fmt.Println("Active: match")
	}

	if leftResource.Gender == rightResource.Gender {
		matchCtr++
		fmt.Println("Gender: match")
	}

	if leftResource.BirthDate.Time == rightResource.BirthDate.Time {
		matchCtr++
		fmt.Println("Birthdate: match")
	}

	if leftResource.DeceasedBoolean == rightResource.DeceasedBoolean {
		matchCtr++
		fmt.Println("DeceasedBoolean: match")
	}

	if leftResource.MaritalStatus.Text == rightResource.MaritalStatus.Text {
		matchCtr++
		fmt.Println("MaritalStatus: match")
	}

	matchRatio := (matchCtr / matchCriteriaTotal)

	if matchRatio >= matchThreshold {
		isMatch = true
	}

	return isMatch, err
}
