// Copyright (c) 2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package posminer

import (
	"github.com/btcsuite/btclog"
	"github.com/sat20-labs/satsnet_btcd/mining/posminer/localpeer"
	"github.com/sat20-labs/satsnet_btcd/mining/posminer/localvalidator"
	"github.com/sat20-labs/satsnet_btcd/mining/posminer/validator"
	"github.com/sat20-labs/satsnet_btcd/mining/posminer/validatormanager"
	"github.com/sat20-labs/satsnet_btcd/mining/posminer/validatorpeer"
)

// log is a logger that is initialized with no output filters.  This
// means the package will not perform any logging by default until the caller
// requests it.
var log btclog.Logger

// The default amount of logging is none.
func init() {
	DisableLog()
}

// DisableLog disables all library log output.  Logging output is disabled
// by default until UseLogger is called.
func DisableLog() {
	log = btclog.Disabled
}

// UseLogger uses a specified Logger to output package logging info.
func UseLogger(logger btclog.Logger) {
	log = logger
	validatormanager.UseLogger(logger)

	validator.UseLogger(logger)
	validatorpeer.UseLogger(logger)

	localpeer.UseLogger(logger)
	localvalidator.UseLogger(logger)
}
