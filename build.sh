#!/usr/bin/env bash

rm -f stpd.so

cd ../transcend/plugin
go build -buildmode=plugin -o ../../satsnet_btcd/stpd.so main.go
cd ../../satsnet_btcd

rm -f satsnet_btcd
go build -o satsnet_btcd

echo build completed.