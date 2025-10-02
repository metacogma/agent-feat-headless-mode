package apxmiddlewares

import (
	// Go Internal Packages
	"net/http"
	// External Packages

	"github.com/rs/cors"
)

// EnabCors creates CORS middleware using github.com/rs/cors package.
func EnabCors(allowedOrigins []string) func(next http.Handler) http.Handler {
	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowCredentials: true,
		AllowedHeaders:   []string{"*"},
	})

	return func(next http.Handler) http.Handler {
		return corsHandler.Handler(next)
	}
}
