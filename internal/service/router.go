package service

import (
	"github.com/debabky/relayer/internal/contracts"
	"github.com/debabky/relayer/internal/service/api/handlers"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/go-chi/chi"
	"gitlab.com/distributed_lab/ape"
	"gitlab.com/distributed_lab/logan/v3/errors"
)

func (s *service) router() chi.Router {
	r := chi.NewRouter()

	ethClient, err := ethclient.Dial(s.cfg.NetworkConfig().RPC)
	if err != nil {
		panic(errors.Wrap(err, "failed to dial connect"))
	}

	registrationContract, err := contracts.NewRegistration(s.cfg.NetworkConfig().RegistrationAddress, ethClient)
	if err != nil {
		panic(errors.Wrap(err, "failed to init new registration contract"))
	}

	r.Use(
		ape.RecoverMiddleware(s.log),
		ape.LoganMiddleware(s.log),
		ape.CtxMiddleware(
			handlers.CtxLog(s.log),
			handlers.CtxNetworkConfig(s.cfg.NetworkConfig()),
			handlers.CtxEthClient(ethClient),
			handlers.CtxRegistrationContract(registrationContract),
		),
	)

	r.Route("/integrations/registration-relayer", func(r chi.Router) {
		r.Route("/v1", func(r chi.Router) {
			r.Post("/register", handlers.Register)
		})
	})

	return r
}
