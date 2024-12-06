package validatechain

import (
	"bytes"

	"github.com/sat20-labs/satsnet_btcd/chaincfg/chainhash"
	"github.com/sat20-labs/satsnet_btcd/mining/posminer/utils"
)

type ValidateChainState struct {
	LatestHeight int64
	LatestHash   chainhash.Hash
}

func (vcs *ValidateChainState) Encode() ([]byte, error) {
	// Encode the VC state payload.
	var bw bytes.Buffer
	err := utils.WriteElements(&bw, vcs.LatestHeight, vcs.LatestHash)
	if err != nil {
		return nil, err
	}

	payload := bw.Bytes()
	return payload, nil
}

func (vcs *ValidateChainState) Decode(stateData []byte) error {

	br := bytes.NewReader(stateData)
	err := utils.ReadElements(br, &vcs.LatestHeight, &vcs.LatestHash)

	return err
}
