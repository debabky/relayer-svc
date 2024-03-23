package handlers

import (
	"net/http"
	"strings"

	"github.com/debabky/relayer/internal/service/api/requests"
	"github.com/debabky/relayer/resources"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"gitlab.com/distributed_lab/ape"
	"gitlab.com/distributed_lab/ape/problems"
)

func CreateAccount(w http.ResponseWriter, r *http.Request) {
	req, err := requests.NewVerifyProofRequest(r)
	if err != nil {
		Log(r).WithError(err).Error("failed to get request")
		ape.RenderErr(w, problems.BadRequest(err)...)
		return
	}

	dataBytes, err := hexutil.Decode(req.Data.TxData)
	if err != nil {
		Log(r).WithError(err).Error("failed to decode data")
		ape.RenderErr(w, problems.BadRequest(err)...)
		return
	}

	// TODO(danielost): generate relevant bindings to interact with the contract

	gasPrice, err := EthClient(r).SuggestGasPrice(r.Context())
	if err != nil {
		Log(r).WithError(err).Error("failed to suggest gas price")
		ape.RenderErr(w, problems.InternalError())
		return
	}

	NetworkConfig(r).LockNonce()
	defer NetworkConfig(r).UnlockNonce()

	gas, err := EthClient(r).EstimateGas(r.Context(), ethereum.CallMsg{
		From:     crypto.PubkeyToAddress(NetworkConfig(r).PrivateKey.PublicKey),
		To:       nil, // TODO(danielost): replace with an actual contract
		GasPrice: gasPrice,
		Data:     dataBytes,
	})
	if err != nil {
		Log(r).WithError(err).Error("failed to estimate gas")
		ape.RenderErr(w, problems.InternalError())
		return
	}

	tx, err := types.SignNewTx(
		NetworkConfig(r).PrivateKey,
		types.NewCancunSigner(NetworkConfig(r).ChainID),
		&types.LegacyTx{
			Nonce:    NetworkConfig(r).Nonce(),
			Gas:      gas,
			GasPrice: gasPrice,
			To:       nil, // TODO(danielost): replace with an actual contract
			Data:     dataBytes,
		},
	)
	if err != nil {
		Log(r).WithError(err).Error("failed to sign new tx")
		ape.RenderErr(w, problems.InternalError())
		return
	}

	if err := EthClient(r).SendTransaction(r.Context(), tx); err != nil {
		if strings.Contains(err.Error(), "nonce") {
			if err := NetworkConfig(r).ResetNonce(EthClient(r)); err != nil {
				Log(r).WithError(err).Error("failed to reset nonce")
				ape.RenderErr(w, problems.InternalError())
				return
			}

			tx, err = types.SignNewTx(
				NetworkConfig(r).PrivateKey,
				types.NewCancunSigner(NetworkConfig(r).ChainID),
				&types.LegacyTx{
					Nonce:    NetworkConfig(r).Nonce(),
					Gas:      gas,
					GasPrice: gasPrice,
					To:       nil, // TODO(danielost): replace with an actual contract
					Data:     dataBytes,
				},
			)
			if err != nil {
				Log(r).WithError(err).Error("failed to sign new tx")
				ape.RenderErr(w, problems.InternalError())
				return
			}

			if err := EthClient(r).SendTransaction(r.Context(), tx); err != nil {
				Log(r).WithError(err).Error("failed to send transaction")
				ape.RenderErr(w, problems.InternalError())
				return
			}
		} else {
			Log(r).WithError(err).Error("failed to send transaction")
			ape.RenderErr(w, problems.InternalError())
			return
		}
	}

	NetworkConfig(r).IncrementNonce()

	ape.Render(w, resources.Tx{
		Key: resources.Key{
			ID:   tx.Hash().String(),
			Type: resources.TXS,
		},
		Attributes: resources.TxAttributes{
			TxHash: tx.Hash().String(),
		},
	})
}
