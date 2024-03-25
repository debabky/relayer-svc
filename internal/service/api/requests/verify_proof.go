package requests

import (
	"encoding/json"
	snark "github.com/iden3/go-rapidsnark/types"
	"gitlab.com/distributed_lab/logan/v3/errors"
	"net/http"
)

type RegisterRequestData struct {
	InternalPublicKey struct {
		X string `json:"x"`
		Y string `json:"y"`
	} `json:"internal_public_key"`
	Signature struct {
		S string `json:"s"`
		N string `json:"n"`
	} `json:"signature"`
	Proof     snark.ZKProof `json:"proof"`
	Timestamp int64         `json:"timestamp"`
}

type RegisterRequest struct {
	Data RegisterRequestData `json:"data"`
}

func NewRegisterRequest(r *http.Request) (RegisterRequest, error) {
	var request RegisterRequest

	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		return request, errors.Wrap(err, "failed to unmarshal")
	}

	return request, nil
}
