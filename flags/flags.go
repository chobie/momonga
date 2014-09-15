// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.
package flags

import (
	log "github.com/chobie/momonga/logger"
)

var (
	Mflags map[string]bool
)

func init() {
	Mflags = make(map[string]bool)
	// リトライできるQoS1をゆうこうにする
	// これがまたつくりのおかげで色々な問題があるんだわー。
	// newidもonにしないと動きません
	//
	// このモデル(配送制御をgoroutineでやる)はスケールしないのでダメ
	Mflags["experimental.qos1"] = false

	// newidをonにするとidentifier:guidという形式にする
	// これだとdetach時にidentifierに戻してやりなおして、再度attach出来るようになる。
	// 複数コネクションをひとつのコネクションに束ねることはできなくなるけど（そんな仕様なかったよね？）
	// まぁ、たぶんこっちが本ちゃんなのかなー、と。
	Mflags["experimental.newid"] = true

	log.Debug("=============EXPERIMENTAL FLAGS=============")
	for k, v := range Mflags {
		log.Debug("%s: %t", k, v)
	}
	log.Debug("============================================")
}
