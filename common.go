package limiter

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	defaultRps              = 10
	defaultBurst            = 20
	defaultTTL              = time.Minute * 5
	defaultCleanupFrequency = time.Minute * 5
	defaultPeriod           = time.Second
)

const (
	localhost     = "::1"
	notAllowedFmt = "ratelimited for ip: %s, FwdFor: %s, XOrigFwdFor: %s, clientIP: %s"
	XFF           = "x-forwarded-for"
	XOFF          = "x-original-forwarded-for"
	tooManyReqMsg = "Too many requests"
)

type (
	Limiter interface {
		Stop()
		visitor(string) *rate.Limiter
		whiteListed(string) bool
		ipHeader() string
	}

	limiter struct {
		storage map[string]*record
		opts    *limiterOptions
		stop    chan struct{}
		limit   rate.Limit
		sync.RWMutex
	}

	record struct {
		lastSeen time.Time
		limiter  *rate.Limiter
	}

	limiterOptions struct {
		ttl           time.Duration
		customPeriod  bool
		period        time.Duration
		burst         int
		requests      int
		cleanupFreq   time.Duration
		ipHeader      string
		allowedPrefix []string
		allowedIPs    map[string]struct{}
	}

	option func(*limiterOptions)
)
