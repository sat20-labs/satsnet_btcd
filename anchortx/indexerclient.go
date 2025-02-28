package anchortx

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/sat20-labs/satsnet_btcd/btcutil"
	"github.com/sat20-labs/satsnet_btcd/wire"
)

const (
	TIME_OUT_DURATION = 60
)

type IndexerClient struct {
	scheme string
	host  string
	proxy string
	http  *http.Client
}

type Userinfo struct {
	Username    string
	Password    string
}

type URL struct {
	Scheme      string
	User        *Userinfo // username and password information
	Host        string    // host or host:port (see Hostname and Port methods)
	Path        string    // path (relative paths may omit leading slash)
}

func (p *URL) String() string {
	return p.Scheme + "://" + p.Host + p.Path
}

func NewIndexerClient(scheme, host string, net string) *IndexerClient {
	// net = "mainnet"  -- btc mainnet
	// net = "testnet"  -- btc testnet4, for indexer, it's "testnet"

	http := newHTTPClient()

	if scheme == "" {
		scheme = "http"
	}

	return &IndexerClient{
		scheme: scheme,
		host:   host,
		proxy:  net,
		http:   http,
	}
}

func newHTTPClient() *http.Client {
	var httpClient *http.Client

	netTransport := &http.Transport{
		Dial: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second, // keepalive超时时间
		}).Dial,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		MaxConnsPerHost:       10,
		MaxIdleConnsPerHost:   10,
	}
	httpClient = &http.Client{
		Timeout:   60 * time.Second,
		Transport: netTransport,
	}

	return httpClient
}

func (p *IndexerClient) GetUrl(path string) *URL {
	return &URL{
		Scheme: p.scheme,
		Host:   p.host,
		Path:   p.proxy + path,
	}
}


type BaseResp struct {
	Code int    `json:"code" example:"0"`
	Msg  string `json:"msg" example:"ok"`
}

type TxResp struct {
	BaseResp
	Data any `json:"data"`
}

type TxOut struct {
	Value    int64  `json:"Value"`
	PkScript []byte `json:"PkScript"`
}

type OffsetRange struct {
	Start int64
	End   int64 // 不包括End
}

type AssetOffsets []*OffsetRange

type UtxoAssetInfo struct {
	Asset   wire.AssetInfo     `json:"asset"`
	Offsets AssetOffsets `json:"offsets"`
}

type TxOutputInfo struct {
	UtxoId    uint64       `json:"utxoid"`
	OutPoint  string       `json:"outpoint"`
	OutValue  TxOut   `json:"outvalue"`
	AssetInfo []*UtxoAssetInfo `json:"assets"`
}

type TxOutputResp struct {
	BaseResp
	Data *TxOutputInfo `json:"data"`
}

// btcutil.Tx
func (p *IndexerClient) GetRawTx(tx string) (string, error) {
	path := p.GetUrl("/btc/rawtx/" + tx)
	rsp, err := p.SendGetRequest(path)
	if err != nil {
		//Log.Errorf("SendGetRequest %v failed. %v", url, err)
		return "", err
	}

	fmt.Printf("%v response: %s", path, string(rsp))

	// Unmarshal the response.
	var result TxResp
	if err := json.Unmarshal(rsp, &result); err != nil {
		err := fmt.Errorf("%s response data format failed: %s", path, string(rsp))
		return "", err
	}

	if result.Code != 0 {
		err := fmt.Errorf("%s response failed: %s", path, result.Msg)
		return "", err
	}

	return result.Data.(string), nil
}

func (p *IndexerClient) GetTxUtxoAssets(utxo string) (*TxOutputInfo, error) {
	path := p.GetUrl("/v2/utxo/info/" + utxo)
	rsp, err := p.SendGetRequest(path)
	if err != nil {
		//Log.Errorf("SendGetRequest %v failed. %v", url, err)
		return nil, err
	}

	fmt.Printf("%v response: %s\n", path, string(rsp))

	// Unmarshal the response.
	var result TxOutputResp
	if err := json.Unmarshal(rsp, &result); err != nil {
		err := fmt.Errorf("%s response data format failed: %s", path, string(rsp))
		return nil, err
	}

	if result.Code != 0 {
		err := fmt.Errorf("%s response failed: %s", path, result.Msg)
		return nil, err
	}

	return result.Data, nil
}

// DecodeStringToTx takes a string and decodes it to a btcutil.Tx
func DecodeStringToTx(encodedStr string) (*btcutil.Tx, error) {
	// Convert the hex string back to bytes
	txBytes, err := hex.DecodeString(encodedStr)
	if err != nil {
		return nil, err
	}

	// Create a buffer from the byte slice
	buf := bytes.NewBuffer(txBytes)

	// Create an empty MsgTx to deserialize into
	msgTx := wire.MsgTx{}

	// Deserialize the bytes into the MsgTx
	err = msgTx.Deserialize(buf)
	if err != nil {
		return nil, err
	}

	// Wrap the deserialized MsgTx into a btcutil.Tx and return
	return btcutil.NewTx(&msgTx), nil
}

func (p *IndexerClient) SendGetRequest(u *URL) ([]byte, error) {

	url := url.URL{
		Scheme: u.Scheme,
		Host:   u.Host,
		Path:   u.Path,
	}

	httpResponse, err := p.http.Get(url.String())
	if err != nil {
		return nil, err
	}

	// Read the raw bytes and close the response.
	respBytes, err := io.ReadAll(httpResponse.Body)
	httpResponse.Body.Close()
	if err != nil {
		err = fmt.Errorf("error reading json reply: %v", err)
		return nil, err
	}

	// Handle unsuccessful HTTP responses
	if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 300 {
		// Generate a standard error to return if the server body is
		// empty.  This should not happen very often, but it's better
		// than showing nothing in case the target server has a poor
		// implementation.
		if len(respBytes) == 0 {
			return nil, fmt.Errorf("%d %s", httpResponse.StatusCode,
				http.StatusText(httpResponse.StatusCode))
		}
		return nil, fmt.Errorf("%s", respBytes)
	}

	// Unmarshal the response.
	// var resp btcjson.Response
	// if err := json.Unmarshal(respBytes, &resp); err != nil {
	// 	return nil, err
	// }

	// if resp.Error != nil {
	// 	return nil, resp.Error
	// }
	// return resp.Result, nil
	return respBytes, nil
}

// sendPostRequest sends the marshalled JSON command using HTTP-POST mode
// to the server described in the passed config struct.  It also attempts to
// unmarshal the response as a JSON response and returns either the result
// field or the error field depending on whether or not there is an error.
func (p *IndexerClient) SendPostRequest(u *URL, marshalledJSON []byte) ([]byte, error) {
	url := url.URL{
		Scheme: u.Scheme,
		Host:   u.Host,
		Path:   u.Path,
	}

	bodyReader := bytes.NewReader(marshalledJSON)
	httpRequest, err := http.NewRequest("POST", url.String(), bodyReader)
	if err != nil {
		return nil, err
	}
	httpRequest.Close = true
	httpRequest.Header.Set("Content-Type", "application/json")

	httpResponse, err := p.http.Do(httpRequest)
	if err != nil {
		return nil, err
	}

	// Read the raw bytes and close the response.
	respBytes, err := io.ReadAll(httpResponse.Body)
	httpResponse.Body.Close()
	if err != nil {
		err = fmt.Errorf("error reading json reply: %v", err)
		return nil, err
	}

	// Handle unsuccessful HTTP responses
	if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 300 {
		// Generate a standard error to return if the server body is
		// empty.  This should not happen very often, but it's better
		// than showing nothing in case the target server has a poor
		// implementation.
		if len(respBytes) == 0 {
			return nil, fmt.Errorf("%d %s", httpResponse.StatusCode,
				http.StatusText(httpResponse.StatusCode))
		}
		return nil, fmt.Errorf("%s", respBytes)
	}

	// Unmarshal the response.
	// var resp btcjson.Response
	// if err := json.Unmarshal(respBytes, &resp); err != nil {
	// 	return nil, err
	// }

	// if resp.Error != nil {
	// 	return nil, resp.Error
	// }
	// return resp.Result, nil
	return respBytes, nil
}
