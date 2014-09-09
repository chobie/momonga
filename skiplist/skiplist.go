/*
 * Copyright (c) 2014 Shuhei Tanuma <https://github.com/chobie/>
 *
 * This skiplist implementation is almost re implementation of the redis's
 * Skiplist implementation. As to understanding Skiplist algorithm and easy to re use.
 *
 * see also redis/src/t_zset.c
 *
 * Copyright (c) 2009-2012, Salvatore Sanfilippo <antirez at gmail dot com>
 * Copyright (c) 2009-2012, Pieter Noordhuis <pcnoordhuis at gmail dot com>
 *  All rights reserved.
 */
package skiplist

import (
	"fmt"
	"math/rand"
)

const (
	MAX_LEVEL = 32
)

type Comparator interface {
	// NOTE: skiplist will be ascending order if a < b, otherwise descending
	Compare(interface{}, interface{}) int
}

type SkipListLevel struct {
	Forward *SkipListNode
	Span    int
}

type SkipListNode struct {
	Score interface{}
	Data  interface{}
	Level map[int]*SkipListLevel
}

type SkipList struct {
	Header     *SkipListNode
	Tail       *SkipListNode
	Length     int
	MaxLevel   int
	Level      int
	Comparator Comparator
}

func CreateSkiplistNode(level int, score interface{}, data interface{}) (*SkipListNode, error) {
	node := &SkipListNode{
		Level: map[int]*SkipListLevel{},
	}

	for i := 0; i < level; i++ {
		node.Level[i] = &SkipListLevel{
			Forward: nil,
			Span:    0,
		}
	}

	node.Score = score
	node.Data = data

	return node, nil
}

func NewSkipList(c Comparator) *SkipList {
	list := &SkipList{
		MaxLevel: MAX_LEVEL,
		Level:    1,
	}
	list.Header, _ = CreateSkiplistNode(MAX_LEVEL, 0, 0)
	list.Comparator = c

	return list
}

func (self *SkipList) Insert(score, data interface{}) {
	update := make(map[int]*SkipListNode)
	rank := make(map[int]int)

	node := self.Header
	for i := self.Level - 1; i >= 0; i-- {
		for node.Level[i].Forward != nil &&
			self.Comparator.Compare(node.Level[i].Forward.Score, score) == -1 {
			rank[i] += node.Level[i].Span
			node = node.Level[i].Forward
		}

		update[i] = node
	}

	level := self.getRandomLevel()
	if level > self.Level {
		for i := self.Level; i < level; i++ {
			rank[i] = 0
			update[i] = self.Header
			update[i].Level[i].Span = self.Length
		}
		self.Level = level
	}

	add, err := CreateSkiplistNode(level, score, data)
	if err != nil {
		panic(fmt.Sprintf("Error: %s", err))
	}

	for i := 0; i < level; i++ {
		add.Level[i].Forward = update[i].Level[i].Forward
		update[i].Level[i].Forward = add
		add.Level[i].Span = update[i].Level[i].Span - (rank[i] - rank[i])
		update[i].Level[i].Span = (rank[i] - rank[i]) + 1
	}

	for i := level; i < self.Level; i++ {
		update[i].Level[i].Span++
	}
	self.Length++
}

func (self *SkipList) Delete(score interface{}) {
	update := make(map[int]*SkipListNode)

	node := self.Header
	for i := self.Level - 1; i >= 0; i-- {
		for node.Level[i].Forward != nil &&
			self.Comparator.Compare(node.Level[i].Forward.Score, score) == -1 {
			node = node.Level[i].Forward
		}
		update[i] = node
	}

	node = node.Level[0].Forward
	if node != nil && score == node.Score {
		self.deleteNode(node, score, update)
		freeSkipListNode(node)
	}
}

func (self *SkipList) deleteNode(node *SkipListNode, score interface{}, update map[int]*SkipListNode) {
	for i := 0; i < self.Level; i++ {
		if update[i].Level[i].Forward == node {
			update[i].Level[i].Span += node.Level[i].Span - 1
			update[i].Level[i].Forward = node.Level[i].Forward
		} else {
			update[i].Level[i].Span -= 1
		}
	}

	for self.Level > 1 && self.Header.Level[self.Level-1].Forward == nil {
		self.Length--
	}
	self.Length--
}

func (self *SkipList) getRandomLevel() int {
	level := 1

	for {
		v := (rand.Int() & 0xFFFF)
		if float64(v) < (0.25 * 0xFFFF) {
			level++
		} else {
			break
		}
	}

	if level < MAX_LEVEL {
		return level
	} else {
		return MAX_LEVEL
	}
}

func freeSkipListNode(node *SkipListNode) {
	node.Score = nil
	node.Data = nil
	node.Level = nil
}
