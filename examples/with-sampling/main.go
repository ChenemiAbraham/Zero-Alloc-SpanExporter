package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/exporter"
	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/sampler"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func main() {
	fmt.Println("🎲 LTT with Span Sampling Example")
	fmt.Println("📉 Demonstrates different sampling strategies")
	fmt.Println()

	// Choose sampling strategy (change this to test different samplers)
	samplingType := chooseSamplingStrategy()

	// Create config with sampling enabled
	config := exporter.DefaultConfig()
	config.Sampler = &samplingType

	// Create LTT exporter
	exp, err := exporter.New(config)
	if err != nil {
		log.Fatalf("Failed to create exporter: %v", err)
	}
	defer exp.Shutdown(context.Background())

	// Create trace provider with syncer
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exp),
		sdktrace.WithSampler(sdktrace.AlwaysSample()), // OTEL SDK samples everything, let LTT do the sampling
	)
	defer tp.Shutdown(context.Background())

	otel.SetTracerProvider(tp)

	tracer := otel.Tracer("sampling-example")

	fmt.Printf("✅ Sampling enabled: %s\n", config.Sampler.Type)
	printSamplingConfig(*config.Sampler)
	fmt.Println()
	fmt.Println("Generating traces...")

	// Generate continuous traces
	count := 0
	for {
		ctx := context.Background()

		// Create API request span
		ctx, span := tracer.Start(ctx, "API Request")
		span.SetAttributes(
			attribute.String("http.method", "GET"),
			attribute.String("http.route", fmt.Sprintf("/api/resource-%d", rand.Intn(10))),
		)

		// Simulate work
		simulateWork(ctx, tracer)

		// Randomly make some requests slow or fail (for tail sampling)
		duration := time.Duration(10+rand.Intn(40)) * time.Millisecond
		if rand.Float64() < 0.05 {
			// 5% of requests are slow
			duration = time.Duration(1000+rand.Intn(2000)) * time.Millisecond
		}
		time.Sleep(duration)

		if rand.Float64() < 0.1 {
			// 10% error rate
			span.SetStatus(codes.Error, "Server error")
		} else {
			span.SetStatus(codes.Ok, "")
		}

		span.End()
		count++

		// Print stats every 50 spans
		if count%50 == 0 {
			stats := exp.GetStats()
			fmt.Printf("\r[%s] Generated: %d | Exported: %d | Sampled Out: %d | Sampling Rate: %.1f%%  ",
				time.Now().Format("15:04:05"),
				count,
				stats.ExportedSpans,
				stats.SampledSpans,
				stats.SamplingRate*100,
			)
		}

		time.Sleep(50 * time.Millisecond) // 20 spans/sec
	}
}

func chooseSamplingStrategy() sampler.Config {
	// Uncomment one of these to test different strategies:

	// 1. Probability Sampling - Sample 10% of all spans
	return sampler.Config{
		Type:        "probability",
		Probability: 0.1, // 10%
	}

	// 2. Rate Limiting - Max 100 spans per second
	// return sampler.Config{
	// 	Type: "rate",
	// 	Rate: 100,
	// }

	// 3. Tail Sampling - Keep errors + slow spans, sample others at 1%
	// return sampler.Config{
	// 	Type: "tail",
	// 	Tail: sampler.TailConfig{
	// 		SampleErrors:    true,
	// 		SlowThreshold:   500 * time.Millisecond,
	// 		BaseProbability: 0.01, // 1% for normal spans
	// 	},
	// }

	// 4. Adaptive Sampling - Auto-adjust to hit 200 spans/sec
	// return sampler.Config{
	// 	Type: "adaptive",
	// 	Adaptive: sampler.AdaptiveConfig{
	// 		TargetSpansPerSecond: 200,
	// 		AdjustInterval:       10 * time.Second,
	// 		MinProbability:       0.01,
	// 		MaxProbability:       1.0,
	// 	},
	// }

	// 5. Always sample (no sampling)
	// return sampler.Config{
	// 	Type: "always",
	// }

	// 6. Never sample (drop all - for testing)
	// return sampler.Config{
	// 	Type: "never",
	// }
}

func printSamplingConfig(config sampler.Config) {
	switch config.Type {
	case "probability":
		fmt.Printf("   Strategy: Probability-based\n")
		fmt.Printf("   Rate: %.1f%% of all spans\n", config.Probability*100)
	case "rate":
		fmt.Printf("   Strategy: Rate limiting\n")
		fmt.Printf("   Max: %d spans/second\n", config.Rate)
	case "tail":
		fmt.Printf("   Strategy: Tail sampling\n")
		fmt.Printf("   Keep: Errors=%v, Slow>%v\n", config.Tail.SampleErrors, config.Tail.SlowThreshold)
		fmt.Printf("   Base rate: %.1f%%\n", config.Tail.BaseProbability*100)
	case "adaptive":
		fmt.Printf("   Strategy: Adaptive\n")
		fmt.Printf("   Target: %d spans/second\n", config.Adaptive.TargetSpansPerSecond)
	case "always":
		fmt.Printf("   Strategy: Always sample (100%%)\n")
	case "never":
		fmt.Printf("   Strategy: Never sample (0%%)\n")
	}
}

func simulateWork(ctx context.Context, tracer trace.Tracer) {
	_, dbSpan := tracer.Start(ctx, "Database Query")
	time.Sleep(time.Duration(5+rand.Intn(15)) * time.Millisecond)
	dbSpan.End()

	_, cacheSpan := tracer.Start(ctx, "Cache Lookup")
	time.Sleep(time.Duration(1+rand.Intn(5)) * time.Millisecond)
	cacheSpan.End()
}
