package limiter

import (
	"net"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"golang.org/x/time/rate"
)

// Limit attempts to extract ip using header from options,
// if fails, uses http RemoreAddr(). If limit is reached,
// will respond with http 429 and "Too many requests" message
func Limit(l Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.Header.Get(l.ipHeader())

			if ip != "" {
				ip = strings.Split(ip, ",")[0]
				ip = strings.TrimSpace(ip)
			}

			if ip == "" {
				host, _, err := net.SplitHostPort(r.RemoteAddr)
				if err == nil {
					ip = host
				} else {
					ip = r.RemoteAddr
				}
			}

			if l.whiteListed(ip) {
				next.ServeHTTP(w, r)
				return
			}

			if !l.visitor(ip).Allow() {
				http.Error(w, tooManyReqMsg, http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// GinLimit attempts to extract ip using header from options,
// if fails, uses gin clientIP(). If limit is reached,
// will respond with http 429 and "Too many requests" message
func GinLimit(l Limiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.GetHeader(l.ipHeader())

		if ip != "" {
			ip = strings.Split(ip, ",")[0]
			ip = strings.TrimSpace(ip)
		}

		if ip == "" {
			ip = c.ClientIP()
		}

		if l.whiteListed(ip) {
			c.Next()
		}

		if !l.visitor(ip).Allow() {
			//	logger.Error(fmt.Sprintf(notAllowedFmt, ip, c.GetString(domain.XFwdForHeader), ip, c.ClientIP()))
			c.String(http.StatusTooManyRequests, tooManyReqMsg)
			c.Abort()
			return
		}

		c.Next()
	}
}

// Returns new instance of ratelimiter. If no opts are provided uses default settings. RPS = 10, BURST = 20, ttl and cleanup 5 minutes.
func New(opts ...option) Limiter {
	o := defautlOptions()

	for _, opt := range opts {
		opt(o)
	}

	lim := &limiter{
		storage: make(map[string]*record),
		opts:    o,
		stop:    make(chan struct{}),
		limit:   rate.Limit(float64(o.requests) / o.period.Seconds()),
	}

	go lim.scheduleCleanup()

	return lim
}

// visitor lloks up entry in storage and returns *rate.Limiter, updating lastSeen field. Doesnt check if string is empty, so will return same updated limiter for all empty ip visitors.
func (lim *limiter) visitor(ip string) *rate.Limiter {
	lim.RLock()
	v, e := lim.storage[ip]
	lim.RUnlock()

	if !e {
		l := rate.NewLimiter(lim.limit, lim.opts.burst)

		lim.Lock()
		lim.storage[ip] = &record{
			lastSeen: time.Now(),
			limiter:  l,
		}
		lim.Unlock()

		return l
	}

	if v != nil {
		lim.Lock()
		v.lastSeen = time.Now()
		lim.Unlock()
	}

	return v.limiter
}

func defautlOptions() *limiterOptions {
	return &limiterOptions{
		ttl:           defaultTTL,
		requests:      defaultRps,
		burst:         defaultBurst,
		period:        defaultPeriod,
		cleanupFreq:   defaultCleanupFrequency,
		ipHeader:      XOFF,
		allowedPrefix: []string{},
		allowedIPs:    []string{},
	}
}

// RpsWithBurst sets custom rps and burst values. If burst is zero, all events will be blocked, unless rps is inf. If burst < rps requests will be limited by burst.
func RpsWithBurst(rps, burst int) option {
	if rps < 0 {
		rps = defaultRps
	}

	if burst < 0 {
		burst = defaultRps
	}

	return func(opts *limiterOptions) {
		opts.requests = rps
		opts.burst = burst
	}
}

// Rps sets allowed rps, burst will be disabled
func Rps(rps int) option {
	if rps < 0 {
		rps = defaultRps
	}

	return func(opts *limiterOptions) {
		opts.requests = rps
		opts.burst = rps
	}
}

func Burst(burst int) option {
	if burst < 0 {
		burst = defaultBurst
	}

	return func(opts *limiterOptions) {
		opts.burst = burst
	}
}

// Period sets allowed period when rps is smaller than 1. For example 1 request per 5 seconds. In most cases set burst to 1.
func Period(requests int, period time.Duration) option {
	if period < 0 {
		period = defaultPeriod
	}
	if requests < 0 {
		requests = 0
	}

	return func(opts *limiterOptions) {
		opts.period = period
		opts.customPeriod = true
		opts.requests = requests
	}
}

// CleanupFrequency sets how often to cleanup storage.
func CleanupFrequency(cf time.Duration) option {
	if cf <= 0 {
		cf = defaultCleanupFrequency
	}

	return func(opts *limiterOptions) {
		opts.cleanupFreq = cf
	}
}

// RecordTTL sets lifetime of every record, before it is expired.
func RecordTTL(ttl time.Duration) option {
	if ttl < 0 {
		ttl = defaultTTL
	}

	return func(opts *limiterOptions) {
		opts.ttl = ttl
	}
}

// Allowed prefixes takes strings with ips (requester ip will be checked for equality) that will not be ratelimited.
func AllowedIPs(ip ...string) option {
	return func(opts *limiterOptions) {
		opts.allowedIPs = append(opts.allowedIPs, ip...)
	}
}

// Allowed prefixes takes strings with ip prefixes that will not be ratelimited.
func AllowedPrefixes(prefix ...string) option {
	return func(opts *limiterOptions) {
		opts.allowedPrefix = append(opts.allowedPrefix, prefix...)
	}
}

func (lim *limiter) IPHeader(h string) option {
	return func(opts *limiterOptions) {
		opts.ipHeader = h
	}
}

// Stop stops cleanup routine in limiter
func (lim *limiter) Stop() {
	close(lim.stop)
}

func (lim *limiter) scheduleCleanup() {
	ti := time.NewTicker(lim.opts.cleanupFreq)
	defer ti.Stop()

	for {
		select {
		case <-ti.C:
			lim.cleanup()
		case <-lim.stop:
			return
		}
	}
}

func (lim *limiter) cleanup() {
	exp := make([]string, len(lim.storage)>>1)

	lim.RLock()
	for k, v := range lim.storage {
		if v == nil {
			exp = append(exp, k)
		}

		if v == nil || time.Since(v.lastSeen) >= lim.opts.ttl {
			exp = append(exp, k)
		}
	}
	lim.RUnlock()

	lim.Lock()
	for _, k := range exp {
		delete(lim.storage, k)
	}
	lim.Unlock()
}

func (lim *limiter) whiteListed(ip string) bool {
	return slices.Contains(lim.opts.allowedIPs, ip) || lim.hasWhitelistedPrefix(ip)
}

func (lim *limiter) hasWhitelistedPrefix(ip string) bool {
	for _, v := range lim.opts.allowedPrefix {
		if strings.HasPrefix(ip, v) {
			return true
		}
	}

	return false
}

func (lim *limiter) ipHeader() string {
	return lim.opts.ipHeader
}
