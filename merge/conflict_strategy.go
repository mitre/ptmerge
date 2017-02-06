package merge

type ConflictStrategy interface {
	Conflicts(left interface{}, right interface{}) (locations []string, err error)
}
