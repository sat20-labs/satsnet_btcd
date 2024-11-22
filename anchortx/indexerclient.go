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
	host  string
	proxy string
}

func (p *IndexerClient) getUrl(path string) string {
	return p.proxy + path
}

func NewIndexerClient(host string, net string) *IndexerClient {
	// net = "mainnet"  -- btc mainnet
	// net = "testnet"  -- btc testnet4, for indexer, it's "testnet"

	return &IndexerClient{
		host:  host,
		proxy: net,
	}
}

type IndexerTxResp struct {
	Code int    `json:"code" example:"0"`
	Msg  string `json:"msg" example:"ok"`
	Data string `json:"data"`
}

// btcutil.Tx
func (p *IndexerClient) GetRawTx(tx string) (string, error) {
	path := p.getUrl("/btc/rawtx/" + tx)
	rsp, err := p.SendGetRequest(path)
	if err != nil {
		//Log.Errorf("SendGetRequest %v failed. %v", url, err)
		return "", err
	}

	fmt.Printf("%v response: %s", path, string(rsp))

	// Unmarshal the response.
	var result IndexerTxResp
	if err := json.Unmarshal(rsp, &result); err != nil {
		err := fmt.Errorf("%s response data format failed: %s", path, string(rsp))
		return "", err
	}

	if result.Code != 0 {
		err := fmt.Errorf("%s response failed: %s", path, result.Msg)
		return "", err
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

func (p *IndexerClient) SendGetRequest(path string) ([]byte, error) {

	url := url.URL{
		Scheme: "http",
		Host:   p.host,
		Path:   path,
	}

	netTransport := &http.Transport{
		Dial: (&net.Dialer{
			Timeout: TIME_OUT_DURATION * time.Second,
		}).Dial,
	}
	httpClient := &http.Client{
		Timeout:   60 * time.Second,
		Transport: netTransport,
	}

	httpResponse, err := httpClient.Get(url.String())
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
