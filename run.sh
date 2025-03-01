#!/usr/bin/env bash

rm -f stpd.so

cd ../transcend/plugin
go build -buildmode=plugin -o ../../satsnet_btcd/stpd.so main.go
cd ../../satsnet_btcd

rm -f satsnet_btcd
go build -o satsnet_btcd

if [ $# -eq 0 ]; then
  nohup ./satsnet_btcd --homedir ./data --txindex > ./nohup.log 2>&1 &
  disown
else
  if [ "$1" = "off" ]; then
    ./satsnet_btcd --homedir ./data --txindex
  else
    echo "unknown parameter"
  fi
fi

