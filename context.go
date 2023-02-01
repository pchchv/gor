package gor

// contextKey is a value to be used with context.WithValue.
// It is used as a pointer, so it is placed in the interface{} without being selected.
type contextKey struct {
	name string
}

// RouteParams is a structure to track URL routing parameters efficiently.
type RouteParams struct {
	Keys   []string
	Values []string
}

func (k *contextKey) String() string {
	return "chi context value " + k.name
}

// Add will append a URL parameter to the end of the route param
func (s *RouteParams) Add(key, value string) {
	s.Keys = append(s.Keys, key)
	s.Values = append(s.Values, value)
}
