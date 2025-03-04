#!/usr/bin/env bash

./build.sh

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

