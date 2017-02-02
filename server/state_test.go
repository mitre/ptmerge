package server

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

type MergeStateTestSuite struct {
	suite.Suite
}

func TestMergeStateTestSuite(t *testing.T) {
	suite.Run(t, new(MergeStateTestSuite))
}

func (m *MergeStateTestSuite) TestConflictKeys() {
	conflicts := make(ConflictMap)
	conflicts["foo"] = "bar"
	conflicts["bar"] = "foo"
	conflicts["hey"] = "ho"

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
