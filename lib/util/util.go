// Package util contains helper functions used around the code.
package util

// In returns true if s is found in ss, false otherwise
func In(ss []string, s string) bool {
	for _, v := range ss {
		if s == v {
			return true
		}
	}
	return false
}
