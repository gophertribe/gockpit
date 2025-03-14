package audit

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sync"
	"time"

	"github.com/google/uuid"

	"go.etcd.io/bbolt"
)

const (
	logBucket = "audit_log"
)

const (
	levelInfo  = "info"
	levelError = "error"
)

const txmax = 1 << 25

func init() {
	gob.Register(Event{})
	gob.Register(MsgClearError{})
}

type MsgClearError struct{}

type Publisher interface {
	Publish(ctx context.Context, msg interface{}) error
}

type StdLogger interface {
	Info(string)
	Infof(string, ...interface{})
}

type Bolt struct {
	mx        sync.Mutex
	path      string
	pub       Publisher
	logger    StdLogger
	db        *bbolt.DB
	listeners map[string]map[string]func(*Event)
}

func NewBolt(path string, pub Publisher, logger StdLogger) (*Bolt, error) {
	b := &Bolt{
		path:      path,
		pub:       pub,
		logger:    logger,
		listeners: map[string]map[string]func(*Event){},
	}
	var err error
	b.db, err = bbolt.Open(path, 0600, bbolt.DefaultOptions)
	if err != nil {
		return b, fmt.Errorf("could not open store from %s: %w", path, err)
	}
	tx, err := b.db.Begin(true)
	if err != nil {
		return b, fmt.Errorf("could not open database transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.CreateBucketIfNotExists([]byte(logBucket))
	if err != nil {
		return b, fmt.Errorf("could not initialize log bucket: %w", err)
	}
	err = tx.Commit()
	if err != nil {
		return b, fmt.Errorf("could not commit transaction: %w", err)
	}
	return b, nil
}

func (b *Bolt) RegisterListener(ns, msg string, listener func(*Event)) {
	b.mx.Lock()
	defer b.mx.Unlock()
	namespace := b.listeners[ns]
	if namespace == nil {
		namespace = map[string]func(*Event){}
		b.listeners[ns] = namespace
	}
	namespace[msg] = listener
}

func (b *Bolt) Close() error {
	if b.db == nil {
		return nil
	}
	err := b.db.Close()
	if err != nil {
		return fmt.Errorf("could not close underlying database: %w", err)
	}
	return nil
}

func (b *Bolt) Log(ctx context.Context, l *Event) error {
	b.mx.Lock()
	defer b.mx.Unlock()
	tx, err := b.db.Begin(true)
	if err != nil {
		return fmt.Errorf("could not open database transaction: %w", err)
	}
	bucket := tx.Bucket([]byte(logBucket))
	seq, err := bucket.NextSequence()
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("could not increment sequence: %w", err)
	}
	l.Seq = seq
	key := make([]byte, 10)
	binary.BigEndian.PutUint64(key, uint64(l.Timestamp))
	var val bytes.Buffer
	err = gob.NewEncoder(&val).Encode(l)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("could not encode value: %w", err)
	}
	err = tx.Bucket([]byte(logBucket)).Put(key, val.Bytes())
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("log save failed: %w", err)
	}
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("could not commit transaction: %w", err)
	}
	ns := b.listeners[l.Namespace]
	if ns != nil {
		if listener, ok := ns[l.Event]; ok {
			listener(l)
		}
	}
	return b.pub.Publish(ctx, l)
}

type RetentionLoopOptions struct {
	LogRetention  time.Duration
	LogMaxRecords int
	Compact       bool
}

func (b *Bolt) RetentionLoop(ctx context.Context, opts RetentionLoopOptions, period time.Duration, wg *sync.WaitGroup) {
	b.logger.Info("starting audit retention loop")
	wg.Add(1)
	go func() {
		defer wg.Done()
		now := time.Now()
		if opts.LogRetention > 0 {
			err := b.removeBefore(now.Add(-opts.LogRetention))
			if err != nil {
				b.logger.Infof("could not evict outdated audit logs: %v", err)
			}
		}
		if opts.LogMaxRecords > 0 {
			err := b.removeOverLimit(opts.LogMaxRecords)
			if err != nil {
				b.logger.Infof("could not evict audit logs over limit: %v", err)
			}
		}
		if opts.Compact {
			err := b.compact()
			if err != nil {
				b.logger.Infof("could not compact database: %v", err)
			}
		}
		b.logger.Infof("audit retention loop iteration executed in %v", time.Since(now))
		for {
			select {
			case now := <-time.After(period):
				if opts.LogRetention > 0 {
					err := b.removeBefore(now.Add(-opts.LogRetention))
					if err != nil {
						b.logger.Infof("could not evict outdated audit logs: %v", err)
					}
				}
				if opts.LogMaxRecords > 0 {
					err := b.removeOverLimit(opts.LogMaxRecords)
					if err != nil {
						b.logger.Infof("could not evict audit logs over limit: %v", err)
					}
				}
				if opts.Compact {
					err := b.compact()
					if err != nil {
						b.logger.Infof("could not compact database: %v", err)
					}
				}
				b.logger.Infof("audit retention loop iteration executed in %v", time.Since(now))
			case <-ctx.Done():
				b.logger.Info("terminating audit retention loop")
				return
			}
		}
	}()
}

func (b *Bolt) removeOverLimit(maxRecords int) error {
	tx, err := b.db.Begin(true)
	if err != nil {
		return fmt.Errorf("could not open database transaction: %w", err)
	}
	defer func() {
		err = tx.Rollback()
		if err != nil && !errors.Is(err, bbolt.ErrTxClosed) {
			b.logger.Infof("could not rollback transaction: %v", err)
		}
	}()
	bucket := tx.Bucket([]byte(logBucket))
	c := bucket.Cursor()
	if diff := bucket.Stats().KeyN - maxRecords; diff > 0 {
		var key []byte
		for key, _ = c.First(); key != nil && diff > 0; key, _ = c.Next() {
			if err := c.Delete(); err != nil {
				return fmt.Errorf("could not delete key: %w", err)
			}
			diff--
		}
	}
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("could not commit transaction: %w", err)
	}
	return nil
}

func (b *Bolt) removeBefore(stamp time.Time) error {
	b.mx.Lock()
	defer b.mx.Unlock()
	tx, err := b.db.Begin(true)
	if err != nil {
		return fmt.Errorf("could not open database transaction: %w", err)
	}
	defer func() {
		err = tx.Rollback()
		if err != nil && !errors.Is(err, bbolt.ErrTxClosed) {
			b.logger.Infof("could not rollback transaction: %v", err)
		}
	}()
	bucket := tx.Bucket([]byte(logBucket))
	limit := uint64(stamp.Unix())
	c := bucket.Cursor()
	for key, _ := c.First(); key != nil; key, _ = c.Next() {
		stamp := binary.BigEndian.Uint64(key)
		if stamp > limit {
			continue
		}
		if err := c.Delete(); err != nil {
			return fmt.Errorf("could not delete key %d: %w", stamp, err)
		}
	}
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("could not commit transaction: %w", err)
	}
	return nil
}

func (b *Bolt) compact() error {
	// compact the database
	nextPath := b.path + ".next"
	next, err := bbolt.Open(nextPath, 0600, bbolt.DefaultOptions)
	if err != nil {
		return fmt.Errorf("could not open next audit store from %s: %w", nextPath, err)
	}
	err = bbolt.Compact(next, b.db, txmax)
	_ = next.Close()
	_ = b.db.Close()
	if err != nil {
		return fmt.Errorf("could not compact the database: %w", err)
	}
	err = os.Rename(nextPath, b.path)
	if err != nil {
		return fmt.Errorf("could not replace compacted database: %w", err)
	}
	b.db, err = bbolt.Open(b.path, 0600, bbolt.DefaultOptions)
	if err != nil {
		return fmt.Errorf("could not open store from %s: %w", b.path, err)
	}
	return nil
}

func (b *Bolt) Info(ctx context.Context, namespace, code string, payload interface{}) {
	b.log(ctx, levelInfo, namespace, code, payload)
}

func (b *Bolt) Error(ctx context.Context, namespace, code string, payload interface{}) {
	b.log(ctx, levelError, namespace, code, payload)
}

func (b *Bolt) SetError(ctx context.Context, ns, code string, err error) {
	b.Error(ctx, ns, code, err)
}

func (b *Bolt) ClearError(ctx context.Context, ns, code string, err error) {
	b.Info(ctx, ns, code, MsgClearError{})
}

func (b *Bolt) log(ctx context.Context, level, namespace, code string, payload interface{}) {
	l := get()
	l.ID = uuid.New().String()
	l.Event = code
	l.Namespace = namespace
	l.Timestamp = time.Now().UnixNano()
	l.Level = level
	if payload != nil {
		l.Type = reflect.TypeOf(payload).String()
		l.Payload = payload
	}
	defer collect(l)
	err := b.Log(ctx, l)
	if err != nil {
		b.logger.Infof("could not log audit %s message: %v", level, err)
	}
}

func (b *Bolt) GetPage(page, pageSize int, filters ...Filter) ([]Event, int, error) {
	skip := (page - 1) * pageSize
	var res []Event
	tx, err := b.db.Begin(false)
	if err != nil {
		return nil, 0, fmt.Errorf("could not begin datastore transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	bucket := tx.Bucket([]byte(logBucket))
	c := bucket.Cursor()
	for k, v := c.Last(); k != nil; k, v = c.Prev() {
		if skip > 0 {
			skip--
			continue
		}
		var row Event
		buf := bytes.NewReader(v)
		err = gob.NewDecoder(buf).Decode(&row)
		if err != nil {
			return nil, int(bucket.Sequence()), fmt.Errorf("could not decode log event: %w", err)
		}
		for _, f := range filters {
			if !f(row) {
				continue
			}
		}
		res = append(res, row)
		if len(res) == pageSize {
			return res, int(bucket.Sequence()), nil
		}
	}
	return res, int(bucket.Sequence()), nil
}
