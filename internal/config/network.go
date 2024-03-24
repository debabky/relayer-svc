package config

import (
	"context"
	"crypto/ecdsa"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"gitlab.com/distributed_lab/figure/v3"
	"gitlab.com/distributed_lab/kit/comfig"
	"gitlab.com/distributed_lab/kit/kv"
	"gitlab.com/distributed_lab/logan/v3/errors"
	"math/big"
	"sync"
)

type NetworkConfiger interface {
	NetworkConfig() *NetworkConfig
}

func NewNetworkConfiger(getter kv.Getter) NetworkConfiger {
	return &ethereum{
		getter: getter,
	}
}

type ethereum struct {
	once   comfig.Once
	getter kv.Getter
}

type NetworkConfig struct {
	RPC                 string            `fig:"rpc,required"`
	RegistrationAddress common.Address    `fig:"registration_address,required"`
	PrivateKey          *ecdsa.PrivateKey `fig:"private_key,required"`

	ChainID *big.Int
	nonce   uint64
	mut     *sync.Mutex
}

func (e *ethereum) NetworkConfig() *NetworkConfig {
	return e.once.Do(func() interface{} {
		var result NetworkConfig

		err := figure.
			Out(&result).
			With(figure.BaseHooks, figure.EthereumHooks).
			From(kv.MustGetStringMap(e.getter, "network")).
			Please()
		if err != nil {
			panic(errors.Wrap(err, "failed to figure out ethereum config"))
		}

		ethClient, err := ethclient.Dial(result.RPC)
		if err != nil {
			panic(errors.Wrap(err, "failed to create connection"))
		}
		defer ethClient.Close()

		chainID, err := ethClient.ChainID(context.Background())
		if err != nil {
			return errors.Wrap(err, "failed to get chain ID")
		}

		result.ChainID = chainID

		nonce, err := ethClient.NonceAt(context.Background(), crypto.PubkeyToAddress(result.PrivateKey.PublicKey), nil)
		if err != nil {
			panic(errors.Wrap(err, "failed to get nonce"))
		}

		result.nonce = nonce

		result.mut = &sync.Mutex{}

		return &result
	}).(*NetworkConfig)
}

func (n *NetworkConfig) LockNonce() {
	n.mut.Lock()
}

func (n *NetworkConfig) UnlockNonce() {
	n.mut.Unlock()
}

func (n *NetworkConfig) Nonce() uint64 {
	return n.nonce
}

func (n *NetworkConfig) IncrementNonce() {
	n.nonce++
}

// ResetNonce sets nonce to the value received from a node
func (n *NetworkConfig) ResetNonce(client *ethclient.Client) error {
	nonce, err := client.NonceAt(context.Background(), crypto.PubkeyToAddress(n.PrivateKey.PublicKey), nil)
	if err != nil {
		return errors.Wrap(err, "failed to get nonce")
	}
	n.nonce = nonce
	return nil
}
