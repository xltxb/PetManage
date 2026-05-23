package monitor

import (
	"database/sql"
	"strconv"
	"sync"
	"time"
)

// LogEntry represents a single API request log entry.
type LogEntry struct {
	DeveloperID *int64
	Endpoint    string
	Method      string
	StatusCode  int
	DurationMs  int
	IPAddress   string
	RequestID   string
}

// Service handles API monitoring — logging requests and aggregating metrics.
type Service struct {
	db    *sql.DB
	logCh chan LogEntry
	wg    sync.WaitGroup
	done  chan struct{}
}

// NewService creates a monitoring service and starts the async log consumer.
func NewService(db *sql.DB) *Service {
	s := &Service{
		db:    db,
		logCh: make(chan LogEntry, 2000),
		done:  make(chan struct{}),
	}
	s.wg.Add(1)
	go s.consumeLogs()
	return s
}

func (s *Service) consumeLogs() {
	defer s.wg.Done()
	for {
		select {
		case entry := <-s.logCh:
			s.db.Exec(
				`INSERT INTO open_api_logs (developer_id, endpoint, method, status_code, duration_ms, ip_address, request_id, created_at)
				 VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())`,
				entry.DeveloperID, entry.Endpoint, entry.Method,
				entry.StatusCode, entry.DurationMs, entry.IPAddress, entry.RequestID,
			)
		case <-s.done:
			for {
				select {
				case entry := <-s.logCh:
					s.db.Exec(
						`INSERT INTO open_api_logs (developer_id, endpoint, method, status_code, duration_ms, ip_address, request_id, created_at)
						 VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())`,
						entry.DeveloperID, entry.Endpoint, entry.Method,
						entry.StatusCode, entry.DurationMs, entry.IPAddress, entry.RequestID,
					)
				default:
					return
				}
			}
		}
	}
}

// LogAsync enqueues a log entry for async insertion. Drops if the channel is full.
func (s *Service) LogAsync(entry LogEntry) {
	select {
	case s.logCh <- entry:
	default:
	}
}

// Close shuts down the log consumer goroutine, draining remaining entries.
func (s *Service) Close() {
	close(s.done)
	s.wg.Wait()
}

// EndpointMetric holds aggregated metrics for a single API endpoint.
type EndpointMetric struct {
	Endpoint    string  `json:"endpoint"`
	Method      string  `json:"method"`
	CallCount   int64   `json:"call_count"`
	SuccessRate float64 `json:"success_rate"`
	ErrorRate   float64 `json:"error_rate"`
	P95Latency  float64 `json:"p95_latency_ms"`
	AvgLatency  float64 `json:"avg_latency_ms"`
}

// DeveloperMetric holds aggregated metrics grouped by developer.
type DeveloperMetric struct {
	DeveloperID int64   `json:"developer_id"`
	CompanyName string  `json:"company_name"`
	CallCount   int64   `json:"call_count"`
	SuccessRate float64 `json:"success_rate"`
	ErrorRate   float64 `json:"error_rate"`
	P95Latency  float64 `json:"p95_latency_ms"`
	AvgLatency  float64 `json:"avg_latency_ms"`
}

func computeSince(period string) time.Time {
	now := time.Now()
	switch period {
	case "1h":
		return now.Add(-1 * time.Hour)
	case "24h":
		return now.Add(-24 * time.Hour)
	case "7d":
		return now.Add(-7 * 24 * time.Hour)
	default:
		return now.Add(-24 * time.Hour)
	}
}

// GetEndpointMetrics returns per-endpoint aggregated metrics for the given time period.
func (s *Service) GetEndpointMetrics(period, keyword, sortBy, sortDir string) ([]EndpointMetric, error) {
	since := computeSince(period)

	query := `
		SELECT endpoint, method,
			COUNT(*)::bigint AS call_count,
			ROUND(COUNT(*) FILTER (WHERE status_code < 400) * 100.0 / NULLIF(COUNT(*), 0), 1) AS success_rate,
			ROUND(COUNT(*) FILTER (WHERE status_code >= 400) * 100.0 / NULLIF(COUNT(*), 0), 1) AS error_rate,
			COALESCE(ROUND(percentile_cont(0.95) WITHIN GROUP (ORDER BY duration_ms)::numeric, 1), 0) AS p95_latency,
			COALESCE(ROUND(AVG(duration_ms)::numeric, 1), 0) AS avg_latency
		FROM open_api_logs
		WHERE created_at >= $1`

	args := []interface{}{since}
	argIdx := 2

	if keyword != "" {
		query += ` AND endpoint ILIKE $` + itoa(argIdx)
		args = append(args, "%"+keyword+"%")
		argIdx++
	}

	query += ` GROUP BY endpoint, method`

	orderCol := "call_count"
	switch sortBy {
	case "success_rate":
		orderCol = "success_rate"
	case "error_rate":
		orderCol = "error_rate"
	case "p95_latency_ms":
		orderCol = "p95_latency"
	case "avg_latency_ms":
		orderCol = "avg_latency"
	}

	orderDir := "DESC"
	if sortDir == "asc" {
		orderDir = "ASC"
	}
	query += ` ORDER BY ` + orderCol + ` ` + orderDir
	query += ` LIMIT 100`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metrics []EndpointMetric
	for rows.Next() {
		var m EndpointMetric
		if err := rows.Scan(&m.Endpoint, &m.Method, &m.CallCount, &m.SuccessRate, &m.ErrorRate, &m.P95Latency, &m.AvgLatency); err != nil {
			return nil, err
		}
		metrics = append(metrics, m)
	}
	return metrics, rows.Err()
}

// GetDeveloperMetrics returns per-developer aggregated metrics for the given time period.
func (s *Service) GetDeveloperMetrics(period string) ([]DeveloperMetric, error) {
	since := computeSince(period)

	query := `
		SELECT d.id, d.company_name,
			COUNT(*)::bigint AS call_count,
			ROUND(COUNT(*) FILTER (WHERE l.status_code < 400) * 100.0 / NULLIF(COUNT(*), 0), 1) AS success_rate,
			ROUND(COUNT(*) FILTER (WHERE l.status_code >= 400) * 100.0 / NULLIF(COUNT(*), 0), 1) AS error_rate,
			COALESCE(ROUND(percentile_cont(0.95) WITHIN GROUP (ORDER BY l.duration_ms)::numeric, 1), 0) AS p95_latency,
			COALESCE(ROUND(AVG(l.duration_ms)::numeric, 1), 0) AS avg_latency
		FROM open_api_logs l
		JOIN open_developers d ON l.developer_id = d.id
		WHERE l.created_at >= $1 AND l.developer_id IS NOT NULL
		GROUP BY d.id, d.company_name
		ORDER BY call_count DESC
		LIMIT 100`

	rows, err := s.db.Query(query, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metrics []DeveloperMetric
	for rows.Next() {
		var m DeveloperMetric
		if err := rows.Scan(&m.DeveloperID, &m.CompanyName, &m.CallCount, &m.SuccessRate, &m.ErrorRate, &m.P95Latency, &m.AvgLatency); err != nil {
			return nil, err
		}
		metrics = append(metrics, m)
	}
	return metrics, rows.Err()
}

// AnomalyThreshold is the error rate above which an endpoint is flagged.
const AnomalyThreshold = 10.0

// GetAnomalies returns endpoints whose error rate exceeds the threshold.
func (s *Service) GetAnomalies(period string) ([]EndpointMetric, error) {
	since := computeSince(period)

	query := `
		SELECT endpoint, method,
			COUNT(*)::bigint AS call_count,
			ROUND(COUNT(*) FILTER (WHERE status_code < 400) * 100.0 / NULLIF(COUNT(*), 0), 1) AS success_rate,
			ROUND(COUNT(*) FILTER (WHERE status_code >= 400) * 100.0 / NULLIF(COUNT(*), 0), 1) AS error_rate,
			COALESCE(ROUND(percentile_cont(0.95) WITHIN GROUP (ORDER BY duration_ms)::numeric, 1), 0) AS p95_latency,
			COALESCE(ROUND(AVG(duration_ms)::numeric, 1), 0) AS avg_latency
		FROM open_api_logs
		WHERE created_at >= $1
		GROUP BY endpoint, method
		HAVING COUNT(*) FILTER (WHERE status_code >= 400) * 100.0 / NULLIF(COUNT(*), 0) > $2
		ORDER BY error_rate DESC
		LIMIT 50`

	rows, err := s.db.Query(query, since, AnomalyThreshold)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metrics []EndpointMetric
	for rows.Next() {
		var m EndpointMetric
		if err := rows.Scan(&m.Endpoint, &m.Method, &m.CallCount, &m.SuccessRate, &m.ErrorRate, &m.P95Latency, &m.AvgLatency); err != nil {
			return nil, err
		}
		metrics = append(metrics, m)
	}
	return metrics, rows.Err()
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
