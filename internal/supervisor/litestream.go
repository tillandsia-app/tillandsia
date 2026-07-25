package supervisor

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/benbjohnson/litestream"
)

const (
	LitestreamConfigPath  = "/etc/litestream.yml"
	LitestreamRestoreTimeout = 60 * time.Second
	DataDir              = "/data"
	DBPath               = "/data/db"
	SyncInterval         = 1 * time.Second
)

type LitestreamEnv struct {
	AccessKeyID     string
	SecretAccessKey string
	URL             string
	Region          string
}

type LitestreamManager struct {
	env     *LitestreamEnv
	db      *litestream.DB
	replica *litestream.Replica
	running bool
	logger  *log.Logger
}

func NewLitestreamManager(env *LitestreamEnv, logger *log.Logger) *LitestreamManager {
	return &LitestreamManager{
		env:    env,
		logger: logger,
	}
}

func (lm *LitestreamManager) setupReplica(db *litestream.DB) error {
	client, err := litestream.NewReplicaClientFromURL(lm.env.URL)
	if err != nil {
		return fmt.Errorf("creating replica client: %w", err)
	}

	lm.replica = litestream.NewReplicaWithClient(db, client)
	lm.replica.SyncInterval = SyncInterval
	lm.replica.MonitorEnabled = true

	db.Replica = lm.replica

	return nil
}

func (lm *LitestreamManager) Restore(ctx context.Context) error {
	if lm.env.URL == "" {
		lm.logger.Println("litestream: no replica URL configured, skipping restore")
		return nil
	}

	dbPath := filepath.Join(DataDir, "db")

	// Check if DB already exists
	if _, err := os.Stat(dbPath); err == nil {
		lm.logger.Println("litestream: database already exists, skipping restore")
		return nil
	}

	restoreCtx, cancel := context.WithTimeout(ctx, LitestreamRestoreTimeout)
	defer cancel()

	lm.logger.Println("litestream: restoring from latest snapshot...")

	opts := litestream.NewRestoreOptions()
	opts.OutputPath = dbPath

	replica := &litestream.Replica{}
	if err := replica.Restore(restoreCtx, opts); err != nil {
		lm.logger.Printf("litestream: restore warning (starting fresh): %v", err)
		return nil
	}

	lm.logger.Println("litestream: restore complete")
	return nil
}

func (lm *LitestreamManager) StartReplication(ctx context.Context) error {
	if lm.env.URL == "" {
		return nil
	}

	dbPath := filepath.Join(DataDir, "db")

	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return fmt.Errorf("creating data directory: %w", err)
	}

	db := litestream.NewDB(dbPath)
	db.MonitorInterval = 1 * time.Second
	db.CheckpointInterval = 1 * time.Minute

	if err := db.Open(); err != nil {
		return fmt.Errorf("opening litestream db: %w", err)
	}

	if err := lm.setupReplica(db); err != nil {
		db.Close(ctx)
		return fmt.Errorf("setting up replica: %w", err)
	}

	lm.db = db

	go func() {
		if err := lm.replica.Start(ctx); err != nil {
			lm.logger.Printf("litestream: replication exited: %v", err)
		}
	}()

	lm.running = true
	lm.logger.Println("litestream: replication started")

	return nil
}

func (lm *LitestreamManager) Flush(ctx context.Context) {
	if lm.db == nil {
		return
	}

	lm.logger.Println("litestream: flushing WAL...")
	if err := lm.db.SyncAndWait(ctx); err != nil {
		lm.logger.Printf("litestream: sync warning: %v", err)
	}
	if _, err := lm.db.Snapshot(ctx); err != nil {
		lm.logger.Printf("litestream: snapshot warning: %v", err)
	}
}

func (lm *LitestreamManager) Stop() {
	if lm.replica != nil {
		lm.replica.Stop(true)
	}
	if lm.db != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		lm.db.Close(ctx)
	}
	lm.running = false
}

func (lm *LitestreamManager) IsRunning() bool {
	return lm.running
}

func (lm *LitestreamManager) Lag() int64 {
	if lm.db == nil {
		return 0
	}
	pos, err := lm.db.Pos()
	if err != nil {
		return 0
	}
	return int64(pos.TXID)
}