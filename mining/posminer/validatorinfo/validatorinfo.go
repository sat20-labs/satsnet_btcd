package validatorinfo

import (
	"net"
	"time"

	"github.com/sat20-labs/satsnet_btcd/btcec"
)

type ValidatorInfo struct {
	Host            string
	ValidatorId     uint64
	PublicKey       [btcec.PubKeyBytesLenCompressed]byte
	CreateTime      time.Time
	ActivitionCount int32
	GeneratorCount  int32
	DiscountCount   int32
	FaultCount      int32
	ValidatorScore  int32
}

type ValidatorInfoMask uint64

const (
	MaskValidatorId     ValidatorInfoMask = 1 << 0
	MaskPublicKey       ValidatorInfoMask = 1 << 1
	MaskActivitionCount ValidatorInfoMask = 1 << 2
	MaskGeneratorCount  ValidatorInfoMask = 1 << 3
	MaskDiscountCount   ValidatorInfoMask = 1 << 4
	MaskFaultCount      ValidatorInfoMask = 1 << 5
	MaskCreateTime      ValidatorInfoMask = 1 << 6
	MaskHost            ValidatorInfoMask = 1 << 7

	MaskAll ValidatorInfoMask = MaskValidatorId | MaskPublicKey | MaskActivitionCount | MaskGeneratorCount | MaskDiscountCount | MaskFaultCount | MaskCreateTime
)

func GetAddrStringHost(addr string) string {
	host, _, _ := net.SplitHostPort(addr)
	return host
}

func GetAddrHost(addr net.Addr) net.IP {
	switch addr.(type) {
	case *net.TCPAddr:
		addrHost := addr.(*net.TCPAddr).IP
		return addrHost
	}

	return nil
}
