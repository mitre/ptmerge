package merge

import (
	"reflect"
	"testing"
	"time"

	"github.com/mitre/ptmerge/testutil"
	"github.com/stretchr/testify/suite"
)

type MergeTypesTestSuite struct {
	suite.Suite
}

func TestMergeTypesTestSuite(t *testing.T) {
	suite.Run(t, new(MergeTypesTestSuite))
}

func (m *MergeTypesTestSuite) TestGetResourceType() {
	test, err := testutil.LoadFixture("Patient", "../fixtures/patients/foo_bar.json")
	m.NoError(err)
	typeAsString := getResourceType(test)
	m.Equal("Patient", typeAsString)
}

func (m *MergeTypesTestSuite) TestResourceMapKeys() {
	rm := make(ResourceMap)
	rm["foo"] = []interface{}{1}
	rm["bar"] = []interface{}{1, 2}
	rm["hey"] = []interface{}{1, 2, 3}

	expected := []string{"foo", "bar", "hey"} // Not necessarily in this order
	keys := rm.Keys()
	for _, k := range keys {
		m.True(contains(expected, k))
	}
}

func (m *MergeTypesTestSuite) TestPathMapKeys() {
	pm := make(PathMap)
	pm["foo"] = reflect.ValueOf(uint32(1))
	pm["bar"] = reflect.ValueOf("hello")
	pm["baz"] = reflect.ValueOf(time.Now())

	expected := []string{"foo", "bar", "baz"} // Not necessarily in this order
	keys := pm.Keys()
	for _, k := range keys {
		m.True(contains(expected, k))
	}
}

func (m *MergeTypesTestSuite) TestSetIntersection() {
	left := []string{"a", "b", "c"}
	right := []string{"b", "c", "d"}

	ints := intersection(left, right)
	m.Equal([]string{"b", "c"}, ints)
}

func (m *MergeTypesTestSuite) TestSetDiff() {
	left := []string{"a", "b", "c"}
	right := []string{"b", "c", "d"}

	lDiffs := setDiff(left, right)
	m.Equal([]string{"a"}, lDiffs)

	rDiffs := setDiff(right, left)
	m.Equal([]string{"d"}, rDiffs)
}

func (m *MergeTypesTestSuite) TestSetContains() {
	set := []string{"a", "b", "c"}
	m.True(contains(set, "b"))
	m.False(contains(set, "d"))
}
