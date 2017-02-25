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
		OperationOutcomeURL: "foo",
		Resolved:            true,
	}
	conflicts["bar"] = &ConflictState{
		OperationOutcomeURL: "bar",
		Resolved:            true,
	}
	conflicts["hey"] = &ConflictState{
		OperationOutcomeURL: "hey",
		Resolved:            true,
	}

	expected := []string{"foo", "bar", "hey"} // Not necessarily in this order
	keys := conflicts.Keys()
	m.Len(keys, 3)
	for _, k := range keys {
		m.True(contains(expected, k))
	}
}

func (m *StateTestSuite) TestRemainingAndResolvedConflicts() {
	conflicts := make(ConflictMap)
	conflicts["foo"] = &ConflictState{
		OperationOutcomeURL: "foo",
		Resolved:            false,
	}
	conflicts["bar"] = &ConflictState{
		OperationOutcomeURL: "bar",
		Resolved:            false,
	}
	conflicts["hey"] = &ConflictState{
		OperationOutcomeURL: "hey",
		Resolved:            true,
	}

	expected := []string{"foo", "bar"} // Not necessarily in this order
	remaining := conflicts.RemainingConflicts()
	m.Len(remaining, 2)
	for _, r := range remaining {
		m.True(contains(expected, r))
	}

	resolved := conflicts.ResolvedConflicts()
	m.Len(resolved, 1)
	m.Equal("hey", resolved[0])
}

func contains(set []string, value string) bool {
	for _, val := range set {
		if strings.Compare(val, value) == 0 {
			return true
		}
	}
	return false
}
