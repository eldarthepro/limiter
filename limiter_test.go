package limiter

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestLimit(t *testing.T) {
	var (
		someIP = "1.1.1.1"
	)

	type limOpts struct {
		allowedIps      []string
		allowedPrefixes []string
		rps             int
		burst           int
	}

	tests := []struct {
		name           string
		ipHeaderValue  string
		opts           limOpts
		numReq         int
		expectedStatus int
		remoteAddr     string
	}{
		{
			name:          "allow_whitelisted_ip",
			ipHeaderValue: someIP,
			opts: limOpts{
				allowedIps: []string{someIP},
			},
			numReq:         1,
			expectedStatus: http.StatusOK,
		},
		{
			name:          "allow_prefix",
			ipHeaderValue: someIP,
			opts: limOpts{
				allowedPrefixes: []string{"1."},
			},
			numReq:         1,
			expectedStatus: http.StatusOK,
		},
		{
			name:          "allow_under_limit",
			ipHeaderValue: someIP,
			opts: limOpts{
				rps:   1,
				burst: 1,
			},
			numReq:         1,
			expectedStatus: http.StatusOK,
		},
		{
			name:          "reject_over_limit",
			ipHeaderValue: someIP,
			opts: limOpts{
				rps:   1,
				burst: 1,
			},
			numReq:         2,
			expectedStatus: http.StatusTooManyRequests,
		},
		{
			name:           "fallback_to_client_ip",
			ipHeaderValue:  "",
			expectedStatus: http.StatusOK,
			opts: limOpts{
				rps:   1,
				burst: 1,
			},
			numReq:     1,
			remoteAddr: "4.4.4.4:12345",
		},
		{
			name:          "csv_in_header",
			ipHeaderValue: "5.5.5.5, 6.6.6.6, 7.7.7.7",
			opts: limOpts{
				rps:   1,
				burst: 1,
			},
			numReq:         1,
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			l := New(AllowedIPs(tt.opts.allowedIps...),
				AllowedPrefixes(tt.opts.allowedPrefixes...),
				RpsWithBurst(tt.opts.rps, tt.opts.burst))

			mux := http.NewServeMux()
			mux.Handle("/test", Limit(l)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("OK"))
			})))

			server := httptest.NewServer(mux)
			defer server.Close()

			var rec *httptest.ResponseRecorder
			for i := 0; i < tt.numReq; i++ {
				req := httptest.NewRequest(http.MethodGet, server.URL+"/test", nil)
				rec = httptest.NewRecorder()

				if tt.ipHeaderValue != "" {
					req.Header.Set(XOFF, tt.ipHeaderValue)
				}

				if tt.remoteAddr != "" {
					req.RemoteAddr = tt.remoteAddr
				}

				mux.ServeHTTP(rec, req)
			}

			assert.Equal(t, tt.expectedStatus, rec.Code)
		})
	}
}

func TestGinLimit(t *testing.T) {
	var (
		someIP = "1.1.1.1"
	)

	type limOpts struct {
		allowedIps      []string
		allowedPrefixes []string
		rps             int
		burst           int
	}

	tests := []struct {
		name           string
		ipHeaderValue  string
		opts           limOpts
		numReq         int
		expectedStatus int
		remoteAddr     string
	}{
		{
			name:          "allow_whitelisted_ip",
			ipHeaderValue: someIP,
			opts: limOpts{
				allowedIps: []string{someIP},
			},
			numReq:         1,
			expectedStatus: http.StatusOK,
		},
		{
			name:          "allow_prefix",
			ipHeaderValue: someIP,
			opts: limOpts{
				allowedPrefixes: []string{"1."},
			},
			numReq:         1,
			expectedStatus: http.StatusOK,
		},
		{
			name:          "allow_under_limit",
			ipHeaderValue: someIP,
			opts: limOpts{
				rps:   1,
				burst: 1,
			},
			numReq:         1,
			expectedStatus: http.StatusOK,
		},
		{
			name:          "reject_over_limit",
			ipHeaderValue: someIP,
			opts: limOpts{
				rps:   1,
				burst: 1,
			},
			numReq:         2,
			expectedStatus: http.StatusTooManyRequests,
		},
		{
			name:           "fallback_to_client_ip",
			ipHeaderValue:  "",
			expectedStatus: http.StatusOK,
			opts: limOpts{
				rps:   1,
				burst: 1,
			},
			numReq:     1,
			remoteAddr: "4.4.4.4:12345",
		},
		{
			name:          "csv_in_header",
			ipHeaderValue: "5.5.5.5, 6.6.6.6, 7.7.7.7",
			opts: limOpts{
				rps:   1,
				burst: 1,
			},
			numReq:         1,
			expectedStatus: http.StatusOK,
		},
	}

	gin.SetMode(gin.TestMode)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			l := New(AllowedIPs(tt.opts.allowedIps...),
				AllowedPrefixes(tt.opts.allowedPrefixes...),
				RpsWithBurst(tt.opts.rps, tt.opts.burst))

			router := gin.New()
			router.Use(GinLimit(l))
			router.GET("/test", func(c *gin.Context) {
				c.String(http.StatusOK, "OK")
			})

			var rec *httptest.ResponseRecorder
			for i := 0; i < tt.numReq; i++ {
				req := httptest.NewRequest(http.MethodGet, "/test", nil)
				rec = httptest.NewRecorder()

				if tt.ipHeaderValue != "" {
					req.Header.Set("X-Forwarded-For", tt.ipHeaderValue)
				}

				if tt.remoteAddr != "" {
					req.RemoteAddr = tt.remoteAddr
				}

				router.ServeHTTP(rec, req)
			}

			assert.Equal(t, tt.expectedStatus, rec.Code)
		})
	}
}
