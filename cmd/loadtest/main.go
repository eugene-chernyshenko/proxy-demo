package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"time"

	"golang.org/x/net/proxy"
)

// Metrics хранит метрики одного запроса
type Metrics struct {
	TTFB         time.Duration
	DNSResolve   time.Duration
	Connect      time.Duration
	TLSHandshake time.Duration
	TotalLatency time.Duration
	Success      bool
	Error        string
	StatusCode   int
	BytesRead    int64
}

// Result представляет результат одного запроса
type Result struct {
	Metrics Metrics
	URL     string
}

// Stats собирает статистику всех запросов
type Stats struct {
	mu              sync.Mutex
	Total           int
	Success         int
	Errors          map[string]int
	StatusCodes     map[int]int
	TTFB            []time.Duration
	DNSResolve      []time.Duration
	Connect         []time.Duration
	TLSHandshake    []time.Duration
	TotalLatency    []time.Duration
	TotalBytes      int64
}

// NewStats создает новую структуру статистики
func NewStats() *Stats {
	return &Stats{
		Errors:      make(map[string]int),
		StatusCodes: make(map[int]int),
	}
}

// Add добавляет результат запроса в статистику
func (s *Stats) Add(result Result) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Total++
	if result.Metrics.Success {
		s.Success++
		s.StatusCodes[result.Metrics.StatusCode]++
	} else {
		s.Errors[result.Metrics.Error]++
	}

	if result.Metrics.TTFB > 0 {
		s.TTFB = append(s.TTFB, result.Metrics.TTFB)
	}
	if result.Metrics.DNSResolve > 0 {
		s.DNSResolve = append(s.DNSResolve, result.Metrics.DNSResolve)
	}
	if result.Metrics.Connect > 0 {
		s.Connect = append(s.Connect, result.Metrics.Connect)
	}
	if result.Metrics.TLSHandshake > 0 {
		s.TLSHandshake = append(s.TLSHandshake, result.Metrics.TLSHandshake)
	}
	if result.Metrics.TotalLatency > 0 {
		s.TotalLatency = append(s.TotalLatency, result.Metrics.TotalLatency)
	}
	s.TotalBytes += result.Metrics.BytesRead
}

// Percentile вычисляет перцентиль для слайса длительностей
func (s *Stats) Percentile(durations []time.Duration, p float64) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})
	index := int(float64(len(sorted)) * p / 100.0)
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}

// measuredDialer оборачивает proxy.Dialer для измерения времени Connect
type measuredDialer struct {
	dialer proxy.Dialer
}

func (m *measuredDialer) Dial(network, addr string) (net.Conn, error) {
	connectStart := time.Now()
	conn, err := m.dialer.Dial(network, addr)
	connectEnd := time.Now()
	if err != nil {
		return nil, err
	}

	// Сохраняем время Connect в контексте соединения
	return &measuredConn{
		Conn:         conn,
		connectTime:  connectEnd.Sub(connectStart),
		connectStart: connectStart,
	}, nil
}

// measuredConn оборачивает net.Conn для измерения TLS handshake
type measuredConn struct {
	net.Conn
	connectTime  time.Duration
	connectStart time.Time
	tlsStart     time.Time
	tlsEnd       time.Time
}

func (m *measuredConn) getConnectTime() time.Duration {
	return m.connectTime
}

func (m *measuredConn) getTLSHandshakeTime() time.Duration {
	if m.tlsStart.IsZero() || m.tlsEnd.IsZero() {
		return 0
	}
	return m.tlsEnd.Sub(m.tlsStart)
}

// makeRequest выполняет HTTP запрос и измеряет все метрики
func makeRequest(client *http.Client, targetURL string) Result {
	start := time.Now()
	result := Result{
		URL: targetURL,
		Metrics: Metrics{
			Success: false,
		},
	}

	// Parse URL for DNS resolve
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		result.Metrics.Error = fmt.Sprintf("invalid URL: %v", err)
		result.Metrics.TotalLatency = time.Since(start)
		return result
	}

	host := parsedURL.Hostname()

	// DNS Resolve
	dnsStart := time.Now()
	_, err = net.LookupIP(host)
	dnsEnd := time.Now()
	if err == nil {
		result.Metrics.DNSResolve = dnsEnd.Sub(dnsStart)
	}

	// HTTP Request
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		result.Metrics.Error = fmt.Sprintf("failed to create request: %v", err)
		result.Metrics.TotalLatency = time.Since(start)
		return result
	}

	// Create context with placeholders for connect and TLS times
	var connectTime time.Duration
	var tlsTime time.Duration
	ctx := context.WithValue(req.Context(), "connectTime", &connectTime)
	ctx = context.WithValue(ctx, "tlsTime", &tlsTime)
	req = req.WithContext(ctx)

	resp, err := client.Do(req)
	if err != nil {
		result.Metrics.Error = err.Error()
		result.Metrics.TotalLatency = time.Since(start)
		return result
	}
	defer resp.Body.Close()

	// Read first byte (TTFB)
	firstByte := make([]byte, 1)
	ttfbStart := time.Now()
	_, err = resp.Body.Read(firstByte)
	ttfbEnd := time.Now()
	if err == nil || err == io.EOF {
		result.Metrics.TTFB = ttfbEnd.Sub(ttfbStart) + (ttfbStart.Sub(start))
	}

	// Read rest of body and count bytes
	bytesRead, _ := io.Copy(io.Discard, resp.Body)
	result.Metrics.BytesRead = bytesRead + 1 // +1 for first byte

	result.Metrics.Success = true
	result.Metrics.StatusCode = resp.StatusCode
	result.Metrics.TotalLatency = time.Since(start)

	// Get Connect and TLS times from context
	if ct := req.Context().Value("connectTime"); ct != nil {
		if ctPtr, ok := ct.(*time.Duration); ok && ctPtr != nil {
			result.Metrics.Connect = *ctPtr
		}
	}
	if tt := req.Context().Value("tlsTime"); tt != nil {
		if ttPtr, ok := tt.(*time.Duration); ok && ttPtr != nil {
			result.Metrics.TLSHandshake = *ttPtr
		}
	}

	return result
}

// createHTTPClient создает HTTP клиент с SOCKS5 прокси и измерением метрик
func createHTTPClient(proxyAddr string) (*http.Client, error) {
	var dialer proxy.Dialer
	var err error

	if proxyAddr != "" {
		// Create SOCKS5 dialer
		dialer, err = proxy.SOCKS5("tcp", proxyAddr, nil, proxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("failed to create SOCKS5 dialer: %w", err)
		}
	} else {
		dialer = proxy.Direct
	}

	// Wrap dialer to measure Connect time
	measuredDialer := &measuredDialer{
		dialer: dialer,
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			conn, err := measuredDialer.Dial(network, addr)
			if err != nil {
				return nil, err
			}
			// Store connect time in context if available
			if mc, ok := conn.(*measuredConn); ok {
				connectTime := mc.getConnectTime()
				if ctPtr := ctx.Value("connectTime"); ctPtr != nil {
					if ct, ok := ctPtr.(*time.Duration); ok {
						*ct = connectTime
					}
				}
			}
			return conn, nil
		},
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
		},
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	return client, nil
}

// runLoadTest запускает нагрузочный тест
func runLoadTest(proxyAddr, targetURL string, concurrency, requests int) {
	stats := NewStats()
	client, err := createHTTPClient(proxyAddr)
	if err != nil {
		fmt.Printf("Error creating HTTP client: %v\n", err)
		return
	}

	semaphore := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	fmt.Printf("Starting load test: %d requests, %d concurrent\n", requests, concurrency)
	fmt.Printf("Target: %s\n", targetURL)
	if proxyAddr != "" {
		fmt.Printf("Proxy: %s\n", proxyAddr)
	}
	fmt.Println()

	startTime := time.Now()
	for i := 0; i < requests; i++ {
		wg.Add(1)
		semaphore <- struct{}{} // Acquire

		go func() {
			defer wg.Done()
			defer func() { <-semaphore }() // Release

			result := makeRequest(client, targetURL)
			stats.Add(result)
		}()
	}

	wg.Wait()
	duration := time.Since(startTime)

	// Print report
	printReport(stats, duration)
}

// printReport выводит отчет в терминал
func printReport(stats *Stats, duration time.Duration) {
	stats.mu.Lock()
	defer stats.mu.Unlock()

	fmt.Println("==================================================================================")
	fmt.Println("LOAD TEST REPORT")
	fmt.Println("==================================================================================")
	fmt.Println()

	// Success Rate
	successRate := float64(stats.Success) / float64(stats.Total) * 100
	fmt.Printf("Success Rate\n")
	fmt.Println("+-------+-------+---------+")
	fmt.Printf("| VALUE | COUNT | PERCENT |\n")
	fmt.Println("+-------+-------+---------+")
	fmt.Printf("| ok    | %5d | %6.2f  |\n", stats.Success, successRate)
	if stats.Total-stats.Success > 0 {
		fmt.Printf("| error | %5d | %6.2f  |\n", stats.Total-stats.Success, 100-successRate)
	}
	fmt.Println("+-------+-------+---------+")
	fmt.Println()

	// Errors breakdown
	if len(stats.Errors) > 0 {
		fmt.Printf("Errors\n")
		fmt.Println("+---------------------------------+-------+---------+")
		fmt.Printf("| VALUE                           | COUNT | PERCENT |\n")
		fmt.Println("+---------------------------------+-------+---------+")
		for err, count := range stats.Errors {
			percent := float64(count) / float64(stats.Total) * 100
			errStr := err
			if len(errStr) > 31 {
				errStr = errStr[:28] + "..."
			}
			fmt.Printf("| %-31s | %5d | %6.2f  |\n", errStr, count, percent)
		}
		fmt.Println("+---------------------------------+-------+---------+")
		fmt.Println()
	}

	// HTTP Status Codes
	if len(stats.StatusCodes) > 0 {
		fmt.Printf("Target HTTP status codes\n")
		fmt.Println("+-------+-------+---------+")
		fmt.Printf("| VALUE | COUNT | PERCENT |\n")
		fmt.Println("+-------+-------+---------+")
		for code, count := range stats.StatusCodes {
			percent := float64(count) / float64(stats.Total) * 100
			fmt.Printf("| %5d | %5d | %6.2f  |\n", code, count, percent)
		}
		fmt.Println("+-------+-------+---------+")
		fmt.Println()
	}

	// Latency metrics
	fmt.Printf("Latency\n")
	fmt.Println("+--------------+-------+-------+-------+-------+-------+-------+-------+")
	fmt.Printf("| NAME         |   50  |   75  |   85  |   90  |   95  |   99  |  100  |\n")
	fmt.Println("+--------------+-------+-------+-------+-------+-------+-------+-------+")

	printLatencyRow("TTFB", stats.TTFB, stats)
	printLatencyRow("DNS resolve", stats.DNSResolve, stats)
	printLatencyRow("Connect", stats.Connect, stats)
	printLatencyRow("TLSHandshake", stats.TLSHandshake, stats)
	fmt.Println("+--------------+-------+-------+-------+-------+-------+-------+-------+")
	fmt.Println()

	// Summary
	fmt.Printf("Summary\n")
	fmt.Printf("Total duration: %v\n", duration)
	fmt.Printf("Requests/sec: %.2f\n", float64(stats.Total)/duration.Seconds())
	if stats.TotalBytes > 0 {
		fmt.Printf("Throughput: %.2f KB/s\n", float64(stats.TotalBytes)/1024/duration.Seconds())
	}
}

// printLatencyRow выводит строку с перцентилями для метрики
func printLatencyRow(name string, durations []time.Duration, stats *Stats) {
	if len(durations) == 0 {
		fmt.Printf("| %-12s | %5s | %5s | %5s | %5s | %5s | %5s | %5s |\n", name, "-", "-", "-", "-", "-", "-", "-")
		return
	}

	p50 := stats.Percentile(durations, 50)
	p75 := stats.Percentile(durations, 75)
	p85 := stats.Percentile(durations, 85)
	p90 := stats.Percentile(durations, 90)
	p95 := stats.Percentile(durations, 95)
	p99 := stats.Percentile(durations, 99)
	p100 := durations[len(durations)-1]

	fmt.Printf("| %-12s | %5d | %5d | %5d | %5d | %5d | %5d | %5d |\n",
		name,
		int(p50.Milliseconds()),
		int(p75.Milliseconds()),
		int(p85.Milliseconds()),
		int(p90.Milliseconds()),
		int(p95.Milliseconds()),
		int(p99.Milliseconds()),
		int(p100.Milliseconds()))
}

func main() {
	var (
		proxyAddr  = flag.String("proxy", "127.0.0.1:1080", "SOCKS5 proxy address")
		targetURL  = flag.String("url", "http://httpbin.org/get", "Target URL")
		concurrency = flag.Int("c", 10, "Number of concurrent requests")
		requests   = flag.Int("n", 100, "Total number of requests")
	)
	flag.Parse()

	runLoadTest(*proxyAddr, *targetURL, *concurrency, *requests)
}
