// This library ported from davedoesdev/qlobber
//
// Copyright (c) 2013 David Halls <https://github.com/davedoesdev/>
// Copyright (c) 2014 Shuhei Tanuma <https://github.com/chobie/>
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is furnished
// to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package util

import (
	"strings"
	"reflect"
	"sync"
	"fmt"
	"io"
)

type QlobberTrie struct {
	Collections []interface{}
	Trie map[string]*QlobberTrie
}

type Qlobber struct {
	Separator string
	WildcardSome string
	WildcardOne string
	QlobberTrie *QlobberTrie
	Mutex *sync.RWMutex
	Cache map[string][]interface {}
}

func NewQlobber() *Qlobber {
	q := &Qlobber{
		QlobberTrie: &QlobberTrie{
			Collections: make([]interface{}, 0),
			Trie: make(map[string]*QlobberTrie),
		},
		Cache: make(map[string][]interface{}),
		Mutex: &sync.RWMutex{},
	}
	q.Separator = "/"
	q.WildcardOne = "+"
	q.WildcardSome = "#"
	return q
}

func (self *Qlobber) Match(Topic string) []interface{} {
	self.Mutex.RLock()
//	if v, ok := self.Cache[Topic]; ok && v != nil {
//		self.Mutex.Unlock()
//		return v
//	}

	var v []interface{}
	result := self.match(v, 0, strings.Split(Topic, self.Separator), self.QlobberTrie);

//	self.Cache[Topic] = result
	self.Mutex.RUnlock()
	return result
}

func (self *Qlobber) match(v []interface{}, length int, words []string, sub_trie *QlobberTrie) []interface{} {
	if st, ok := sub_trie.Trie[self.WildcardSome]; ok {
		for offset := range st.Collections {
			w := st.Collections[offset]
			if w != self.Separator {
				for j := length; j < len(words); j++ {
					v = self.match(v, j, words, st)
				}
				break
			}
		}
		v = self.match(v, len(words), words, st)
	}

	if length == len(words) {
		v = append(v, sub_trie.Collections...)
	} else {
		word := words[length];
		if word != self.WildcardOne && word != self.WildcardSome {
			if st, ok := sub_trie.Trie[word]; ok {
				v = self.match(v, length + 1, words, st)
			}
		}

		if st, ok := sub_trie.Trie[self.WildcardOne]; ok {
			v = self.match(v, length + 1, words, st)
		}
	}

	return v
}

func (self *Qlobber) Add(Topic string, Value interface{}) {
	self.Mutex.Lock()
	self.Cache[Topic] = nil

	self.add(Value, 0, strings.Split(Topic, self.Separator), self.QlobberTrie)
	self.Mutex.Unlock()
}

func (self *Qlobber) add(Value interface{}, length int, words []string, sub_trie *QlobberTrie) {
	if length == len(words) {
		sub_trie.Collections = append(sub_trie.Collections, Value)
		return
	}
	word := words[length]

	var st *QlobberTrie
	var ok bool
	if st, ok = sub_trie.Trie[word]; !ok {
		sub_trie.Trie[word] = &QlobberTrie{
			Collections: make([]interface{}, 0),
			Trie: make(map[string]*QlobberTrie),
		}
		st = sub_trie.Trie[word]
	}

	self.add(Value, length + 1, words, st)
}

func (self *Qlobber) remove(val interface{}, i int, words []string, sub_trie *QlobberTrie) {
	if i == len(words) {
		if val == nil {
			sub_trie.Collections = make([]interface{}, 0)
			sub_trie.Trie = make(map[string]*QlobberTrie)
		} else {
			switch val.(type){
			case string:
				for o := 0; o < len(sub_trie.Collections); o++ {
					sf1 := sub_trie.Collections[o].(string)
					if sf1 == val {
						sub_trie.Collections = append(sub_trie.Collections[:o], sub_trie.Collections[o+1:]...)
					}
				}
			// TODO: 対応していない奴はpanicしておきたいんだけど
			default:
				sf2 := reflect.ValueOf(val)
				for o := 0; o < len(sub_trie.Collections); o++ {
					sf1 := reflect.ValueOf(sub_trie.Collections[o])
					if sf1.Pointer() == sf2.Pointer() {
						sub_trie.Collections = append(sub_trie.Collections[:o], sub_trie.Collections[o+1:]...)
					}
				}
			}
		}

		if len(sub_trie.Trie) == 0 {
			delete(sub_trie.Trie, self.Separator)
		}

		return
	}

	word := words[i];

	var st *QlobberTrie
	var ok bool
	if st, ok = sub_trie.Trie[word]; !ok {
		return
	}
	self.remove(val, i + 1, words, st);
	for _ = range st.Trie {
		return
	}
	for _ = range st.Collections {
		return
	}
	delete(sub_trie.Trie, word)
}

func (self *Qlobber) Remove(Topic string, val interface{}) {
	self.Mutex.Lock()
	self.Cache[Topic] = nil

	self.remove(val, 0, strings.Split(Topic, self.Separator), self.QlobberTrie);
	self.Mutex.Unlock()
}

func (self *Qlobber) Dump(writer io.Writer) {
	self.Mutex.RLock()
	self.dump(self.QlobberTrie, 0, writer)
	self.Mutex.RUnlock()
}


func (self *Qlobber) dump(sub_trie *QlobberTrie, level int, writer io.Writer) {
	ls := strings.Repeat(" ", level * 2)
	for offset := range sub_trie.Collections {
		w := sub_trie.Collections[offset]
		fmt.Fprintf(writer, "%s`%s\n", ls, w)
	}

	for k, v := range sub_trie.Trie {
		if k == "" {
			k = "root"
		}
		fmt.Fprintf(writer, "%s[%s]\n", ls, k)
		self.dump(v, level + 1, writer)
	}
}
