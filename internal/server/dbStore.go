package server

import (
	ctx "context"
	"errors"
	"fmt"

	log "metrics/internal/logger"
	s "metrics/internal/service"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type dbOperation uint8

const (
	insertMetric dbOperation = iota
	selectMetric
)

const (
	insertCounter = "insertCounter"
	insertGauge   = "insertGauge"
	selectGauge   = "selectGauge"
	selectCounter = "selectCounter"
	selectAll     = "selectAll"
)

type DataBase struct {
	*pgxpool.Pool
}

var ErrConnDB = errors.New("db connection error")

func NewDB(cx ctx.Context, addr string) (*DataBase, error) {
	if err := runMigrations(addr); err != nil {
		return nil, err
	}
	config, err := pgxpool.ParseConfig(addr)
	if err != nil {
		return nil, fmt.Errorf("newDB: unable to parse connection string: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(cx, config)
	if err != nil {
		return nil, fmt.Errorf("newDB: unable to create connection pool: %w", err)
	}
	if err := prepareQueries(cx, pool); err != nil {
		return nil, err
	}
	return &DataBase{pool}, nil
}

func (db *DataBase) Put(cx ctx.Context, met *s.Metrics) (*s.Metrics, error) {
	conn, err := db.connectWithRetry(cx)
	if err != nil {
		return nil, fmt.Errorf("db put conn err: %w", err)
	}
	defer conn.Release()

	var val any
	query := getQuery(insertMetric, met)
	if err = conn.QueryRow(cx, query, met.ToSlice()...).Scan(&val); err != nil {
		return nil, fmt.Errorf("db put queryRow error: %w", err)
	}
	setVal(met, val)
	return met, nil
}

func (db *DataBase) Get(cx ctx.Context, met *s.Metrics) (*s.Metrics, error) {
	conn, err := db.connectWithRetry(cx)
	if err != nil {
		return nil, fmt.Errorf("db get conn err: %w", err)
	}
	defer conn.Release()

	query := getQuery(selectMetric, met)
	var val any
	if err = conn.QueryRow(cx, query, met.ID).Scan(&val); err != nil {
		return nil, fmt.Errorf("db get failed to execute query: %w", err)
	}
	setVal(met, val)
	return met, nil
}

func (db *DataBase) List(cx ctx.Context) (metrics []*s.Metrics, err error) {
	conn, err := db.connectWithRetry(cx)
	if err != nil {
		return nil, fmt.Errorf("db list conn err: %w", err)
	}
	defer conn.Release()

	rows, err := conn.Query(cx, selectAll)
	if err != nil {
		return nil, fmt.Errorf("dbList query err: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var met s.Metrics
		var val any
		if err := rows.Scan(&met.ID, &val); err != nil {
			return nil, fmt.Errorf("dbList query scan err: %w", err)
		}
		setVal(&met, val)
		metrics = append(metrics, &met)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("dbList query rows error: %w", err)
	}
	return metrics, nil
}

func (db *DataBase) PutBatch(cx ctx.Context, mets []*s.Metrics) error {
	conn, err := db.connectWithRetry(cx)
	if err != nil {
		return fmt.Errorf("putBatch err: %w", err)
	}
	defer conn.Release()

	tx, err := conn.Begin(cx)
	if err != nil {
		return fmt.Errorf("failed transaction beginning: %w", err)
	}
	defer func() { _ = tx.Rollback(cx) }()

	batch := &pgx.Batch{}
	for _, met := range mets {
		batch.Queue(getQuery(insertMetric, met), met.ToSlice()...)
	}
	br := tx.SendBatch(cx, batch)
	if _, err := br.Exec(); err != nil {
		return fmt.Errorf("batch exec failed: %w", err)
	}
	if err := br.Close(); err != nil {
		return fmt.Errorf("failed to close batch res: %w", err)
	}
	if err := tx.Commit(cx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

func prepareQueries(cx ctx.Context, pool *pgxpool.Pool) error {
	conn, err := pool.Acquire(cx)
	if err != nil {
		return fmt.Errorf("prepareQueries conn err: %w", err)
	}
	defer conn.Release()

	queries := map[string]string{
		insertGauge: `INSERT INTO gauge(id, value) VALUES($1, $2) 
			          ON CONFLICT(id) 
				      DO UPDATE SET value = EXCLUDED.value
				      RETURNING value`,

		insertCounter: `INSERT INTO counter(id, value) VALUES($1, $2) 
			            ON CONFLICT(id) 
				        DO UPDATE SET value = counter.value + excluded.value
				        RETURNING value`,

		selectGauge: `SELECT value FROM gauge WHERE id = $1`,

		selectCounter: `SELECT value FROM counter WHERE id = $1`,

		selectAll: `SELECT id, value FROM gauge
			        UNION ALL
			        SELECT id, value FROM counter;`,
	}
	for name, query := range queries {
		if _, err = conn.Conn().Prepare(cx, name, query); err != nil {
			return fmt.Errorf("prepareQueries %s err: %w", name, err)
		}
	}
	return nil
}

func runMigrations(addr string) error {
	m, err := migrate.New("file://migrations", addr)
	if err != nil {
		return fmt.Errorf("migrate err: %w", err)
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate err: %w", err)
	}
	log.Debug("success migrations...")
	return nil
}

func (db *DataBase) connectWithRetry(cx ctx.Context) (*pgxpool.Conn, error) {
	conn, err := db.Acquire(cx)
	if err != nil {
		if err = s.Retry(cx, func() error {
			conn, err = db.Acquire(cx)
			return ErrConnDB
		}); err != nil {
			return nil, ErrConnDB
		}
	}
	if err = conn.Conn().Ping(cx); err != nil {
		if err = s.Retry(cx, func() error {
			err = conn.Conn().Ping(cx)
			return ErrConnDB
		}); err != nil {
			return nil, ErrConnDB
		}
	}
	log.Debug("DB connection is established")
	return conn, nil
}
