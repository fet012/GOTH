package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ─── JSON-RPC Core ────────────────────────────────────────────────────────────

type rpcRequest struct {
	Jsonrpc string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

type rpcResponse struct {
	Jsonrpc string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ─── Ethereum Types ───────────────────────────────────────────────────────────

type Transaction struct {
	Hash        string `json:"hash"`
	From        string `json:"from"`
	To          string `json:"to"`
	Value       string `json:"value"`
	BlockNumber string `json:"blockNumber"`
	Gas         string `json:"gas"`
	GasPrice    string `json:"gasPrice"`
}

type Block struct {
	Number       string        `json:"number"`
	Hash         string        `json:"hash"`
	Timestamp    string        `json:"timestamp"`
	Transactions []Transaction `json:"transactions"`
}

// ─── API Response Types ───────────────────────────────────────────────────────

type BalanceResponse struct {
	Address string `json:"address"`
	Wei     string `json:"wei"`
	ETH     string `json:"eth"`
}

type TransactionsResponse struct {
	Address      string        `json:"address"`
	Block        uint64        `json:"scanned_block"`
	Transactions []TxSummary   `json:"transactions"`
	Count        int           `json:"count"`
}

type TxSummary struct {
	Hash      string `json:"hash"`
	From      string `json:"from"`
	To        string `json:"to"`
	ETH       string `json:"eth"`
	Direction string `json:"direction"` // "sent" | "received" | "self"
}

type BlockResponse struct {
	Number    uint64 `json:"number"`
	Hash      string `json:"hash"`
	Timestamp string `json:"timestamp"`
	TxCount   int    `json:"tx_count"`
}

type WatchedResponse struct {
	Addresses []string `json:"watched_addresses"`
	Count     int      `json:"count"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// ─── Eth Client ───────────────────────────────────────────────────────────────

type EthClient struct {
	endpoint string
	client   *http.Client
}

func NewEthClient(endpoint string) *EthClient {
	return &EthClient{
		endpoint: endpoint,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *EthClient) call(method string, params []interface{}, result interface{}) error {
	req := rpcRequest{Jsonrpc: "2.0", Method: method, Params: params, ID: 1}
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	resp, err := c.client.Post(c.endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	var rpcResp rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	if rpcResp.Error != nil {
		return fmt.Errorf("rpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	return json.Unmarshal(rpcResp.Result, result)
}

func (c *EthClient) GetBalance(address string) (*big.Int, error) {
	var hex string
	if err := c.call("eth_getBalance", []interface{}{address, "latest"}, &hex); err != nil {
		return nil, err
	}
	return hexToBigInt(hex)
}

func (c *EthClient) GetBlockNumber() (uint64, error) {
	var hex string
	if err := c.call("eth_blockNumber", []interface{}{}, &hex); err != nil {
		return 0, err
	}
	n, err := hexToBigInt(hex)
	if err != nil {
		return 0, err
	}
	return n.Uint64(), nil
}

func (c *EthClient) GetBlockByNumber(blockNum uint64) (*Block, error) {
	hexNum := fmt.Sprintf("0x%x", blockNum)
	var block Block
	if err := c.call("eth_getBlockByNumber", []interface{}{hexNum, true}, &block); err != nil {
		return nil, err
	}
	return &block, nil
}

// ─── Watch List (thread-safe) ─────────────────────────────────────────────────

type WatchList struct {
	mu        sync.RWMutex
	addresses map[string]bool
}

func NewWatchList() *WatchList {
	return &WatchList{addresses: make(map[string]bool)}
}

func (w *WatchList) Add(addr string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.addresses[strings.ToLower(addr)] = true
}

func (w *WatchList) All() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	list := make([]string, 0, len(w.addresses))
	for addr := range w.addresses {
		list = append(list, addr)
	}
	return list
}

// ─── API Server ───────────────────────────────────────────────────────────────

type Server struct {
	eth       *EthClient
	watchList *WatchList
	mux       *http.ServeMux
}

func NewServer(eth *EthClient) *Server {
	s := &Server{
		eth:       eth,
		watchList: NewWatchList(),
		mux:       http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("/balance/", s.handleBalance)
	s.mux.HandleFunc("/transactions/", s.handleTransactions)
	s.mux.HandleFunc("/block/latest", s.handleLatestBlock)
	s.mux.HandleFunc("/watch/", s.handleWatch)
	s.mux.HandleFunc("/watched", s.handleWatched)
	s.mux.HandleFunc("/", s.handleIndex)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// CORS + JSON headers on every response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	s.mux.ServeHTTP(w, r)
}

// GET /balance/:address
func (s *Server) handleBalance(w http.ResponseWriter, r *http.Request) {
	address := strings.TrimPrefix(r.URL.Path, "/balance/")
	if address == "" {
		writeError(w, http.StatusBadRequest, "address is required")
		return
	}

	bal, err := s.eth.GetBalance(address)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, BalanceResponse{
		Address: address,
		Wei:     bal.String(),
		ETH:     weiToEth(bal),
	})
}

// GET /transactions/:address
func (s *Server) handleTransactions(w http.ResponseWriter, r *http.Request) {
	address := strings.TrimPrefix(r.URL.Path, "/transactions/")
	if address == "" {
		writeError(w, http.StatusBadRequest, "address is required")
		return
	}

	blockNum, err := s.eth.GetBlockNumber()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Scan last 5 blocks for transactions involving this address
	addrLower := strings.ToLower(address)
	var txSummaries []TxSummary

	start := blockNum - 4
	for b := start; b <= blockNum; b++ {
		block, err := s.eth.GetBlockByNumber(b)
		if err != nil {
			continue
		}
		for _, tx := range block.Transactions {
			isSender := strings.ToLower(tx.From) == addrLower
			isReceiver := strings.ToLower(tx.To) == addrLower
			if !isSender && !isReceiver {
				continue
			}

			direction := "received"
			if isSender && isReceiver {
				direction = "self"
			} else if isSender {
				direction = "sent"
			}

			value, _ := hexToBigInt(tx.Value)
			txSummaries = append(txSummaries, TxSummary{
				Hash:      tx.Hash,
				From:      tx.From,
				To:        tx.To,
				ETH:       weiToEth(value),
				Direction: direction,
			})
		}
	}

	if txSummaries == nil {
		txSummaries = []TxSummary{} // return [] not null
	}

	writeJSON(w, http.StatusOK, TransactionsResponse{
		Address:      address,
		Block:        blockNum,
		Transactions: txSummaries,
		Count:        len(txSummaries),
	})
}

// GET /block/latest
func (s *Server) handleLatestBlock(w http.ResponseWriter, r *http.Request) {
	blockNum, err := s.eth.GetBlockNumber()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	block, err := s.eth.GetBlockByNumber(blockNum)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	ts, _ := hexToBigInt(block.Timestamp)
	t := time.Unix(ts.Int64(), 0).UTC().Format(time.RFC3339)

	writeJSON(w, http.StatusOK, BlockResponse{
		Number:    blockNum,
		Hash:      block.Hash,
		Timestamp: t,
		TxCount:   len(block.Transactions),
	})
}

// GET /watch/:address
func (s *Server) handleWatch(w http.ResponseWriter, r *http.Request) {
	address := strings.TrimPrefix(r.URL.Path, "/watch/")
	if address == "" {
		writeError(w, http.StatusBadRequest, "address is required")
		return
	}

	s.watchList.Add(address)
	writeJSON(w, http.StatusOK, map[string]string{
		"message": "address added to watch list",
		"address": strings.ToLower(address),
	})
}

// GET /watched
func (s *Server) handleWatched(w http.ResponseWriter, r *http.Request) {
	addrs := s.watchList.All()
	writeJSON(w, http.StatusOK, WatchedResponse{
		Addresses: addrs,
		Count:     len(addrs),
	})
}

// GET /
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"name":    "GOTH API — Go + Ethereum",
		"version": "1.0.0",
		"routes": []string{
			"GET /balance/:address",
			"GET /transactions/:address",
			"GET /block/latest",
			"GET /watch/:address",
			"GET /watched",
		},
	})
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{Error: msg})
}

func hexToBigInt(hex string) (*big.Int, error) {
	hex = strings.TrimPrefix(hex, "0x")
	n := new(big.Int)
	_, ok := n.SetString(hex, 16)
	if !ok {
		return nil, fmt.Errorf("invalid hex: %s", hex)
	}
	return n, nil
}

func weiToEth(wei *big.Int) string {
	if wei == nil {
		return "0.000000"
	}
	f := new(big.Float).SetInt(wei)
	f.Quo(f, new(big.Float).SetFloat64(1e18))
	return f.Text('f', 6)
}

// ─── Main ─────────────────────────────────────────────────────────────────────

func main() {
	rpcURL := "https://eth.llamarpc.com"
	port := ":8080"

	eth := NewEthClient(rpcURL)
	server := NewServer(eth)

	fmt.Println("╔══════════════════════════════════════╗")
	fmt.Println("║     GOTH API  —  Go + Ethereum      ║")
	fmt.Println("╚══════════════════════════════════════╝")
	fmt.Printf("  Listening on http://localhost%s\n\n", port)
	fmt.Println("  Routes:")
	fmt.Println("  GET /balance/:address")
	fmt.Println("  GET /transactions/:address")
	fmt.Println("  GET /block/latest")
	fmt.Println("  GET /watch/:address")
	fmt.Println("  GET /watched")
	fmt.Println()

	if err := http.ListenAndServe(port, server); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}
