module github.com/JIsaacSamuel/chirpy

go 1.21.5

require github.com/go-chi/chi/v5 v5.0.11

require internal/database v1.0.0

require golang.org/x/crypto v0.18.0

replace internal/database => ./internal/database
