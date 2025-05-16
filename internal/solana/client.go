package solana

import (
	"context"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// Client 包装Solana客户端功能
type Client struct {
	rpcClient *rpc.Client
	ctx       context.Context
}

// New 创建新的Solana客户端
func New(endpoint string, ctx context.Context) *Client {
	client := rpc.New(endpoint)
	return &Client{
		rpcClient: client,
		ctx:       ctx,
	}
}

// Close 关闭客户端连接
func (c *Client) Close() error {
	return c.rpcClient.Close()
}

// GetLatestBlockhash 获取最新的区块哈希
func (c *Client) GetLatestBlockhash() (*rpc.GetLatestBlockhashResult, error) {
	return c.rpcClient.GetLatestBlockhash(c.ctx, rpc.CommitmentConfirmed)
}

// GetTokenAccountBalance 获取代币账户余额
func (c *Client) GetTokenAccountBalance(account solana.PublicKey) (*rpc.GetTokenAccountBalanceResult, error) {
	return c.rpcClient.GetTokenAccountBalance(c.ctx, account, rpc.CommitmentConfirmed)
}

// SimulateTransaction 模拟交易
func (c *Client) SimulateTransaction(tx *solana.Transaction) (*rpc.SimulateTransactionResponse, error) {
	return c.rpcClient.SimulateTransaction(c.ctx, tx)
}

// SendTransaction 发送交易
func (c *Client) SendTransaction(tx *solana.Transaction) (solana.Signature, error) {
	return c.rpcClient.SendTransaction(c.ctx, tx)
}

// GetSignatureStatuses 获取交易签名状态
// func (c *Client) GetSignatureStatuses(signatures []solana.Signature) (*rpc.GetSignatureStatusesResult, error) {
// 	return c.rpcClient.GetSignatureStatuses(c.ctx, signatures, nil)
// }
