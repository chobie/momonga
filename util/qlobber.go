/*
This library ported from davedoesdev/qlobber

Copyright (c) 2013 David Halls <https://github.com/davedoesdev/>
Copyright (c) 2014 Shuhei Tanuma <https://github.com/chobie/>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is furnished
to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
 */

package util

import (
	"strings"
	"reflect"
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
}

func NewQlobber() *Qlobber {
	q := &Qlobber{
		QlobberTrie: &QlobberTrie{
			Collections: make([]interface{}, 0),
			Trie: make(map[string]*QlobberTrie),
		},
	}
	q.Separator = "/"
	q.WildcardOne = "+"
	q.WildcardSome = "#"
	return q
}

func (self *Qlobber) Match(Topic string) []interface{} {
	var v []interface{}
	return self.match(v, 0, strings.Split(Topic, self.Separator), self.QlobberTrie);
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
	self.add(Value, 0, strings.Split(Topic, self.Separator), self.QlobberTrie)
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
			sf2 := reflect.ValueOf(val)
			for o := 0; o < len(sub_trie.Collections); o++ {
				sf1 := reflect.ValueOf(sub_trie.Collections[o])
				if sf1.Pointer() == sf2.Pointer() {
					sub_trie.Collections = append(sub_trie.Collections[:o], sub_trie.Collections[o+1:]...)
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
	self.remove(val, 0, strings.Split(Topic, self.Separator), self.QlobberTrie);
}
