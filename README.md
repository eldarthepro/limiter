## IP Based Rate Limiter for http server.
The `limiter` package provides various customization options to configure request rate limiting behavior. These options allow fine-tuning of request per second (RPS), burst limits, cleanup behavior, and whitelisted IPs or prefixes.

Example:
```
package main

import (
	"net/http"
	"github.com/eldarthepro/limiter"
)

func main() {
	l := limiter.New()

	mux := http.NewServeMux()
	mux.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	server := &http.Server{
		Addr:    ":8080",
		Handler: limiter.Limit(l)(mux),
	}

	server.ListenAndServe()
}

```
Example with gin:
```
package main

import (
	"github.com/gin-gonic/gin"
	"github.com/eldarthepro/limiter"
)

func main() {
	l := limiter.New()

	router := gin.Default()
	router.Use(limiter.GinLimit(l))

	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	router.Run(":8080")
}
```


## Custom Parameterization in Limiter

### RPS and Burst Configuration

  - Sets custom RPS and burst values.
  - If `burst == 0`, all requests are blocked unless `rps == ∞`.
  - If `burst < rps`, requests are limited by the burst value.

  ```
  limiter := limiter.New(limiter.RpsWithBurst(5, 10))
  ```
  - Sets the allowed requests per second.
  - Burst is set equal to RPS.

  ```
  limiter := limiter.New(limiter.Rps(10))
  ```

  - Configures the burst limit separately from RPS.

  ```
  limiter := limiter.New(limiter.Burst(15))
  ```

### Period-based Rate Limiting
  - Allows defining request limits over a custom time period.
  - Useful for scenarios like "1 request per 5 seconds".
  ```
  limiter := limiter.New(limiter.Period(1, 5*time.Second))
  ```
### Cleanup and Expiry Settings

  - Defines how often expired records are cleaned up.

  ```
  limiter := limiter.New(limiter.CleanupFrequency(time.Minute * 2))
  ```

  - Sets the expiration time for request records.

  ```
  limiter := limiter.New(limiter.RecordTTL(time.Minute * 10))
  ```

### IP Whitelisting

  - Whitelists specific IPs from being rate-limited.

  ```
  limiter := limiter.New(limiter.AllowedIPs("192.168.1.1", "10.0.0.2"))
  ```
  - Whitelists entire IP ranges by prefix.

  ```
  limiter := limiter.New(limiter.AllowedPrefixes("192.168.1."))
  ```

### Header-Based IP Detection
  - Defines a custom header to extract the client's IP address.
  ```
  limiter := limiter.New(limiter.IPHeader("X-Real-IP"))
  ```

### Stopping the Limiter
  - Stops the cleanup routine gracefully.
  ```
  limiter.Stop()
  ```

### Default Configuration Values

By default, the limiter uses the following settings:

| Parameter      | Default Value             |
| -------------- | ------------------------- |
| RPS            | 10 requests/second        |
| Burst          | 20 requests               |
| Record TTL     | 5 minutes                 |
| Cleanup Period | 5 minutes                 |
| Default Period | 1 second                  |
| Header         | "x-original-forwarded-for"|

You can override these values by passing custom options during initialization.

