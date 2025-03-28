package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"sync/atomic"

	"github.com/isotronic/http-go-server/internal/database"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileServerHits atomic.Int32
	database *database.Queries
	platform string
	tokenSecret string
	polkaKey string
}

func main() {
	godotenv.Load()
	var server http.Server
	var apiCfg apiConfig
	mux := http.NewServeMux()
	server.Addr = ":8080"
	server.Handler = mux

	apiCfg.platform = os.Getenv("PLATFORM")
	apiCfg.tokenSecret = os.Getenv("TOKEN_SECRET")
	apiCfg.polkaKey = os.Getenv("POLKA_KEY")

	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		log.Fatalf("DB_URL must be set")
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}
	apiCfg.database = database.New(db)

	mux.Handle("/app/", apiCfg.middleWareMetricsInt(http.StripPrefix("/app", http.FileServer(http.Dir(".")))))

	mux.HandleFunc("GET /api/healthz", apiHealthzHandler)

	mux.HandleFunc("GET /api/chirps", apiGetAllChirpsHandler(&apiCfg))
	mux.HandleFunc("GET /api/chirps/{chirpID}", apiGetChirpByIdHandler(&apiCfg))
	mux.HandleFunc("POST /api/chirps", apiPostChirpsHandler(&apiCfg))
	mux.HandleFunc("DELETE /api/chirps/{chirpID}", apiDeleteChirpsHandler(&apiCfg))

	mux.HandleFunc("POST /api/users", apiCreateUserHandler(&apiCfg))
	mux.HandleFunc("PUT /api/users", apiUpdateUserHandler(&apiCfg))

	mux.HandleFunc("POST /api/login", apiLoginHandler(&apiCfg))
	mux.HandleFunc("POST /api/refresh", apiRefreshHandler(&apiCfg))
	mux.HandleFunc("POST /api/revoke", apiRevokeHandler(&apiCfg))

	mux.HandleFunc("POST /api/polka/webhooks", apiPolkaWebhooksHandler(&apiCfg))

	mux.HandleFunc("GET /admin/metrics", adminMetricsHandler(&apiCfg))
	mux.Handle("POST /admin/reset", apiCfg.middleWareMetricsReset(http.HandlerFunc(adminResetHandler(&apiCfg))))

	server.ListenAndServe()
}

func (cfg *apiConfig) middleWareMetricsInt(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileServerHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) middleWareMetricsReset(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cfg.platform == "dev" {
			cfg.fileServerHits.Swap(0)
		}
		next.ServeHTTP(w, r)
	})
}