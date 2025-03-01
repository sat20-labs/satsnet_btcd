package httpclient



type MsgSignReq struct {
	Msg          []byte `json:"msg"`
	Reason       string `json:"reason"`
}

type MsgSignResp struct {
	BaseResp
	Sig         []byte `json:"sig"`
}
