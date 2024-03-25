package handlers

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/debabky/relayer/internal/contracts"
	"github.com/debabky/relayer/internal/service/api/requests"
	"github.com/debabky/relayer/resources"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"gitlab.com/distributed_lab/ape"
	"gitlab.com/distributed_lab/ape/problems"
	"gitlab.com/distributed_lab/logan/v3/errors"
)

func Register(w http.ResponseWriter, r *http.Request) {
	req, err := requests.NewRegisterRequest(r)
	if err != nil {
		Log(r).WithError(err).Error("failed to get request")
		ape.RenderErr(w, problems.BadRequest(err)...)
		return
	}

	x, y, s, n, a, b, c, err := getRegistrationData(req)
	if err != nil {
		Log(r).WithError(err).Error("failed to get registration data")
		ape.RenderErr(w, problems.InternalError())
		return
	}
	timestampUnix := time.Unix(req.Data.Timestamp, 0)
	timestampInt := int64(timestampUnix.Day()) | int64(timestampUnix.Month()<<8) | int64((timestampUnix.Year()-2000)<<16)

	NetworkConfig(r).LockNonce()
	defer NetworkConfig(r).UnlockNonce()

	_, err = RegistrationContract(r).Register(
		&bind.TransactOpts{
			NoSend: true,
			From:   crypto.PubkeyToAddress(NetworkConfig(r).PrivateKey.PublicKey),
			Signer: func(addr common.Address, tx *types.Transaction) (*types.Transaction, error) {
				return types.SignTx(
					tx, types.NewCancunSigner(NetworkConfig(r).ChainID), NetworkConfig(r).PrivateKey,
				)
			},
		},
		x, y, s, n,
		contracts.VerifierHelperProofPoints{
			A: a,
			B: b,
			C: c,
		},
		big.NewInt(timestampInt),
		big.NewInt(0),
	)
	if err != nil {
		Log(r).WithError(err).Error("failed to check transaction validity")
		ape.RenderErr(w, problems.InternalError())
		return
	}

	tx, err := RegistrationContract(r).Register(
		&bind.TransactOpts{
			From:  crypto.PubkeyToAddress(NetworkConfig(r).PrivateKey.PublicKey),
			Nonce: new(big.Int).SetUint64(NetworkConfig(r).Nonce()),
			Signer: func(addr common.Address, tx *types.Transaction) (*types.Transaction, error) {
				return types.SignTx(
					tx, types.NewCancunSigner(NetworkConfig(r).ChainID), NetworkConfig(r).PrivateKey,
				)
			},
		},
		x, y, s, n,
		contracts.VerifierHelperProofPoints{
			A: a,
			B: b,
			C: c,
		},
		big.NewInt(timestampInt),
		big.NewInt(0),
	)
	if err != nil {
		if strings.Contains(err.Error(), "nonce") {
			if err := NetworkConfig(r).ResetNonce(EthClient(r)); err != nil {
				Log(r).WithError(err).Error("failed to reset nonce")
				ape.RenderErr(w, problems.InternalError())
				return
			}

			tx, err = RegistrationContract(r).Register(
				&bind.TransactOpts{
					From:  crypto.PubkeyToAddress(NetworkConfig(r).PrivateKey.PublicKey),
					Nonce: new(big.Int).SetUint64(NetworkConfig(r).Nonce()),
					Signer: func(addr common.Address, tx *types.Transaction) (*types.Transaction, error) {
						return types.SignTx(
							tx, types.NewCancunSigner(NetworkConfig(r).ChainID), NetworkConfig(r).PrivateKey,
						)
					},
				},
				x, y, s, n,
				contracts.VerifierHelperProofPoints{
					A: a,
					B: b,
					C: c,
				},
				big.NewInt(req.Data.Timestamp),
				big.NewInt(0),
			)
			if err != nil {
				Log(r).WithError(err).Error("failed to send registration tx")
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

func getRegistrationData(req requests.RegisterRequest) (
	[32]byte, [32]byte, []byte, []byte, [2]*big.Int, [2][2]*big.Int, [2]*big.Int, error,
) {
	x, err := hex.DecodeString(req.Data.InternalPublicKey.X)
	if err != nil {
		return [32]byte{}, [32]byte{}, nil, nil, [2]*big.Int{}, [2][2]*big.Int{}, [2]*big.Int{}, errors.Wrap(err, "failed to decode hex")
	}
	xArr := [32]byte{}
	copy(xArr[:], x)

	y, err := hex.DecodeString(req.Data.InternalPublicKey.Y)
	if err != nil {
		return [32]byte{}, [32]byte{}, nil, nil, [2]*big.Int{}, [2][2]*big.Int{}, [2]*big.Int{}, errors.Wrap(err, "failed to decode hex")
	}
	yArr := [32]byte{}
	copy(yArr[:], y)

	s, err := hex.DecodeString(req.Data.Signature.S)
	if err != nil {
		return [32]byte{}, [32]byte{}, nil, nil, [2]*big.Int{}, [2][2]*big.Int{}, [2]*big.Int{}, errors.Wrap(err, "failed to decode hex")
	}

	n, err := hex.DecodeString(req.Data.Signature.N)
	if err != nil {
		return [32]byte{}, [32]byte{}, nil, nil, [2]*big.Int{}, [2][2]*big.Int{}, [2]*big.Int{}, errors.Wrap(err, "failed to decode hex")
	}

	a, b, c, err := getProofPoints(req)
	if err != nil {
		return [32]byte{}, [32]byte{}, nil, nil, [2]*big.Int{}, [2][2]*big.Int{}, [2]*big.Int{}, errors.Wrap(err, "failed to get proof points")
	}

	return xArr, yArr, s, n, a, b, c, nil
}

func getProofPoints(req requests.RegisterRequest) ([2]*big.Int, [2][2]*big.Int, [2]*big.Int, error) {
	a, err := stringsToArrayBigInt(req.Data.Proof.Proof.A)
	if err != nil {
		return [2]*big.Int{}, [2][2]*big.Int{}, [2]*big.Int{}, errors.Wrap(err, "failed to convert stings to big ints")
	}
	resB := [2][2]*big.Int{}
	for i, b := range req.Data.Proof.Proof.B {
		bi, err := stringsToArrayBigInt(b)
		if err != nil {
			return [2]*big.Int{}, [2][2]*big.Int{}, [2]*big.Int{}, errors.Wrap(err, "failed to convert stings to big ints")
		}
		biArr := [2]*big.Int{}
		copy(biArr[:], bi)
		resB[i] = biArr
	}
	c, err := stringsToArrayBigInt(req.Data.Proof.Proof.C)
	if err != nil {
		return [2]*big.Int{}, [2][2]*big.Int{}, [2]*big.Int{}, errors.Wrap(err, "failed to convert stings to big ints")
	}

	resA := [2]*big.Int{}
	copy(resA[:], a)

	resC := [2]*big.Int{}
	copy(resC[:], c)

	return resA, resB, resC, nil
}

func stringsToArrayBigInt(publicSignals []string) ([]*big.Int, error) {
	p := make([]*big.Int, 0, len(publicSignals))
	for _, s := range publicSignals {
		sb, err := stringToBigInt(s)
		if err != nil {
			return nil, err
		}
		p = append(p, sb)
	}
	return p, nil
}

func stringToBigInt(s string) (*big.Int, error) {
	base := 10
	if bytes.HasPrefix([]byte(s), []byte("0x")) {
		base = 16
		s = strings.TrimPrefix(s, "0x")
	}
	n, ok := new(big.Int).SetString(s, base)
	if !ok {
		return nil, fmt.Errorf("can not parse string to *big.Int: %s", s)
	}
	return n, nil
}
