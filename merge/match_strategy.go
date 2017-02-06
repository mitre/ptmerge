package merge

type MatchStrategy interface {
	Match(left interface{}, right interface{}) (isMatch bool, err error)
}
