package merge

import (
	"net/http/httptest"
	"testing"

	"fmt"

	"github.com/mitre/ptmerge/testutil"
	"github.com/stretchr/testify/suite"
)

type MatchTestSuite struct {
	testutil.MongoSuite
	FHIRServer *httptest.Server
}

func TestMatchTestSuite(t *testing.T) {
	suite.Run(t, new(MatchTestSuite))
}

func (m *MergerTestSuite) TestMatch() {
	source1 := "../fixtures/clint_abbot_bundle.json"
	source2 := "../fixtures/john_peters_bundle.json"

	fmt.Println(source1 + ", " + source2)
}
