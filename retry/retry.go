package retry

import (
	"context"
	"strconv"
	"time"
)

type Config struct {
	Attempts int    `json:"attempts" yaml:"attempts"`
	Backoff  string `json:"backoff" yaml:"backoff"`
}

type Policy struct {
	Attempts int
	Backoff  time.Duration
}

func NewPolicy(cfg Config) Policy {
	p := Policy{Attempts: 1, Backoff: 100 * time.Millisecond}
	if cfg.Attempts > 0 {
		p.Attempts = cfg.Attempts
	}
	if cfg.Backoff != "" {
		if d, err := time.ParseDuration(cfg.Backoff); err == nil && d > 0 {
			p.Backoff = d
		}
	}
	return p
}

func NewPolicyFromStrings(attempts, backoff string) Policy {
	cfg := Config{Backoff: backoff}
	if attempts != "" {
		if n, err := strconv.Atoi(attempts); err == nil {
			cfg.Attempts = n
		}
	}
	return NewPolicy(cfg)
}

func Do(ctx context.Context, p Policy, fn func() error) error {
	if p.Attempts <= 0 {
		p.Attempts = 1
	}
	var err error
	for i := 0; i < p.Attempts; i++ {
		if ctx != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}
		if err = fn(); err == nil {
			return nil
		}
		if i == p.Attempts-1 || p.Backoff <= 0 {
			continue
		}
		t := time.NewTimer(p.Backoff)
		select {
		case <-t.C:
		case <-ctx.Done():
			if !t.Stop() {
				<-t.C
			}
			return ctx.Err()
		}
	}
	return err
}
