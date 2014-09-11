// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package skiplist

func (self *SkipList) Iterator() *SkipListIterator {
	return &SkipListIterator{
		Node:   self.Header.Level[0].Forward,
		Parent: self,
	}
}

type SkipListIterator struct {
	Node   *SkipListNode
	Parent *SkipList
}

func (self *SkipListIterator) Seek(score interface{}) {
	node := self.Parent.Header

	for i := self.Parent.Level - 1; i >= 0; i-- {
		for node.Level[i].Forward != nil &&
			self.Parent.Comparator.Compare(node.Level[i].Forward.Score, score) == -1 {
			node = node.Level[i].Forward
		}
	}

	self.Node = node.Level[0].Forward
}

func (self *SkipListIterator) Key() interface{} {
	if self.Node == nil {
		return nil
	}

	return self.Node.Score
}

func (self *SkipListIterator) Value() interface{} {
	if self.Node == nil {
		return nil
	}

	return self.Node.Data
}

func (self *SkipListIterator) Valid() bool {
	if self.Node != nil {
		return true
	} else {
		return false
	}
}

func (self *SkipListIterator) Next() {
	if v, ok := self.Node.Level[0]; ok {
		self.Node = v.Forward
	} else {
		self.Node = nil
	}
}

func (self *SkipListIterator) Rewind() {
	self.Node = self.Parent.Header.Level[0].Forward
}
