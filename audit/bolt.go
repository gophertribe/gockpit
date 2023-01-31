package audit

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/gob"
	"fmt"
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

func init() {
	gob.Register(Event{})
	gob.Register(MsgClearError{})
}

type MsgClearError struct{}

type Publisher interface {
	Publish(ctx context.Context, msg interface{}) error
}

type Logger interface {
	Error(ctx context.Context, ns, code string, payload interface{})
	Info(ctx context.Context, ns, event string, payload interface{})
	GetPage(page, pageSize int, filters ...Filter) ([]Event, int, error)
	SetError(ctx context.Context, ns, code string, err error)
	ClearError(ctx context.Context, ns, code string, err error)
}

type StdLogger interface {
	Infof(string, ...interface{})
}

type Bolt struct {
	mx sync.Mutex
	path   string
	pub    Publisher
	logger StdLogger
	db     *bbolt.DB
}

func NewBolt(path string, pub Publisher, logger StdLogger) (*Bolt, error) {
	b := &Bolt{
		path:   path,
		pub:    pub,
		logger: logger,
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
	return b.pub.Publish(ctx, l)
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
