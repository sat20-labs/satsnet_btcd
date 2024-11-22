package validatormanager

import "github.com/sat20-labs/satsnet_btcd/mining/posminer/validator"

const (
	// Vote type
	VoteTypeNone = 0
	VoteTypePass = 1
	VoteTypeFail = 2
)

type Vote struct {
	VoteList []*validator.Validator
	VoteType int32 // Vote type
}

type VoteManager struct {
}
