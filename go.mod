module github.com/JIsaacSamuel/chirpy

go 1.21.5

require github.com/go-chi/chi/v5 v5.0.11

require internal/database v1.0.0

require golang.org/x/crypto v0.18.0

require github.com/joho/godotenv v1.5.1

require github.com/golang-jwt/jwt/v5 v5.2.0 // indirect

replace internal/database => ./internal/database

require internal/auth v1.0.0

replace internal/auth => ./internal/auth
