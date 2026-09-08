// Package controldialpresentation defines the optional keyboard control-dial workspace contract.
package controldialpresentation

import "sort"

type Option struct {
	Value int
	Label string
}
type Snapshot struct {
	Available bool
	Value     int
	Options   []Option
}

func Valid(s Snapshot) bool {
	if !s.Available || len(s.Options) == 0 {
		return false
	}
	seen := map[int]bool{}
	selected := false
	for _, option := range s.Options {
		if option.Label == "" || seen[option.Value] {
			return false
		}
		seen[option.Value] = true
		selected = selected || option.Value == s.Value
	}
	return selected
}
func Sorted(options map[int]string) []Option {
	keys := make([]int, 0, len(options))
	for key := range options {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	result := make([]Option, 0, len(keys))
	for _, key := range keys {
		result = append(result, Option{Value: key, Label: options[key]})
	}
	return result
}
