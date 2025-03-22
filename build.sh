#!/usr/bin/env bash

rm -f stpd.so

cd ../transcend/plugin
go build -buildmode=plugin -o ../../satoshinet/stpd.so main.go
cd ../../satoshinet

rm -f satoshinet
go build -o satoshinet

echo build completed.