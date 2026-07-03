package server

import (
	"net/http"
	"time"

	"github.com/alireza/tvtime2serializd/internal/cache"
	"github.com/alireza/tvtime2serializd/internal/crypto"
	"github.com/alireza/tvtime2serializd/internal/handler"
	"github.com/alireza/tvtime2serializd/internal/mapping"
	"github.com/alireza/tvtime2serializd/internal/outbound"
	"github.com/alireza/tvtime2serializd/internal/repository"
	"github.com/alireza/tvtime2serializd/internal/serializd"
	"github.com/alireza/tvtime2serializd/internal/service"
	"github.com/alireza/tvtime2serializd/internal/tmdb"
	"github.com/alireza/tvtime2serializd/internal/tvtime"
	"github.com/alireza/tvtime2serializd/internal/wikidata"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Deps struct {
	DB                 *pgxpool.Pool
	Redis              *redis.Client
	TVTime             *tvtime.Client
	Serializd          *serializd.Client
	TokenEncryptionKey string
	TMDBAPIKey         string
	CORSAllowedOrigins []string
}

type Server struct {
	router http.Handler
}

func New(deps Deps) (*Server, error) {
	cipher, err := crypto.NewFromBase64Key(deps.TokenEncryptionKey)
	if err != nil {
		return nil, err
	}

	tokens := repository.NewTokenRepository(deps.DB, cipher)
	serializdTokens := repository.NewSerializdTokenRepository(deps.DB, cipher)
	sessions := cache.NewSessionCache(deps.Redis)
	serializdSessions := cache.NewSerializdSessionCache(deps.Redis)
	exports := repository.NewExportRepository(deps.DB)
	migrateJobs := repository.NewMigrateJobRepository(deps.DB)
	serializdImported := repository.NewSerializdImportedRepository(deps.DB)
	unresolved := repository.NewUnresolvedShowRepository(deps.DB)

	gates := outbound.NewSetFromEnv()
	deps.TVTime.SetOutboundGate(gates.TVTime)
	deps.Serializd.SetOutboundGate(gates.Serializd)

	wiki := wikidata.NewClient()
	wiki.SetOutboundGate(gates.Wikidata)
	tmdbClient := tmdb.NewClient(deps.TMDBAPIKey)
	tmdbClient.SetOutboundGate(gates.TMDB)
	resolver := mapping.NewTMDBResolver(mapping.TMDBResolverDeps{
		Wikidata: wiki,
		TMDB:     tmdbClient,
	})
	shows := repository.NewShowCatalog(deps.DB)
	tmdbCache := cache.NewTMDBCache(deps.Redis)
	showLookup := service.NewShowLookupService(service.ShowLookupDeps{
		TMDBCache:  tmdbCache,
		Shows:      shows,
		Unresolved: unresolved,
		Resolver:   resolver,
	})
	exportService := service.NewExportService(service.ExportDeps{
		TVTime:     deps.TVTime,
		Tokens:     tokens,
		Exports:    exports,
		ShowLookup: showLookup,
	})
	migrateService := service.NewMigrateService(service.MigrateDeps{
		TVTime:          deps.TVTime,
		Serializd:       deps.Serializd,
		Tokens:          tokens,
		ShowLookup:      showLookup,
		Exports:         exports,
		Jobs:            migrateJobs,
		Progress:        cache.NewMigrateProgressCache(deps.Redis),
		ImportedShows:   cache.NewImportedShowsCache(deps.Redis),
		ImportedShowsDB: serializdImported,
	})

	credentialLimiter := newCredentialRateLimiter()

	r := chi.NewRouter()
	useClientIP(r)
	r.Use(securityHeaders)
	r.Use(corsMiddleware(deps.CORSAllowedOrigins))
	r.Use(bodyLimit(maxRequestBodyBytes))
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	healthHandler := handler.NewHealth(handler.HealthDeps{
		DB:    deps.DB,
		Redis: deps.Redis,
	})
	loginHandler := handler.NewLogin(handler.LoginDeps{
		TVTime:   deps.TVTime,
		Tokens:   tokens,
		Sessions: sessions,
	})
	serializdLoginHandler := handler.NewSerializdLogin(handler.SerializdLoginDeps{
		Client:   deps.Serializd,
		Tokens:   serializdTokens,
		Sessions: serializdSessions,
	})
	exportHandler := handler.NewExport(exportService)
	migrateHandler := handler.NewMigrate(migrateService)

	r.Group(func(r chi.Router) {
		r.Use(middleware.Timeout(60 * time.Second))

		r.Get("/health", healthHandler.ServeHTTP)

		r.Route("/tvtime", func(r chi.Router) {
			r.With(credentialLimiter.Handler).Post("/login", loginHandler.ServeHTTP)
			r.Post("/export", exportHandler.Create)
		})

		r.Route("/serializd", func(r chi.Router) {
			r.With(credentialLimiter.Handler).Post("/login", serializdLoginHandler.ServeHTTP)
		})

		r.Route("/migrate", func(r chi.Router) {
			r.With(credentialLimiter.Handler).Post("/init", migrateHandler.Init)
			r.Get("/init/{id}", migrateHandler.Get)
		})
	})

	r.Group(func(r chi.Router) {
		r.Use(middleware.Timeout(30 * time.Minute))

		r.Get("/tvtime/export/{id}", exportHandler.Get)
		r.Get("/tvtime/export/{id}/download", exportHandler.Download)
		r.Get("/migrate/init/{id}/stream", migrateHandler.Stream)
	})

	return &Server{router: r}, nil
}

func (s *Server) Handler() http.Handler {
	return s.router
}
