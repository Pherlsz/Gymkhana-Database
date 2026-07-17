package googleforms

import (
	"fmt"

	"github.com/Pherlsz/Gymkhana-Database/internal/operations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

type RuntimeOptions struct {
	Service      ServiceOptions
	ClientID     string
	ClientSecret string
	RedirectURL  string
	KeyVersion   uint16
	Keys         map[uint16][32]byte
}

func NewRuntime(pool *pgxpool.Pool, operationService *operations.Service, options RuntimeOptions, execute bool) (*Service, *river.Client[pgx.Tx], error) {
	if pool == nil || operationService == nil {
		return nil, nil, ErrInvalidInput
	}
	jobs := NewRiverJobs()
	var provider Provider
	var cipher *TokenCipher
	var err error
	if options.Service.Enabled {
		provider, err = NewGoogleProvider(GoogleProviderOptions{
			ClientID: options.ClientID, ClientSecret: options.ClientSecret, RedirectURL: options.RedirectURL,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("configure Google Forms provider: %w", err)
		}
		cipher, err = NewTokenCipher(options.KeyVersion, options.Keys)
		if err != nil {
			return nil, nil, fmt.Errorf("configure Google Forms token encryption: %w", err)
		}
	}
	var workers *river.Workers
	if execute {
		workers = river.NewWorkers()
	}
	service, err := NewService(NewPostgresStore(pool), provider, cipher, jobs, operationService, options.Service)
	if err != nil {
		return nil, nil, err
	}
	if execute {
		if err := RegisterRiverWorkers(workers, service); err != nil {
			return nil, nil, err
		}
	}
	client, err := NewRiverClient(pool, workers, execute)
	if err != nil {
		return nil, nil, err
	}
	if err := jobs.SetClient(client); err != nil {
		return nil, nil, err
	}
	return service, client, nil
}
