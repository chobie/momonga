package skiplist

import (
	"bytes"
)

type IntComparator struct {
}

func (self *IntComparator) Compare(a, b interface{}) int {
	if ax, ok := a.(int); ok {
		if bx, ok := b.(int); ok {
			if ax == bx {
				return 0
			}
			if ax < bx {
				return -1
			} else {
				return 1
			}
		}
	}

	return -1
}

type BytesComparator struct {
}

func (self *BytesComparator) Compare(a, b interface{}) int {
	if ax, ok := a.([]byte); ok {
		if bx, ok := b.([]byte); ok {
			return bytes.Compare(ax, bx)
		}
	}

	return -1
}

type StringComparator struct {
}

func (self *StringComparator) Compare(a, b interface{}) int {
	if ax, ok := a.(string); ok {
		if bx, ok := b.(string); ok {
			if ax == bx {
				return 0
			}
			if ax < bx {
				return -1
			} else {
				return 1
			}
		}
	}

	return -1
}
