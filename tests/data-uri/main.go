//go:build gofuzz
// +build gofuzz

package fuzz

import "github.com/politepixels/tdewolff-parse/v2"

// Fuzz is a fuzz test.
func Fuzz(data []byte) int {
	data = parse.Copy(data)
	_, _, _ = parse.DataURI(data)
	return 1
}
