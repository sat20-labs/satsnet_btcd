// Copyright (c) 2014-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package chaincfg

import (
	"time"

	"github.com/sat20-labs/satsnet_btcd/chaincfg/chainhash"
	"github.com/sat20-labs/satsnet_btcd/wire"
)

// genesisCoinbaseTx is the coinbase transaction for the genesis blocks for
// the main network, regression test network, and test network (version 3).
var genesisCoinbaseTx = wire.MsgTx{
	Version: 1,
	TxIn: []*wire.TxIn{
		{
			PreviousOutPoint: wire.OutPoint{
				Hash:  chainhash.Hash{},
				Index: 0xffffffff,
			},
			SignatureScript: []byte{
				0x04, 0xff, 0xff, 0x00, 0x1d, 0x01, 0x04, 0x45, /* |.......E| */
				0x54, 0x68, 0x65, 0x20, 0x54, 0x69, 0x6d, 0x65, /* |The Time| */
				0x73, 0x20, 0x30, 0x33, 0x2f, 0x4a, 0x61, 0x6e, /* |s 03/Jan| */
				0x2f, 0x32, 0x30, 0x30, 0x39, 0x20, 0x43, 0x68, /* |/2009 Ch| */
				0x61, 0x6e, 0x63, 0x65, 0x6c, 0x6c, 0x6f, 0x72, /* |ancellor| */
				0x20, 0x6f, 0x6e, 0x20, 0x62, 0x72, 0x69, 0x6e, /* | on brin| */
				0x6b, 0x20, 0x6f, 0x66, 0x20, 0x73, 0x65, 0x63, /* |k of sec|*/
				0x6f, 0x6e, 0x64, 0x20, 0x62, 0x61, 0x69, 0x6c, /* |ond bail| */
				0x6f, 0x75, 0x74, 0x20, 0x66, 0x6f, 0x72, 0x20, /* |out for |*/
				0x62, 0x61, 0x6e, 0x6b, 0x73, /* |banks| */
			},
			Sequence: 0xffffffff,
		},
	},
	TxOut: []*wire.TxOut{
		{
			Value: 0x12a05f200,
			PkScript: []byte{
				0x41, 0x04, 0x67, 0x8a, 0xfd, 0xb0, 0xfe, 0x55, /* |A.g....U| */
				0x48, 0x27, 0x19, 0x67, 0xf1, 0xa6, 0x71, 0x30, /* |H'.g..q0| */
				0xb7, 0x10, 0x5c, 0xd6, 0xa8, 0x28, 0xe0, 0x39, /* |..\..(.9| */
				0x09, 0xa6, 0x79, 0x62, 0xe0, 0xea, 0x1f, 0x61, /* |..yb...a| */
				0xde, 0xb6, 0x49, 0xf6, 0xbc, 0x3f, 0x4c, 0xef, /* |..I..?L.| */
				0x38, 0xc4, 0xf3, 0x55, 0x04, 0xe5, 0x1e, 0xc1, /* |8..U....| */
				0x12, 0xde, 0x5c, 0x38, 0x4d, 0xf7, 0xba, 0x0b, /* |..\8M...| */
				0x8d, 0x57, 0x8a, 0x4c, 0x70, 0x2b, 0x6b, 0xf1, /* |.W.Lp+k.| */
				0x1d, 0x5f, 0xac, /* |._.| */
			},
		},
	},
	LockTime: 0,
}

// genesisHash is the hash of the first block in the block chain for the main
// network (genesis block).
var genesisHash = chainhash.Hash([chainhash.HashSize]byte{ // Make go vet happy.
	0x6f, 0xe2, 0x8c, 0x0a, 0xb6, 0xf1, 0xb3, 0x72,
	0xc1, 0xa6, 0xa2, 0x46, 0xae, 0x63, 0xf7, 0x4f,
	0x93, 0x1e, 0x83, 0x65, 0xe1, 0x5a, 0x08, 0x9c,
	0x68, 0xd6, 0x19, 0x00, 0x00, 0x00, 0x00, 0x00,
})

// genesisMerkleRoot is the hash of the first transaction in the genesis block
// for the main network.
var genesisMerkleRoot = chainhash.Hash([chainhash.HashSize]byte{ // Make go vet happy.
	0x3b, 0xa3, 0xed, 0xfd, 0x7a, 0x7b, 0x12, 0xb2,
	0x7a, 0xc7, 0x2c, 0x3e, 0x67, 0x76, 0x8f, 0x61,
	0x7f, 0xc8, 0x1b, 0xc3, 0x88, 0x8a, 0x51, 0x32,
	0x3a, 0x9f, 0xb8, 0xaa, 0x4b, 0x1e, 0x5e, 0x4a,
})

// genesisBlock defines the genesis block of the block chain which serves as the
// public transaction ledger for the main network.
var genesisBlock = wire.MsgBlock{
	Header: wire.BlockHeader{
		Version:    1,
		PrevBlock:  chainhash.Hash{},         // 0000000000000000000000000000000000000000000000000000000000000000
		MerkleRoot: genesisMerkleRoot,        // 4a5e1e4baab89f3a32518a88c31bc87f618f76673e2cc77ab2127b7afdeda33b
		Timestamp:  time.Unix(0x495fab29, 0), // 2009-01-03 18:15:05 +0000 UTC
		Bits:       0x1d00ffff,               // 486604799 [00000000ffff0000000000000000000000000000000000000000000000000000]
		Nonce:      0x7c2bac1d,               // 2083236893
	},
	Transactions: []*wire.MsgTx{&genesisCoinbaseTx},
}

// regTestGenesisHash is the hash of the first block in the block chain for the
// regression test network (genesis block).
var regTestGenesisHash = chainhash.Hash([chainhash.HashSize]byte{ // Make go vet happy.
	0x06, 0x22, 0x6e, 0x46, 0x11, 0x1a, 0x0b, 0x59,
	0xca, 0xaf, 0x12, 0x60, 0x43, 0xeb, 0x5b, 0xbf,
	0x28, 0xc3, 0x4f, 0x3a, 0x5e, 0x33, 0x2a, 0x1f,
	0xc7, 0xb2, 0xb7, 0x3c, 0xf1, 0x88, 0x91, 0x0f,
})

// regTestGenesisMerkleRoot is the hash of the first transaction in the genesis
// block for the regression test network.  It is the same as the merkle root for
// the main network.
var regTestGenesisMerkleRoot = genesisMerkleRoot

// regTestGenesisBlock defines the genesis block of the block chain which serves
// as the public transaction ledger for the regression test network.
var regTestGenesisBlock = wire.MsgBlock{
	Header: wire.BlockHeader{
		Version:    1,
		PrevBlock:  chainhash.Hash{},         // 0000000000000000000000000000000000000000000000000000000000000000
		MerkleRoot: regTestGenesisMerkleRoot, // 4a5e1e4baab89f3a32518a88c31bc87f618f76673e2cc77ab2127b7afdeda33b
		Timestamp:  time.Unix(1296688602, 0), // 2011-02-02 23:16:42 +0000 UTC
		Bits:       0x207fffff,               // 545259519 [7fffff0000000000000000000000000000000000000000000000000000000000]
		Nonce:      2,
	},
	Transactions: []*wire.MsgTx{&genesisCoinbaseTx},
}

// testNet3GenesisHash is the hash of the first block in the block chain for the
// test network (version 3).
var testNet3GenesisHash = chainhash.Hash([chainhash.HashSize]byte{ // Make go vet happy.
	0x43, 0x49, 0x7f, 0xd7, 0xf8, 0x26, 0x95, 0x71,
	0x08, 0xf4, 0xa3, 0x0f, 0xd9, 0xce, 0xc3, 0xae,
	0xba, 0x79, 0x97, 0x20, 0x84, 0xe9, 0x0e, 0xad,
	0x01, 0xea, 0x33, 0x09, 0x00, 0x00, 0x00, 0x00,
})

// testNet3GenesisMerkleRoot is the hash of the first transaction in the genesis
// block for the test network (version 3).  It is the same as the merkle root
// for the main network.
var testNet3GenesisMerkleRoot = genesisMerkleRoot

// testNet3GenesisBlock defines the genesis block of the block chain which
// serves as the public transaction ledger for the test network (version 3).
var testNet3GenesisBlock = wire.MsgBlock{
	Header: wire.BlockHeader{
		Version:    1,
		PrevBlock:  chainhash.Hash{},          // 0000000000000000000000000000000000000000000000000000000000000000
		MerkleRoot: testNet3GenesisMerkleRoot, // 4a5e1e4baab89f3a32518a88c31bc87f618f76673e2cc77ab2127b7afdeda33b
		Timestamp:  time.Unix(1296688602, 0),  // 2011-02-02 23:16:42 +0000 UTC
		Bits:       0x1d00ffff,                // 486604799 [00000000ffff0000000000000000000000000000000000000000000000000000]
		Nonce:      0x18aea41a,                // 414098458
	},
	Transactions: []*wire.MsgTx{&genesisCoinbaseTx},
}

// testNet4GenesisHash is the hash of the first block in the block chain for the
// test network (version 3).
var testNet4GenesisHash = chainhash.Hash([chainhash.HashSize]byte{ // Make go vet happy.
	0x43, 0xf0, 0x8b, 0xda, 0xb0, 0x50, 0xe3, 0x5b,
	0x56, 0x7c, 0x86, 0x4b, 0x91, 0xf4, 0x7f, 0x50,
	0xae, 0x72, 0x5a, 0xe2, 0xde, 0x53, 0xbc, 0xfb,
	0xba, 0xf2, 0x84, 0xda, 0x00, 0x00, 0x00, 0x00,
})

// genesisMerkleRoot is the hash of the first transaction in the genesis block
// for the main network.
var testnet4GenesisMerkleRoot = chainhash.Hash([chainhash.HashSize]byte{ // Make go vet happy.
	0x4e, 0x7b, 0x2b, 0x91, 0x28, 0xfe, 0x02, 0x91,
	0xdb, 0x06, 0x93, 0xaf, 0x2a, 0xe4, 0x18, 0xb7,
	0x67, 0xe6, 0x57, 0xcd, 0x40, 0x7e, 0x80, 0xcb,
	0x14, 0x34, 0x22, 0x1e, 0xae, 0xa7, 0xa0, 0x7a,
})

// testNet3GenesisMerkleRoot is the hash of the first transaction in the genesis
// block for the test network (version 3).  It is the same as the merkle root
// for the main network.
var testNet4GenesisMerkleRoot = testnet4GenesisMerkleRoot

// testNet3GenesisBlock defines the genesis block of the block chain which
// serves as the public transaction ledger for the test network (version 3).
var testNet4GenesisBlock = wire.MsgBlock{
	Header: wire.BlockHeader{
		Version:    1,
		PrevBlock:  chainhash.Hash{},          // 0000000000000000000000000000000000000000000000000000000000000000
		MerkleRoot: testNet4GenesisMerkleRoot, // 4a5e1e4baab89f3a32518a88c31bc87f618f76673e2cc77ab2127b7afdeda33b
		Timestamp:  time.Unix(1714777860, 0),  // 2024-05-03 13:24:20 +0000 UTC
		Bits:       0x1d00ffff,                // 486604799 [00000000ffff0000000000000000000000000000000000000000000000000000]
		Nonce:      0x17780CBB,                // 393743547
	},
	Transactions: []*wire.MsgTx{&genesisCoinbaseTx},
}

// simNetGenesisHash is the hash of the first block in the block chain for the
// simulation test network.
var simNetGenesisHash = chainhash.Hash([chainhash.HashSize]byte{ // Make go vet happy.
	0xf6, 0x7a, 0xd7, 0x69, 0x5d, 0x9b, 0x66, 0x2a,
	0x72, 0xff, 0x3d, 0x8e, 0xdb, 0xbb, 0x2d, 0xe0,
	0xbf, 0xa6, 0x7b, 0x13, 0x97, 0x4b, 0xb9, 0x91,
	0x0d, 0x11, 0x6d, 0x5c, 0xbd, 0x86, 0x3e, 0x68,
})

// simNetGenesisMerkleRoot is the hash of the first transaction in the genesis
// block for the simulation test network.  It is the same as the merkle root for
// the main network.
var simNetGenesisMerkleRoot = genesisMerkleRoot

// simNetGenesisBlock defines the genesis block of the block chain which serves
// as the public transaction ledger for the simulation test network.
var simNetGenesisBlock = wire.MsgBlock{
	Header: wire.BlockHeader{
		Version:    1,
		PrevBlock:  chainhash.Hash{},         // 0000000000000000000000000000000000000000000000000000000000000000
		MerkleRoot: simNetGenesisMerkleRoot,  // 4a5e1e4baab89f3a32518a88c31bc87f618f76673e2cc77ab2127b7afdeda33b
		Timestamp:  time.Unix(1401292357, 0), // 2014-05-28 15:52:37 +0000 UTC
		Bits:       0x207fffff,               // 545259519 [7fffff0000000000000000000000000000000000000000000000000000000000]
		Nonce:      2,
	},
	Transactions: []*wire.MsgTx{&genesisCoinbaseTx},
}

// sigNetGenesisHash is the hash of the first block in the block chain for the
// signet test network.
var sigNetGenesisHash = chainhash.Hash{
	0xf6, 0x1e, 0xee, 0x3b, 0x63, 0xa3, 0x80, 0xa4,
	0x77, 0xa0, 0x63, 0xaf, 0x32, 0xb2, 0xbb, 0xc9,
	0x7c, 0x9f, 0xf9, 0xf0, 0x1f, 0x2c, 0x42, 0x25,
	0xe9, 0x73, 0x98, 0x81, 0x08, 0x00, 0x00, 0x00,
}

// sigNetGenesisMerkleRoot is the hash of the first transaction in the genesis
// block for the signet test network. It is the same as the merkle root for
// the main network.
var sigNetGenesisMerkleRoot = genesisMerkleRoot

// sigNetGenesisBlock defines the genesis block of the block chain which serves
// as the public transaction ledger for the signet test network.
var sigNetGenesisBlock = wire.MsgBlock{
	Header: wire.BlockHeader{
		Version:    1,
		PrevBlock:  chainhash.Hash{},         // 0000000000000000000000000000000000000000000000000000000000000000
		MerkleRoot: sigNetGenesisMerkleRoot,  // 4a5e1e4baab89f3a32518a88c31bc87f618f76673e2cc77ab2127b7afdeda33b
		Timestamp:  time.Unix(1598918400, 0), // 2020-09-01 00:00:00 +0000 UTC
		Bits:       0x1e0377ae,               // 503543726 [00000377ae000000000000000000000000000000000000000000000000000000]
		Nonce:      52613770,
	},
	Transactions: []*wire.MsgTx{&genesisCoinbaseTx},
}

// Sats net genesis data

// satsNetGenesisCoinbaseTx is the coinbase transaction for the genesis blocks for
// the satsnet network
var satsNetGenesisCoinbaseTx = wire.MsgTx{
	Version: 1,
	TxIn: []*wire.TxIn{
		{
			PreviousOutPoint: wire.OutPoint{
				Hash:  chainhash.Hash{},
				Index: 0xffffffff,
			},
			SignatureScript: []byte{
				0x00, 0x22, 0x51, 0x20, 0x1e, 0xca, 0x94, 0xfc,
				0x17, 0x5e, 0x45, 0xd4, 0x2a, 0x90, 0x7e, 0x97,
				0xea, 0xbf, 0x3e, 0xc7, 0x6a, 0x32, 0x37, 0x65,
				0x35, 0x37, 0xcc, 0x0f, 0x11, 0xfa, 0xf4, 0xdf,
				0xd8, 0xc0, 0xe1, 0x00, 0x04, 0xef, 0x34, 0x28,
				0x67, 0x08, 0x8c, 0x74, 0x27, 0x19, 0x8d, 0xfb,
				0x3d, 0x3d,
			},
			Sequence: 0xffffffff,
		},
	},
	TxOut: []*wire.TxOut{
		{
			Value: 0,
			PkScript: []byte{
				0x51, 0x20, 0x1e, 0xca, 0x94, 0xfc, 0x17, 0x5e,
				0x45, 0xd4, 0x2a, 0x90, 0x7e, 0x97, 0xea, 0xbf,
				0x3e, 0xc7, 0x6a, 0x32, 0x37, 0x65, 0x35, 0x37,
				0xcc, 0x0f, 0x11, 0xfa, 0xf4, 0xdf, 0xd8, 0xc0,
				0xe1, 0x00,
			},
		},
	},
	LockTime: 0,
}

// satsNetGenesisMerkleRoot is the hash of the first transaction in the genesis block
// for the main network.
var satsNetGenesisMerkleRoot = chainhash.Hash([chainhash.HashSize]byte{ // Make go vet happy.
	0x05, 0x13, 0xc8, 0x04, 0x30, 0x62, 0x53, 0x96,
	0x0d, 0x55, 0x05, 0x05, 0xac, 0x06, 0xaf, 0x2b,
	0xc1, 0x3b, 0x3d, 0x12, 0x5e, 0x49, 0xb6, 0x08,
	0x11, 0x7a, 0xfb, 0x5c, 0x75, 0xaa, 0x43, 0x4a,
})

// satsTestNetGenesisHash is the hash of the first block in the block chain for the
// sattestsnet network.
var satsNetGenesisHash = chainhash.Hash{
	0x1f, 0x07, 0x25, 0x30, 0xa8, 0xbe, 0x90, 0xd2,
	0xf4, 0x39, 0x72, 0x5f, 0x58, 0xa5, 0x52, 0xa7,
	0x70, 0x93, 0xc7, 0x30, 0xb1, 0xb0, 0x68, 0xb0,
	0xae, 0x4f, 0x96, 0xa4, 0x6f, 0x5c, 0x48, 0x93,
}

// satsTestNetGenesisBlock defines the genesis block of the block chain which serves
// as the public transaction ledger for the sattestsnet network.
var satsNetGenesisBlock = wire.MsgBlock{
	Header: wire.BlockHeader{
		Version:    1,
		PrevBlock:  chainhash.Hash{},         // 0000000000000000000000000000000000000000000000000000000000000000
		MerkleRoot: satsNetGenesisMerkleRoot, //
		Timestamp:  time.Unix(1730688239, 0), // 2024-11-04 10:43:59
		Bits:       0,                        // 0
		Nonce:      1402046712,
	},
	Transactions: []*wire.MsgTx{&satsNetGenesisCoinbaseTx},
}

// genesisCoinbaseTxSatsnet is the coinbase transaction for the genesis blocks for
// the satsnet network, and satstestnet has same genesis tx.
var satsTestNetGenesisCoinbaseTx = wire.MsgTx{
	Version: 1,
	TxIn: []*wire.TxIn{
		{
			PreviousOutPoint: wire.OutPoint{
				Hash:  chainhash.Hash{},
				Index: 0xffffffff,
			},
			SignatureScript: []byte{
				0x00, 0x22, 0x51, 0x20, 0x1e, 0xca, 0x94, 0xfc,
				0x17, 0x5e, 0x45, 0xd4, 0x2a, 0x90, 0x7e, 0x97,
				0xea, 0xbf, 0x3e, 0xc7, 0x6a, 0x32, 0x37, 0x65,
				0x35, 0x37, 0xcc, 0x0f, 0x11, 0xfa, 0xf4, 0xdf,
				0xd8, 0xc0, 0xe1, 0x00, 0x04, 0xf0, 0x8e, 0x4d,
				0x67, 0x08, 0xa9, 0x92, 0x3c, 0x0c, 0xf4, 0x5c,
				0x23, 0x0c,
			},
			Sequence: 0xffffffff,
		},
	},
	TxOut: []*wire.TxOut{
		{
			Value: 0,
			PkScript: []byte{
				0x51, 0x20, 0x1e, 0xca, 0x94, 0xfc, 0x17, 0x5e,
				0x45, 0xd4, 0x2a, 0x90, 0x7e, 0x97, 0xea, 0xbf,
				0x3e, 0xc7, 0x6a, 0x32, 0x37, 0x65, 0x35, 0x37,
				0xcc, 0x0f, 0x11, 0xfa, 0xf4, 0xdf, 0xd8, 0xc0,
				0xe1, 0x00,
			},
		},
	},
	LockTime: 0,
}

// satsNetGenesisMerkleRoot is the hash of the first transaction in the genesis block
// for the main network.
var satsTestNetGenesisMerkleRoot = chainhash.Hash([chainhash.HashSize]byte{ // Make go vet happy.
	0xe8, 0xf5, 0x87, 0xe6, 0xd2, 0x00, 0xca, 0x9c,
	0x03, 0x92, 0xee, 0x74, 0x6d, 0xab, 0x24, 0xfd,
	0x7e, 0xf8, 0x01, 0x7b, 0xc2, 0xf4, 0x8d, 0x07,
	0x21, 0xe8, 0x78, 0x45, 0x41, 0x27, 0x55, 0xfa,
})

// satsTestNetGenesisHash is the hash of the first block in the block chain for the
// sattestsnet network.
var satsTestNetGenesisHash = chainhash.Hash{
	0x21, 0x3a, 0xb6, 0xeb, 0x99, 0x39, 0x1c, 0xbb,
	0xb2, 0xb9, 0x8c, 0xaf, 0x93, 0x47, 0xaa, 0xb5,
	0xf4, 0x72, 0xd6, 0x95, 0x1f, 0xe2, 0x66, 0x70,
	0x57, 0xe1, 0x43, 0x4e, 0x04, 0x68, 0x1b, 0xdf,
}

// satsTestNetGenesisBlock defines the genesis block of the block chain which serves
// as the public transaction ledger for the sattestsnet network.
var satsTestNetGenesisBlock = wire.MsgBlock{
	Header: wire.BlockHeader{
		Version:    1,
		PrevBlock:  chainhash.Hash{},             // 0000000000000000000000000000000000000000000000000000000000000000
		MerkleRoot: satsTestNetGenesisMerkleRoot, //
		Timestamp:  time.Unix(1733136112, 0),     // 2024-11-04 10:22:02
		Bits:       0,                            // 0
		Nonce:      1182242621,
	},
	Transactions: []*wire.MsgTx{&satsTestNetGenesisCoinbaseTx},
}
