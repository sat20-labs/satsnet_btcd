package httpclient

import (
	
	"encoding/json"
	"fmt"

)



type NodeClient struct {
	*RESTClient
}

func NewNodeClient(scheme, host string, net string) *IndexerClient {
	// net = "mainnet"  -- btc mainnet
	// net = "testnet"  -- btc testnet4, for indexer, it's "testnet"

	
	http := newHTTPClient()

	client := NewRESTClient(scheme, host, net, http)
	return &IndexerClient{client}
}


func (p *NodeClient) SendMsgSigReq(msg []byte, reason string) ([]byte, error) {

	req := MsgSignReq{
		Msg:		  msg,
		Reason:       reason,
	}

	buff, err := json.Marshal(&req)
	if err != nil {
		return nil, err
	}

	url := p.GetUrl("/msgsign/require")
	rsp, err := p.Http.SendPostRequest(url, buff)
	if err != nil {
		log.Errorf("SendPostRequest %v failed. %v", url, err)
		return nil, err
	}

	var result MsgSignResp
	if err := json.Unmarshal(rsp, &result); err != nil {
		log.Errorf("Unmarshal failed. %v", err)
		return nil, err
	}

	if result.Code != 0 {
		log.Errorf("SendMsgSigReq error message %s", result.Msg)
		return nil, fmt.Errorf("%s", result.Msg)
	}

	return result.Sig, nil
}
