package collectors

import (
	"context"
	"database/sql"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"net"
	"os"
	"time"
)

func (r Runner) CheckDatabase(ctx context.Context, o model.WorkspaceObject) (map[string]any, error) {
	if o.Engine != "postgresql" {
		return nil, errors.New("somente PostgreSQL possui conector SQL nesta versão")
	}
	raw := os.Getenv(o.CredentialEnv)
	if raw == "" {
		return nil, errors.New("variável de conexão não configurada no servidor")
	}
	cfg, err := pgx.ParseConfig(raw)
	if err != nil {
		return nil, errors.New("configuração PostgreSQL inválida")
	}
	if cfg.Host == "" || cfg.Host[0] == '/' {
		return nil, errors.New("use uma conexão TCP explícita")
	}
	cfg.DialFunc = func(ctx context.Context, network, address string) (net.Conn, error) {
		if network != "tcp" {
			return nil, errors.New("somente TCP")
		}
		return r.dial(ctx, network, address)
	}
	cfg.ConnectTimeout = 5 * time.Second
	cfg.RuntimeParams["default_transaction_read_only"] = "on"
	cfg.RuntimeParams["statement_timeout"] = "5000"
	cfg.RuntimeParams["application_name"] = "Vetra Observability"
	db := stdlib.OpenDB(*cfg)
	defer db.Close()
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(0)
	return CollectPostgres(ctx, db)
}

// Fixed read-only catalog queries; callers cannot supply SQL through a profile or workflow.
func CollectPostgres(ctx context.Context, db *sql.DB) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	start := time.Now()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, errors.New("conexão PostgreSQL recusada, sem permissão ou destino não permitido")
	}
	defer tx.Rollback()
	var name, version string
	if err = tx.QueryRowContext(ctx, "SELECT current_database(), current_setting('server_version')").Scan(&name, &version); err != nil {
		return nil, errors.New("não foi possível ler identificação do PostgreSQL")
	}
	var size int64
	if err = tx.QueryRowContext(ctx, "SELECT pg_database_size(current_database())").Scan(&size); err != nil {
		return nil, errors.New("sem permissão para métricas de tamanho")
	}
	var sessions, commits, rollbacks, deadlocks, hits, reads int64
	err = tx.QueryRowContext(ctx, "SELECT numbackends, xact_commit, xact_rollback, deadlocks, blks_hit, blks_read FROM pg_stat_database WHERE datname=current_database()").Scan(&sessions, &commits, &rollbacks, &deadlocks, &hits, &reads)
	if err != nil {
		return nil, errors.New("sem permissão para pg_stat_database")
	}
	return map[string]any{"engine": "postgresql", "database": name, "version": version, "latency_ms": float64(time.Since(start)) / float64(time.Millisecond), "size_bytes": size, "sessions": sessions, "commits_total": commits, "rollbacks_total": rollbacks, "deadlocks_total": deadlocks, "cache_hits_total": hits, "disk_reads_total": reads, "read_only": true}, nil
}
