package indexer

import (
	"github.com/dgraph-io/badger/v4"
	"github.com/sat20-labs/satoshinet/indexer/common"

	db "github.com/sat20-labs/indexer/indexer/db"
)

func openDB(filepath string, opts badger.Options) (ldb *badger.DB, err error) {
	opts = opts.WithDir(filepath).WithValueDir(filepath).WithLoggingLevel(badger.WARNING)
	ldb, err = badger.Open(opts)
	if err != nil {
		return nil, err
	}
	common.Log.Infof("InitDB-> start db gc for %s", filepath)
	db.RunBadgerGC(ldb)
	return ldb, nil
}

func (p *IndexerMgr) initDB() (err error) {
	common.Log.Info("InitDB-> start...")

	opts := badger.DefaultOptions("").WithBlockCacheSize(3000 << 20)
	p.baseDB, err = openDB(p.dbDir+"base", opts)
	if err != nil {
		return err
	}

	p.localDB, err = openDB(p.dbDir+"local", opts)
	if err != nil {
		return err
	}

	return nil
}
