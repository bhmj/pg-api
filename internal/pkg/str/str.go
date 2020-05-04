package str

import "strconv"

// CommaSeparatedString returns values from a converted to a single comma-separated string
func CommaSeparatedString(a []int64) string {
	buf := make([]byte, 0, 16)
	for i, v := range a {
		if i > 0 {
			buf = append(buf, []byte{',', ' '}...)
		}
		s := strconv.FormatInt(v, 10)
		buf = append(buf, []byte(s)...)
	}
	return string(buf)
}

// Scoalesce returns first non-empty string
func Scoalesce(str1 string, str2 string) string {
	if len(str1) > 0 {
		return str1
	}
	return str2

}

// Icoalesce returns first non-zero integer
func Icoalesce(a int, b int) int {
	if a != 0 {
		return a
	}
	return b
}
