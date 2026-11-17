//go:build !darwin || !cgo

package browserdata

type row struct{}

func (row) text(int) string { return "" }
func (row) int(int) int64   { return 0 }

func query(string, string, func(row)) error { return ErrNotOnThisSystem }
