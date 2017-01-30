package merge

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type MergerTestSuite struct {
	suite.Suite
}

func TestMergerTestSuite(t *testing.T) {
	suite.Run(t, new(MergerTestSuite))
}

func (m *MergerTestSuite) TestMerge() {
	merger := new(Merger)
	mergeID, outcome, err := merger.Merge("12345", "67890")
	m.Equal("", mergeID)
	m.Nil(outcome)
	m.Nil(err)
}

func (m *MergerTestSuite) TestResolveConflict() {
	merger := new(Merger)
	outcome, err := merger.ResolveConflict("12345", "67890", nil)
	m.Nil(outcome)
	m.Nil(err)
}

func (m *MergerTestSuite) TestAbort() {
	merger := new(Merger)
	outcome, err := merger.Abort("12345")
	m.Nil(outcome)
	m.Nil(err)
}
