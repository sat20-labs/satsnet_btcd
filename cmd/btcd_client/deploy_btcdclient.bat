SET GOOS=linux
SET GOARCH=amd64
go build -o btcd_clinet_l .

scp .\btcd_clinet_l root@192.168.10.104:/data/satsnet/satoshinet/btcd_clinet_l