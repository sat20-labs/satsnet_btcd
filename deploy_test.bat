SET GOOS=linux
SET GOARCH=amd64
go build -o satsnet_btcd_l .

scp .\satsnet_btcd_l root@192.168.10.104:/data/satsnet/satoshinet/satsnet_btcd_l