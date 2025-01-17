package validatechaindb

import (
	"fmt"

	bolt "go.etcd.io/bbolt"
)

func (s *ValidateChainStore) GetBlockData(hash []byte) ([]byte, error) {
	var blockdata []byte
	err := s.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(vcBlocksBucket)
		blockdata = bkt.Get(hash)
		if blockdata == nil {
			return fmt.Errorf("No block data with hash [%x]", hash)
		}
		return nil
	})
	return blockdata, err
}

func (s *ValidateChainStore) SaveBlock(hash []byte, blockData []byte) error {
	err := s.db.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(vcBlocksBucket)
		if err := bkt.Put(hash, blockData); err != nil {
			return fmt.Errorf("could write state to db with key %#x:%v", stateKey, err)
		}
		return nil
	})
	return err
}
