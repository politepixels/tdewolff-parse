//go:build gofuzz
// +build gofuzz

package fuzz

import "github.com/politepixels/tdewolff-parse/v2"

// Fuzz is a fuzz test.
func Fuzz(data []byte) int {
	_ = parse.Number(data)
	return 1
}
