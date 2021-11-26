package parse

type KeyMatcher interface {
	Match(string) bool
}
