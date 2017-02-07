package state

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

type StateTestSuite struct {
	suite.Suite
}

func TestStateTestSuite(t *testing.T) {
	suite.Run(t, new(StateTestSuite))
}

func (m *StateTestSuite) TestConflictKeys() {
	conflicts := make(ConflictMap)
	conflicts["foo"] = &ConflictState{
		URL:      "foo",
		Resolved: true,
	}
	conflicts["bar"] = &ConflictState{
		URL:      "bar",
		Resolved: true,
	}
	conflicts["hey"] = &ConflictState{
		URL:      "hey",
		Resolved: true,
	}

	expected := []string{"foo", "bar", "hey"} // Not necessarily in this order
	keys := conflicts.Keys()
	for _, k := range keys {
		m.True(contains(expected, k))
	}
}

func contains(set []string, value string) bool {
	for _, val := range set {
		if strings.Compare(val, value) == 0 {
			return true
		}
	}
	return false
}
