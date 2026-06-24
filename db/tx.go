package db

import (
	"context"
	"fmt"
	"log/slog"

	"gorm.io/gorm"
)

type txContextKey struct{}

type TxManager struct {
	db     *gorm.DB
	logger *slog.Logger
}

func NewTxManager(db *gorm.DB, logger *slog.Logger) *TxManager {
	if logger == nil {
		logger = slog.Default()
	}
	return &TxManager{db: db, logger: logger.With("component", "dbtx")}
}

func (m *TxManager) DB(ctx context.Context) *gorm.DB {
	if tx, ok := TxFromContext(ctx); ok {
		return tx
	}
	if m == nil {
		return nil
	}
	return m.db.WithContext(ctx)
}

func (m *TxManager) WithTx(ctx context.Context, fn func(ctx context.Context, tx *gorm.DB) error) error {
	if m == nil || m.db == nil {
		return fmt.Errorf("tx manager db is nil")
	}
	if existing, ok := TxFromContext(ctx); ok {
		return fn(ctx, existing)
	}
	m.logger.Debug("transaction started")
	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(NewTxContext(ctx, tx), tx)
	})
}

func NewTxContext(ctx context.Context, tx *gorm.DB) context.Context {
	return context.WithValue(ctx, txContextKey{}, tx)
}
func TxFromContext(ctx context.Context) (*gorm.DB, bool) {
	tx, ok := ctx.Value(txContextKey{}).(*gorm.DB)
	return tx, ok && tx != nil
}

func Exec(ctx context.Context, db *gorm.DB, sql string, args ...any) error {
	if db == nil {
		return fmt.Errorf("gorm db is nil")
	}
	return db.WithContext(ctx).Exec(sql, args...).Error
}
