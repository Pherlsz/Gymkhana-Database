package main

import (
	"context"
	"log/slog"

	"github.com/Pherlsz/Gymkhana-Database/internal/aichat"
	"github.com/Pherlsz/Gymkhana-Database/internal/attachment"
	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/bill"
	"github.com/Pherlsz/Gymkhana-Database/internal/config"
	"github.com/Pherlsz/Gymkhana-Database/internal/customdata"
	"github.com/Pherlsz/Gymkhana-Database/internal/document"
	"github.com/Pherlsz/Gymkhana-Database/internal/googleforms"
	"github.com/Pherlsz/Gymkhana-Database/internal/matching"
	"github.com/Pherlsz/Gymkhana-Database/internal/ocr"
	"github.com/Pherlsz/Gymkhana-Database/internal/operations"
	"github.com/Pherlsz/Gymkhana-Database/internal/profile"
	"github.com/Pherlsz/Gymkhana-Database/internal/queryengine"
	"github.com/Pherlsz/Gymkhana-Database/internal/search"
	"github.com/jackc/pgx/v5/pgxpool"
)

type runtimeServices struct {
	auth            *auth.Service
	profile         *profile.Service
	document        *document.Service
	bill            *bill.Service
	customData      *customdata.Service
	attachment      *attachment.Service
	search          *search.Service
	operations      *operations.Service
	googleForms     *googleforms.Service
	query           *queryengine.Service
	matching        *matching.Service
	chat            *aichat.Service
	chatTools       *aichat.ToolGateway
	chatCoordinator *aichat.Coordinator
	ocr             *ocr.Service
}

func configureServices(
	ctx context.Context,
	pool *pgxpool.Pool,
	cfg config.Config,
	storageCfg config.StorageConfig,
	logger *slog.Logger,
) (runtimeServices, error) {
	services := runtimeServices{}
	var err error
	services.auth, err = configureAuthentication(pool, cfg, logger)
	if err != nil {
		return runtimeServices{}, err
	}
	if pool == nil {
		return services, nil
	}
	if err := services.configureCore(pool, logger); err != nil {
		return runtimeServices{}, err
	}
	if err := services.configureStorage(pool, storageCfg, logger); err != nil {
		return runtimeServices{}, err
	}
	if err := services.configureGoogleForms(pool, cfg, logger); err != nil {
		return runtimeServices{}, err
	}
	if err := services.configureAIChat(ctx, pool, cfg, logger); err != nil {
		return runtimeServices{}, err
	}
	if err := services.configureOCR(pool, cfg, logger); err != nil {
		return runtimeServices{}, err
	}
	return services, nil
}
