package main

import (
		"bytes"
			"encoding/json"
				"fmt"
					"log"
						"math/big"
							"net/http"
								"os"
									"strings"
										"time"
										)

										// ─── JSON-RPC Types ────────────────────────────────────────────────────────────

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

																							// ─── Ethereum Types ────────────────────────────────────────────────────────────

																							type Transaction struct {
																									Hash             string `json:"hash"`
																										From             string `json:"from"`
																											To               string `json:"to"`
																												Value            string `json:"value"` // hex wei
																													BlockNumber      string `json:"blockNumber"`
																														Gas              string `json:"gas"`
																															GasPrice         string `json:"gasPrice"`
																																TransactionIndex string `json:"transactionIndex"`
																																}

																																type Block struct {
																																		Number       string        `json:"number"`
																																			Hash         string        `json:"hash"`
																																				Timestamp    string        `json:"timestamp"`
																																					Transactions []Transaction `json:"transactions"`
																																					}

																																					// ─── RPC Client ───────────────────────────────────────────────────────────────

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
																																																		req := rpcRequest{
																																																					Jsonrpc: "2.0",
																																																							Method:  method,
																																																									Params:  params,
																																																											ID:      1,
																																																												}

																																																													body, err := json.Marshal(req)
																																																														if err != nil {
																																																																	return fmt.Errorf("marshal request: %w", err)
																																																																		}

																																																																			resp, err := c.client.Post(c.endpoint, "application/json", bytes.NewReader(body))
																																																																				if err != nil {
																																																																							return fmt.Errorf("http post: %w", err)
																																																																								}
																																																																									defer resp.Body.Close()

																																																																										var rpcResp rpcResponse
																																																																											if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
																																																																														return fmt.Errorf("decode response: %w", err)
																																																																															}

																																																																																if rpcResp.Error != nil {
																																																																																			return fmt.Errorf("rpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
																																																																																				}

																																																																																					return json.Unmarshal(rpcResp.Result, result)
																																																																																					}

																																																																																					// GetBalance returns the ETH balance of an address in Wei (as *big.Int)
																																																																																					func (c *EthClient) GetBalance(address string) (*big.Int, error) {
																																																																																							var hexBalance string
																																																																																								if err := c.call("eth_getBalance", []interface{}{address, "latest"}, &hexBalance); err != nil {
																																																																																											return nil, err
																																																																																												}
																																																																																													return hexToBigInt(hexBalance)
																																																																																													}

																																																																																													// GetBlockNumber returns the latest block number
																																																																																													func (c *EthClient) GetBlockNumber() (uint64, error) {
																																																																																															var hexBlock string
																																																																																																if err := c.call("eth_blockNumber", []interface{}{}, &hexBlock); err != nil {
																																																																																																			return 0, err
																																																																																																				}
																																																																																																					n, err := hexToBigInt(hexBlock)
																																																																																																						if err != nil {
																																																																																																									return 0, err
																																																																																																										}
																																																																																																											return n.Uint64(), nil
																																																																																																											}

																																																																																																											// GetBlockByNumber returns a full block with transactions
																																																																																																											func (c *EthClient) GetBlockByNumber(blockNum uint64) (*Block, error) {
																																																																																																													hexNum := fmt.Sprintf("0x%x", blockNum)
																																																																																																														var block Block
																																																																																																															if err := c.call("eth_getBlockByNumber", []interface{}{hexNum, true}, &block); err != nil {
																																																																																																																		return nil, err
																																																																																																																			}
																																																																																																																				return &block, nil
																																																																																																																				}

																																																																																																																				// GetTransactionCount returns the nonce (tx count) of an address
																																																																																																																				func (c *EthClient) GetTransactionCount(address string) (uint64, error) {
																																																																																																																						var hexCount string
																																																																																																																							if err := c.call("eth_getTransactionCount", []interface{}{address, "latest"}, &hexCount); err != nil {
																																																																																																																										return 0, err
																																																																																																																											}
																																																																																																																												n, err := hexToBigInt(hexCount)
																																																																																																																													if err != nil {
																																																																																																																																return 0, err
																																																																																																																																	}
																																																																																																																																		return n.Uint64(), nil
																																																																																																																																		}

																																																																																																																																		// ─── Wallet Monitor ───────────────────────────────────────────────────────────

																																																																																																																																		type WalletMonitor struct {
																																																																																																																																				client      *EthClient
																																																																																																																																					addresses   []string
																																																																																																																																						lastBalance map[string]*big.Int
																																																																																																																																							lastBlock   uint64
																																																																																																																																								pollSecs    int
																																																																																																																																								}

																																																																																																																																								func NewWalletMonitor(client *EthClient, addresses []string, pollSecs int) *WalletMonitor {
																																																																																																																																										return &WalletMonitor{
																																																																																																																																													client:      client,
																																																																																																																																															addresses:   addresses,
																																																																																																																																																	lastBalance: make(map[string]*big.Int),
																																																																																																																																																			pollSecs:    pollSecs,
																																																																																																																																																				}
																																																																																																																																																				}

																																																																																																																																																				// Start begins polling. Blocks until ctx is cancelled (or forever).
																																																																																																																																																				func (m *WalletMonitor) Start() {
																																																																																																																																																						fmt.Println(banner())
																																																																																																																																																							fmt.Printf("Monitoring %d address(es) every %ds...\n\n", len(m.addresses), m.pollSecs)

																																																																																																																																																								// Seed initial balances
																																																																																																																																																									for _, addr := range m.addresses {
																																																																																																																																																												bal, err := m.client.GetBalance(addr)
																																																																																																																																																														if err != nil {
																																																																																																																																																																		log.Printf("[WARN] Could not get initial balance for %s: %v", addr, err)
																																																																																																																																																																					m.lastBalance[addr] = big.NewInt(0)
																																																																																																																																																																								continue
																																																																																																																																																																										}
																																																																																																																																																																												m.lastBalance[addr] = bal
																																																																																																																																																																														fmt.Printf("📍 %s\n   Balance: %s ETH\n\n", addr, weiToEth(bal))
																																																																																																																																																																															}

																																																																																																																																																																																// Seed initial block
																																																																																																																																																																																	blockNum, err := m.client.GetBlockNumber()
																																																																																																																																																																																		if err != nil {
																																																																																																																																																																																					log.Fatalf("Could not get latest block: %v", err)
																																																																																																																																																																																						}
																																																																																																																																																																																							m.lastBlock = blockNum
																																																																																																																																																																																								fmt.Printf("🔗 Starting from block #%d\n%s\n", blockNum, strings.Repeat("─", 60))

																																																																																																																																																																																									ticker := time.NewTicker(time.Duration(m.pollSecs) * time.Second)
																																																																																																																																																																																										defer ticker.Stop()

																																																																																																																																																																																											for range ticker.C {
																																																																																																																																																																																														m.poll()
																																																																																																																																																																																															}
																																																																																																																																																																																															}

																																																																																																																																																																																															func (m *WalletMonitor) poll() {
																																																																																																																																																																																																	latestBlock, err := m.client.GetBlockNumber()
																																																																																																																																																																																																		if err != nil {
																																																																																																																																																																																																					log.Printf("[WARN] eth_blockNumber failed: %v", err)
																																																																																																																																																																																																							return
																																																																																																																																																																																																								}

																																																																																																																																																																																																									// Scan new blocks for watched address activity
																																																																																																																																																																																																										for blockNum := m.lastBlock + 1; blockNum <= latestBlock; blockNum++ {
																																																																																																																																																																																																													block, err := m.client.GetBlockByNumber(blockNum)
																																																																																																																																																																																																															if err != nil {
																																																																																																																																																																																																																			log.Printf("[WARN] Could not fetch block %d: %v", blockNum, err)
																																																																																																																																																																																																																						continue
																																																																																																																																																																																																																								}

																																																																																																																																																																																																																										ts := hexToTimestamp(block.Timestamp)
																																																																																																																																																																																																																												for _, tx := range block.Transactions {
																																																																																																																																																																																																																																for _, addr := range m.addresses {
																																																																																																																																																																																																																																					addrLower := strings.ToLower(addr)
																																																																																																																																																																																																																																									isSender := strings.ToLower(tx.From) == addrLower
																																																																																																																																																																																																																																													isReceiver := strings.ToLower(tx.To) == addrLower

																																																																																																																																																																																																																																																	if isSender || isReceiver {
																																																																																																																																																																																																																																																							value, _ := hexToBigInt(tx.Value)
																																																																																																																																																																																																																																																												direction := "⬆️  SENT"
																																																																																																																																																																																																																																																																	counterparty := tx.To
																																																																																																																																																																																																																																																																						if isReceiver && !isSender {
																																																																																																																																																																																																																																																																													direction = "⬇️  RECEIVED"
																																																																																																																																																																																																																																																																																			counterparty = tx.From
																																																																																																																																																																																																																																																																																								}

																																																																																																																																																																																																																																																																																													fmt.Printf("\n%s [Block #%d | %s]\n", direction, blockNum, ts.Format("2006-01-02 15:04:05"))
																																																																																																																																																																																																																																																																																																		fmt.Printf("   Address : %s\n", addr)
																																																																																																																																																																																																																																																																																																							fmt.Printf("   Hash    : %s\n", tx.Hash)
																																																																																																																																																																																																																																																																																																												fmt.Printf("   Value   : %s ETH\n", weiToEth(value))
																																																																																																																																																																																																																																																																																																																	fmt.Printf("   With    : %s\n", counterparty)
																																																																																																																																																																																																																																																																																																																						fmt.Println(strings.Repeat("─", 60))
																																																																																																																																																																																																																																																																																																																										}
																																																																																																																																																																																																																																																																																																																													}
																																																																																																																																																																																																																																																																																																																															}
																																																																																																																																																																																																																																																																																																																																}
																																																																																																																																																																																																																																																																																																																																	m.lastBlock = latestBlock

																																																																																																																																																																																																																																																																																																																																		// Check for balance changes
																																																																																																																																																																																																																																																																																																																																			for _, addr := range m.addresses {
																																																																																																																																																																																																																																																																																																																																						newBal, err := m.client.GetBalance(addr)
																																																																																																																																																																																																																																																																																																																																								if err != nil {
																																																																																																																																																																																																																																																																																																																																												continue
																																																																																																																																																																																																																																																																																																																																														}
																																																																																																																																																																																																																																																																																																																																																oldBal := m.lastBalance[addr]
																																																																																																																																																																																																																																																																																																																																																		if oldBal != nil && newBal.Cmp(oldBal) != 0 {
																																																																																																																																																																																																																																																																																																																																																						diff := new(big.Int).Sub(newBal, oldBal)
																																																																																																																																																																																																																																																																																																																																																									sign := "+"
																																																																																																																																																																																																																																																																																																																																																												if diff.Sign() < 0 {
																																																																																																																																																																																																																																																																																																																																																																	sign = ""
																																																																																																																																																																																																																																																																																																																																																																				}
																																																																																																																																																																																																																																																																																																																																																																							fmt.Printf("\n💰 BALANCE CHANGE — %s\n", addr)
																																																																																																																																																																																																																																																																																																																																																																										fmt.Printf("   Old: %s ETH\n", weiToEth(oldBal))
																																																																																																																																																																																																																																																																																																																																																																													fmt.Printf("   New: %s ETH\n", weiToEth(newBal))
																																																																																																																																																																																																																																																																																																																																																																																fmt.Printf("   Δ  : %s%s ETH\n", sign, weiToEth(diff))
																																																																																																																																																																																																																																																																																																																																																																																			fmt.Println(strings.Repeat("─", 60))
																																																																																																																																																																																																																																																																																																																																																																																						m.lastBalance[addr] = newBal
																																																																																																																																																																																																																																																																																																																																																																																								}
																																																																																																																																																																																																																																																																																																																																																																																									}
																																																																																																																																																																																																																																																																																																																																																																																									}

																																																																																																																																																																																																																																																																																																																																																																																									// ─── Utility Functions ────────────────────────────────────────────────────────

																																																																																																																																																																																																																																																																																																																																																																																									// hexToBigInt converts a "0x..." hex string to *big.Int
																																																																																																																																																																																																																																																																																																																																																																																									func hexToBigInt(hex string) (*big.Int, error) {
																																																																																																																																																																																																																																																																																																																																																																																											hex = strings.TrimPrefix(hex, "0x")
																																																																																																																																																																																																																																																																																																																																																																																												n := new(big.Int)
																																																																																																																																																																																																																																																																																																																																																																																													_, ok := n.SetString(hex, 16)
																																																																																																																																																																																																																																																																																																																																																																																														if !ok {
																																																																																																																																																																																																																																																																																																																																																																																																	return nil, fmt.Errorf("invalid hex: %s", hex)
																																																																																																																																																																																																																																																																																																																																																																																																		}
																																																																																																																																																																																																																																																																																																																																																																																																			return n, nil
																																																																																																																																																																																																																																																																																																																																																																																																			}

																																																																																																																																																																																																																																																																																																																																																																																																			// weiToEth converts Wei (*big.Int) to a human-readable ETH string
																																																																																																																																																																																																																																																																																																																																																																																																			func weiToEth(wei *big.Int) string {
																																																																																																																																																																																																																																																																																																																																																																																																					if wei == nil {
																																																																																																																																																																																																																																																																																																																																																																																																								return "0.000000"
																																																																																																																																																																																																																																																																																																																																																																																																									}
																																																																																																																																																																																																																																																																																																																																																																																																										// ETH = Wei / 1e18
																																																																																																																																																																																																																																																																																																																																																																																																											eth := new(big.Float).SetInt(wei)
																																																																																																																																																																																																																																																																																																																																																																																																												divisor := new(big.Float).SetFloat64(1e18)
																																																																																																																																																																																																																																																																																																																																																																																																													eth.Quo(eth, divisor)
																																																																																																																																																																																																																																																																																																																																																																																																														return eth.Text('f', 6)
																																																																																																																																																																																																																																																																																																																																																																																																														}

																																																																																																																																																																																																																																																																																																																																																																																																														// hexToTimestamp converts a hex Unix timestamp to time.Time
																																																																																																																																																																																																																																																																																																																																																																																																														func hexToTimestamp(hexTs string) time.Time {
																																																																																																																																																																																																																																																																																																																																																																																																																n, err := hexToBigInt(hexTs)
																																																																																																																																																																																																																																																																																																																																																																																																																	if err != nil {
																																																																																																																																																																																																																																																																																																																																																																																																																				return time.Time{}
																																																																																																																																																																																																																																																																																																																																																																																																																					}
																																																																																																																																																																																																																																																																																																																																																																																																																						return time.Unix(n.Int64(), 0)
																																																																																																																																																																																																																																																																																																																																																																																																																						}

																																																																																																																																																																																																																																																																																																																																																																																																																						func banner() string {
																																																																																																																																																																																																																																																																																																																																																																																																																								return `
																																																																																																																																																																																																																																																																																																																																																																																																																								╔═══════════════════════════════════════════════╗
																																																																																																																																																																																																																																																																																																																																																																																																																								║       🔍  Go ETH Wallet Monitor  v1.0        ║
																																																																																																																																																																																																																																																																																																																																																																																																																								║     Real-time Ethereum address watcher       ║
																																																																																																																																																																																																																																																																																																																																																																																																																								╚═══════════════════════════════════════════════╝`
																																																																																																																																																																																																																																																																																																																																																																																																																								}

																																																																																																																																																																																																																																																																																																																																																																																																																								// ─── Entry Point ──────────────────────────────────────────────────────────────

																																																																																																																																																																																																																																																																																																																																																																																																																								func main() {
																																																																																																																																																																																																																																																																																																																																																																																																																										// Configuration — swap these out or load from env
																																																																																																																																																																																																																																																																																																																																																																																																																											rpcEndpoint := getEnv("ETH_RPC_URL", "https://eth.llamarpc.com") // free public RPC

																																																																																																																																																																																																																																																																																																																																																																																																																												// Addresses to watch (Vitalik's public address used as demo)
																																																																																																																																																																																																																																																																																																																																																																																																																													watchAddresses := []string{
																																																																																																																																																																																																																																																																																																																																																																																																																																"0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045", // vitalik.eth
																																																																																																																																																																																																																																																																																																																																																																																																																																	}

																																																																																																																																																																																																																																																																																																																																																																																																																																		// Override from env if set: ETH_WATCH=0xABC...,0xDEF...
																																																																																																																																																																																																																																																																																																																																																																																																																																			if envAddrs := os.Getenv("ETH_WATCH"); envAddrs != "" {
																																																																																																																																																																																																																																																																																																																																																																																																																																						watchAddresses = strings.Split(envAddrs, ",")
																																																																																																																																																																																																																																																																																																																																																																																																																																							}

																																																																																																																																																																																																																																																																																																																																																																																																																																								pollSeconds := 12 // ~1 ETH block time

																																																																																																																																																																																																																																																																																																																																																																																																																																									client := NewEthClient(rpcEndpoint)
																																																																																																																																																																																																																																																																																																																																																																																																																																										monitor := NewWalletMonitor(client, watchAddresses, pollSeconds)
																																																																																																																																																																																																																																																																																																																																																																																																																																											monitor.Start()
																																																																																																																																																																																																																																																																																																																																																																																																																																											}

																																																																																																																																																																																																																																																																																																																																																																																																																																											func getEnv(key, fallback string) string {
																																																																																																																																																																																																																																																																																																																																																																																																																																													if v := os.Getenv(key); v != "" {
																																																																																																																																																																																																																																																																																																																																																																																																																																																return v
																																																																																																																																																																																																																																																																																																																																																																																																																																																	}
																																																																																																																																																																																																																																																																																																																																																																																																																																																		return fallback
																																																																																																																																																																																																																																																																																																																																																																																																																																																		}
																																																																																																																																																																																																																																																																																																																																																																																																																																																		
																																																																																																																																																																																																																																																																																																																																																																																																																																													}
																																																																																																																																																																																																																																																																																																																																																																																																																																											}
																																																																																																																																																																																																																																																																																																																																																																																																																																			}
																																																																																																																																																																																																																																																																																																																																																																																																																													}
																																																																																																																																																																																																																																																																																																																																																																																																																								}
																																																																																																																																																																																																																																																																																																																																																																																																																						}
																																																																																																																																																																																																																																																																																																																																																																																																																	}
																																																																																																																																																																																																																																																																																																																																																																																																														}
																																																																																																																																																																																																																																																																																																																																																																																																					}
																																																																																																																																																																																																																																																																																																																																																																																																			}
																																																																																																																																																																																																																																																																																																																																																																																														}
																																																																																																																																																																																																																																																																																																																																																																																									}
																																																																																																																																																																																																																																																																																																																																																												}
																																																																																																																																																																																																																																																																																																																																																		}
																																																																																																																																																																																																																																																																																																																																								}
																																																																																																																																																																																																																																																																																																																																			}
																																																																																																																																																																																																																																																																						}
																																																																																																																																																																																																																																																	}
																																																																																																																																																																																																																																}
																																																																																																																																																																																																																												}
																																																																																																																																																																																																															}
																																																																																																																																																																																																										}
																																																																																																																																																																																																		}
																																																																																																																																																																																															}
																																																																																																																																																																																											}
																																																																																																																																																																																		}
																																																																																																																																																														}
																																																																																																																																																									}
																																																																																																																																																				}
																																																																																																																																										}
																																																																																																																																								}
																																																																																																																																		}
																																																																																																																													}
																																																																																																																							}
																																																																																																																				}
																																																																																																															}
																																																																																																											}
																																																																																																						}
																																																																																																}
																																																																																													}
																																																																																								}
																																																																																					}
																																																																																}
																																																																											}
																																																																				}
																																																														}
																																																		}
																																																}
																																										}
																																								}
																																					}
																																}
																							}
																				}
															}
										}
)

