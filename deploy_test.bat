SET GOOS=linux
SET GOARCH=amd64
go build -o satsnet_btcd_l .

scp .\satsnet_btcd_l root@39.108.147.241:/satsnet_btcd/satsnet_btcd_l