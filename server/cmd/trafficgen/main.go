// Command trafficgen generates live traffic through the Ingestion API.
// It sends events via HTTP POST to exercise the full pipeline:
// Ingestion API -> NATS -> Worker -> ClickHouse.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"math"
	"math/rand/v2"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/promptmeter/promptmeter/server/internal/datagen"
)

func main() {
	var (
		rps       = flag.Float64("rps", 2.0, "Requests per second")
		scenario  = flag.String("scenario", "normal", "Scenario: normal, spike, anomaly")
		duration  = flag.Duration("duration", 0, "Duration (0 = infinite)")
		apiURL    = flag.String("api-url", "", "Ingestion API URL (default: env PM_API_URL or http://localhost:8443)")
		apiKey    = flag.String("api-key", "", "API key (default: env PM_API_KEY)")
		batchSize = flag.Int("batch-size", 10, "Events per batch request")
	)
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if *apiURL == "" {
		*apiURL = os.Getenv("PM_API_URL")
	}
	if *apiURL == "" {
		*apiURL = "http://localhost:8443"
	}
	if *apiKey == "" {
		*apiKey = os.Getenv("PM_API_KEY")
	}
	if *apiKey == "" {
		*apiKey = "pm_test_SeedDemoKeyForDev000000000000000"
	}

	st := datagen.ScenarioType(*scenario)
	sc := datagen.NewScenario(st)

	// Build a minimal price table for cost estimation in stats.
	pt := datagen.BuildPriceTable(datagen.SeedModelPrices)
	projects := datagen.DefaultProjectDistributions
	gen := datagen.NewGenerator(time.Now().UnixNano(), 1, pt, projects)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		logger.Info("shutdown signal received, finishing current batch...")
		cancel()
	}()

	if *duration > 0 {
		var dcancel context.CancelFunc
		ctx, dcancel = context.WithTimeout(ctx, *duration)
		defer dcancel()
	}

	client := &http.Client{Timeout: 30 * time.Second}
	endpoint := *apiURL + "/v1/events/batch"

	logger.Info("trafficgen starting",
		"rps", *rps,
		"scenario", *scenario,
		"duration", *duration,
		"api_url", *apiURL,
		"batch_size", *batchSize,
	)

	// Statistics counters.
	var (
		sentCount  atomic.Int64
		okCount    atomic.Int64
		errCount   atomic.Int64
		err429     atomic.Int64
		totalCost  atomic.Int64 // stored as micro-dollars
		modelCount sync.Map
	)

	// Stats printer goroutine.
	startTime := time.Now()
	statsTicker := time.NewTicker(10 * time.Second)
	defer statsTicker.Stop()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-statsTicker.C:
				elapsed := time.Since(startTime).Round(time.Second)
				sent := sentCount.Load()
				ok := okCount.Load()
				e429 := err429.Load()
				errs := errCount.Load()
				cost := float64(totalCost.Load()) / 1_000_000

				// Collect model counts.
				type mc struct {
					Model string
					Count int64
				}
				var models []mc
				modelCount.Range(func(key, value any) bool {
					models = append(models, mc{Model: key.(string), Count: value.(*atomic.Int64).Load()})
					return true
				})
				sort.Slice(models, func(i, j int) bool { return models[i].Count > models[j].Count })

				modelStr := ""
				for i, m := range models {
					if i > 0 {
						modelStr += " "
					}
					modelStr += fmt.Sprintf("%s:%d", m.Model, m.Count)
					if i >= 4 {
						break
					}
				}

				fmt.Printf("[%s] sent=%d ok=%d err=%d(429:%d) rps=%.1f cost=$%.2f models=[%s]\n",
					elapsed, sent, ok, errs, e429, *rps, cost, modelStr)
			}
		}
	}()

	// Main send loop.
	scenarioRng := rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), 0xdeadbeef))
	batchInterval := time.Duration(float64(time.Second) * float64(*batchSize) / *rps)
	ticker := time.NewTicker(batchInterval)
	defer ticker.Stop()

	backoff := time.Duration(0)

	for {
		select {
		case <-ctx.Done():
			printFinalStats(startTime, &sentCount, &okCount, &errCount, &err429, &totalCost)
			return
		case <-ticker.C:
			if backoff > 0 {
				time.Sleep(backoff)
			}

			elapsed := time.Since(startTime)
			params := sc.Params(elapsed, scenarioRng)

			// Adjust ticker rate based on scenario.
			effectiveRPS := *rps * params.RPSMultiplier
			newInterval := time.Duration(float64(time.Second) * float64(*batchSize) / effectiveRPS)
			ticker.Reset(newInterval)

			// Generate batch of events.
			events := make([]eventPayload, *batchSize)
			var batchCost float64
			for i := 0; i < *batchSize; i++ {
				e := gen.GenerateEventWithParams(time.Now().UTC(), &params)
				events[i] = toPayload(e)
				batchCost += e.CostUSD

				// Track model counts.
				counter, _ := modelCount.LoadOrStore(e.Model, &atomic.Int64{})
				counter.(*atomic.Int64).Add(1)
			}

			sentCount.Add(int64(*batchSize))
			totalCost.Add(int64(batchCost * 1_000_000))

			// Send batch.
			status, err := sendBatch(ctx, client, endpoint, *apiKey, events)
			if err != nil {
				errCount.Add(1)
				if ctx.Err() != nil {
					continue
				}
				logger.Warn("send failed", "error", err)
				backoff = min(backoff*2+time.Second, 30*time.Second)
				continue
			}

			if status == http.StatusTooManyRequests {
				err429.Add(1)
				errCount.Add(1)
				backoff = min(backoff*2+time.Second, 30*time.Second)
				logger.Warn("rate limited (429), backing off", "backoff", backoff)
				continue
			}

			if status >= 400 {
				errCount.Add(1)
				logger.Warn("unexpected status", "status", status)
				continue
			}

			okCount.Add(1)
			backoff = 0
		}
	}
}

type eventPayload struct {
	IdempotencyKey   string            `json:"idempotency_key"`
	Timestamp        string            `json:"timestamp"`
	Model            string            `json:"model"`
	Provider         string            `json:"provider"`
	PromptTokens     uint32            `json:"prompt_tokens"`
	CompletionTokens uint32            `json:"completion_tokens"`
	LatencyMs        uint32            `json:"latency_ms"`
	StatusCode       uint32            `json:"status_code"`
	Tags             map[string]string `json:"tags,omitempty"`
}

type batchRequest struct {
	Events []eventPayload `json:"events"`
}

func toPayload(e datagen.Event) eventPayload {
	return eventPayload{
		IdempotencyKey:   e.EventID,
		Timestamp:        e.Timestamp.Format(time.RFC3339Nano),
		Model:            e.Model,
		Provider:         e.Provider,
		PromptTokens:     e.PromptTokens,
		CompletionTokens: e.CompletionTokens,
		LatencyMs:        e.LatencyMs,
		StatusCode:       e.StatusCode,
		Tags:             e.Tags,
	}
}

func sendBatch(ctx context.Context, client *http.Client, endpoint, apiKey string, events []eventPayload) (int, error) {
	body, err := json.Marshal(batchRequest{Events: events})
	if err != nil {
		return 0, fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("send: %w", err)
	}
	resp.Body.Close()

	return resp.StatusCode, nil
}

func printFinalStats(startTime time.Time, sent, ok, errs, e429, cost *atomic.Int64) {
	elapsed := time.Since(startTime).Round(time.Second)
	actualRPS := float64(sent.Load()) / math.Max(time.Since(startTime).Seconds(), 1)
	fmt.Printf("\n--- Final Statistics ---\n")
	fmt.Printf("Duration:      %s\n", elapsed)
	fmt.Printf("Events sent:   %d\n", sent.Load())
	fmt.Printf("Batches OK:    %d\n", ok.Load())
	fmt.Printf("Batches err:   %d (429: %d)\n", errs.Load(), e429.Load())
	fmt.Printf("Avg RPS:       %.1f\n", actualRPS)
	fmt.Printf("Total cost:    $%.2f\n", float64(cost.Load())/1_000_000)
	fmt.Println()
}
