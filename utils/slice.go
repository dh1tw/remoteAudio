package utils

// StringInSlice returns a boolean value if a particular
// string has been found in a slice of strings.
func StringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
