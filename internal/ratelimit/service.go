package ratelimit

import (
	"math"
	"net/http"
	"sync"
	"time"
)

// CircuitState represents the state of a circuit breaker.
type CircuitState int

const (
	StateClosed   CircuitState = iota // normal operation
	StateOpen                         // all requests rejected
	StateHalfOpen                     // probing recovery
)

func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half_open"
	default:
		return "unknown"
	}
}

// Config holds rate limiting and circuit breaker configuration.
type Config struct {
	// QPS is the max requests per second per developer.
	QPS int
	// CBWindow is the sliding window duration for error rate calculation.
	CBWindow time.Duration
	// CBErrorThreshold is the error rate (0.0-1.0) that triggers circuit opening.
	CBErrorThreshold float64
	// CBCooldown is how long the circuit stays open before transitioning to half-open.
	CBCooldown time.Duration
	// CBHalfOpenMax is how many probe requests are allowed in half-open state.
	CBHalfOpenMax int
}

// DefaultConfig returns a sensible default configuration.
func DefaultConfig() Config {
	return Config{
		QPS:              100,
		CBWindow:         30 * time.Second,
		CBErrorThreshold: 0.5,
		CBCooldown:       10 * time.Second,
		CBHalfOpenMax:    5,
	}
}

// bucket implements a token bucket rate limiter for a single key.
type bucket struct {
	tokens     float64
	lastRefill time.Time
}

// circuitBreaker tracks error rate for a single key.
type circuitBreaker struct {
	state       CircuitState
	lastChange  time.Time
	results     []result // sliding window of results
	halfOpenCnt int
}

type result struct {
	ts    time.Time
	isErr bool
}

// Service manages rate limiting and circuit breaking per developer key.
type Service struct {
	mu       sync.RWMutex
	cfg      Config
	buckets  map[int64]*bucket
	breakers map[int64]*circuitBreaker
}

// New creates a new Service.
func New(cfg Config) *Service {
	return &Service{
		cfg:      cfg,
		buckets:  make(map[int64]*bucket),
		breakers: make(map[int64]*circuitBreaker),
	}
}

// Allow checks if a request from the given key is allowed under rate limiting.
// Returns false if the rate limit is exceeded.
func (s *Service) Allow(key int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, ok := s.buckets[key]
	if !ok {
		b = &bucket{tokens: float64(s.cfg.QPS), lastRefill: time.Now()}
		s.buckets[key] = b
	}

	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens += elapsed * float64(s.cfg.QPS)
	if b.tokens > float64(s.cfg.QPS) {
		b.tokens = float64(s.cfg.QPS)
	}
	b.lastRefill = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// CircuitAllow checks if the circuit breaker for the given key allows the request.
// Returns the current circuit state and whether the request is allowed.
func (s *Service) CircuitAllow(key int64) (CircuitState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cb, ok := s.breakers[key]
	if !ok {
		cb = &circuitBreaker{state: StateClosed, lastChange: time.Now()}
		s.breakers[key] = cb
	}

	now := time.Now()

	switch cb.state {
	case StateClosed:
		// Prune old results outside the window.
		cutoff := now.Add(-s.cfg.CBWindow)
		pruned := make([]result, 0, len(cb.results))
		errCount := 0
		for _, r := range cb.results {
			if r.ts.After(cutoff) {
				pruned = append(pruned, r)
				if r.isErr {
					errCount++
				}
			}
		}
		cb.results = pruned

		// Check if we should open the circuit.
		total := len(cb.results)
		if total > 0 && float64(errCount)/float64(total) >= s.cfg.CBErrorThreshold {
			cb.state = StateOpen
			cb.lastChange = now
			return StateOpen, false
		}
		return StateClosed, true

	case StateOpen:
		if now.Sub(cb.lastChange) >= s.cfg.CBCooldown {
			cb.state = StateHalfOpen
			cb.lastChange = now
			cb.halfOpenCnt = 0
		} else {
			return StateOpen, false
		}
		fallthrough

	case StateHalfOpen:
		if cb.halfOpenCnt >= s.cfg.CBHalfOpenMax {
			return StateHalfOpen, false
		}
		cb.halfOpenCnt++
		return StateHalfOpen, true
	}

	return StateClosed, true
}

// RecordResult records the outcome of a request for circuit breaker tracking.
func (s *Service) RecordResult(key int64, statusCode int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cb, ok := s.breakers[key]
	if !ok {
		return
	}

	isErr := statusCode >= 500 || (statusCode >= 400 && statusCode != 429)
	r := result{ts: time.Now(), isErr: isErr}
	cb.results = append(cb.results, r)

	// Transition from half-open based on result.
	if cb.state == StateHalfOpen {
		if isErr {
			cb.state = StateOpen
			cb.lastChange = time.Now()
		} else {
			cb.state = StateClosed
			cb.lastChange = time.Now()
			cb.halfOpenCnt = 0
			// Reset results so old errors don't re-open the circuit.
			cb.results = []result{{ts: time.Now(), isErr: false}}
		}
	}

	// Cap results slice size to prevent unbounded growth.
	maxResults := int(s.cfg.CBWindow.Seconds())*2 + 100
	if len(cb.results) > maxResults {
		cb.results = cb.results[len(cb.results)-maxResults:]
	}
}

// CircuitState returns the current circuit state for a key.
func (s *Service) CircuitState(key int64) CircuitState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if cb, ok := s.breakers[key]; ok {
		return cb.state
	}
	return StateClosed
}

// ErrorRate returns the current error rate (0-1) for a key within the configured window.
func (s *Service) ErrorRate(key int64) float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cb, ok := s.breakers[key]
	if !ok {
		return 0
	}

	cutoff := time.Now().Add(-s.cfg.CBWindow)
	errCount, total := 0, 0
	for _, r := range cb.results {
		if r.ts.After(cutoff) {
			total++
			if r.isErr {
				errCount++
			}
		}
	}
	if total == 0 {
		return 0
	}
	return math.Round(float64(errCount)/float64(total)*100) / 100
}

// Status returns the rate limit and circuit breaker status for a key.
type Status struct {
	Key          int64   `json:"key"`
	CircuitState string  `json:"circuit_state"`
	ErrorRate    float64 `json:"error_rate"`
}

// GetStatus returns the current status for a key.
func (s *Service) GetStatus(key int64) Status {
	return Status{
		Key:          key,
		CircuitState: s.CircuitState(key).String(),
		ErrorRate:    s.ErrorRate(key),
	}
}

// statusResponseWriter wraps http.ResponseWriter to capture the status code.
type statusResponseWriter struct {
	http.ResponseWriter
	statusCode  int
	wroteHeader bool
}

func (w *statusResponseWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.statusCode = code
		w.wroteHeader = true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.statusCode = http.StatusOK
		w.wroteHeader = true
	}
	return w.ResponseWriter.Write(b)
}
