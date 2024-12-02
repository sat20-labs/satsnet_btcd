// Copyright (c) 2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package validatormanager

import (
	"net"

	"github.com/sat20-labs/satsnet_btcd/chaincfg"
)

const (
// These constants are used by the DNS seed code
// DefaultPort = 14829
)

// SeedFromDNS uses DNS seeding to populate the address manager with peers.
func (vm *ValidatorManager) getSeed(chainParams *chaincfg.Params) ([]net.Addr, error) {

	addrs := make([]net.Addr, 0, len(chainParams.DNSSeeds))
	for _, dnsseed := range chainParams.DNSSeeds {
		addr, err := vm.getAddr(dnsseed.Host)
		if err != nil {
			log.Infof("DNS discovery failed on seed %s: %v", dnsseed.Host, err)
			continue
		}
		addrs = append(addrs, addr)
	}

	return addrs, nil
}

func (vm *ValidatorManager) getLocalAddr() ([]net.Addr, error) {
	localAddrsList := make([]net.Addr, 0)
	addrs, _ := net.InterfaceAddrs()
	port := vm.GetValidatorPort()

	if addrs != nil && len(addrs) > 0 {
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					newAddr := &net.TCPAddr{
						IP:   ipnet.IP,
						Port: port,
					}
					localAddrsList = append(localAddrsList, newAddr)
				}
			}
		}
		return localAddrsList, nil
	}

	addr := &net.TCPAddr{
		IP: net.IP{
			127, 0, 0, 1},
		Port: port,
	}
	localAddrsList = append(localAddrsList, addr)
	return addrs, nil
}

func (vm *ValidatorManager) getAddr(Host string) (net.Addr, error) {
	ips, err := vm.Cfg.Lookup(Host)
	if err != nil {
		return nil, err
	}
	port := vm.GetValidatorPort()
	addr := &net.TCPAddr{
		IP:   ips[0],
		Port: port,
	}
	return addr, nil
}
