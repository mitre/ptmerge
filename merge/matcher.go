package merge

type Matcher struct{}

func (m *Matcher) Match(leftResources []interface{}, rightResources []interface{}, matchStrategy MatchStrategy) (matches []Match, noMatches []interface{}, err error) {
}
