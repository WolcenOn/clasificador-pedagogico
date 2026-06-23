package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"log"
	"os"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var cacheDB *sql.DB

func initCacheDB() (*sql.DB, func()) {
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		log.Println("Caché PostgreSQL desactivada: DATABASE_URL no está configurada")
		return nil, func() {}
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		log.Printf("Caché PostgreSQL desactivada: no se pudo abrir la conexión: %v", err)
		return nil, func() {}
	}

	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Printf("Caché PostgreSQL desactivada: no se pudo conectar: %v", err)
		_ = db.Close()
		return nil, func() {}
	}

	if err := migrateCacheDB(ctx, db); err != nil {
		log.Printf("Caché PostgreSQL desactivada: no se pudo migrar la tabla: %v", err)
		_ = db.Close()
		return nil, func() {}
	}

	log.Println("Caché PostgreSQL activada")
	return db, func() { _ = db.Close() }
}

func migrateCacheDB(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS resource_classifications (
			id BIGSERIAL PRIMARY KEY,
			file_hash TEXT NOT NULL UNIQUE,
			file_size BIGINT NOT NULL,
			mime_type TEXT NOT NULL,
			edad TEXT NOT NULL,
			area_principal TEXT NOT NULL,
			justificacion TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			last_used_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			uses_count INTEGER NOT NULL DEFAULT 1
		);

		CREATE INDEX IF NOT EXISTS idx_resource_classifications_last_used_at
		ON resource_classifications(last_used_at DESC);
	`)
	return err
}

func getCachedAnalysis(ctx context.Context, fileHash string) (AnalysisResult, bool) {
	if cacheDB == nil {
		return AnalysisResult{}, false
	}

	queryCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	tx, err := cacheDB.BeginTx(queryCtx, nil)
	if err != nil {
		log.Printf("Caché no disponible al iniciar transacción: %v", err)
		return AnalysisResult{}, false
	}
	defer tx.Rollback()

	var result AnalysisResult
	err = tx.QueryRowContext(queryCtx, `
		SELECT edad, area_principal, justificacion
		FROM resource_classifications
		WHERE file_hash = $1
	`, fileHash).Scan(&result.Edad, &result.AreaPrincipal, &result.Justificacion)
	if errors.Is(err, sql.ErrNoRows) {
		return AnalysisResult{}, false
	}
	if err != nil {
		log.Printf("Error consultando caché: %v", err)
		return AnalysisResult{}, false
	}

	_, err = tx.ExecContext(queryCtx, `
		UPDATE resource_classifications
		SET last_used_at = NOW(), uses_count = uses_count + 1
		WHERE file_hash = $1
	`, fileHash)
	if err != nil {
		log.Printf("Error actualizando uso de caché: %v", err)
		return AnalysisResult{}, false
	}

	if err := tx.Commit(); err != nil {
		log.Printf("Error confirmando lectura de caché: %v", err)
		return AnalysisResult{}, false
	}

	return result, true
}

func saveCachedAnalysis(ctx context.Context, fileHash string, fileSize int64, mimeType string, analysis AnalysisResult) error {
	if cacheDB == nil {
		return nil
	}

	queryCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	_, err := cacheDB.ExecContext(queryCtx, `
		INSERT INTO resource_classifications (
			file_hash, file_size, mime_type, edad, area_principal, justificacion
		) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (file_hash) DO UPDATE SET
			last_used_at = NOW(),
			uses_count = resource_classifications.uses_count + 1
	`, fileHash, fileSize, mimeType, analysis.Edad, analysis.AreaPrincipal, analysis.Justificacion)
	return err
}

func sha256Hex(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}
