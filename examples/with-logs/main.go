package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/exporter"
	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/logexporter"
	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/search"
	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/storage"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func main() {
	fmt.Println("🔍 LTT with Logs - Trace-Log Correlation Demo")
	fmt.Println("==============================================")
	fmt.Println()

	// Create temporary storage
	dir := "./demo-logs-data"
	os.RemoveAll(dir) // Clean start
	defer os.RemoveAll(dir)

	cfg := storage.DefaultConfig()
	cfg.Path = dir
	cfg.TTL = 1 * time.Hour

	store, err := storage.NewStore(cfg)
	if err != nil {
		log.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Setup trace exporter
	traceExpCfg := exporter.DefaultConfig()
	traceExpCfg.Storage = &cfg
	traceExp, err := exporter.New(traceExpCfg)
	if err != nil {
		log.Fatalf("Failed to create trace exporter: %v", err)
	}
	defer traceExp.Shutdown(context.Background())

	// Setup log exporter
	logExp, err := logexporter.New(logexporter.Config{
		Storage: store,
	})
	if err != nil {
		log.Fatalf("Failed to create log exporter: %v", err)
	}
	defer logExp.Shutdown(context.Background())

	// Setup OpenTelemetry providers
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(traceExp),
	)
	defer tp.Shutdown(context.Background())
	otel.SetTracerProvider(tp)

	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExp)),
	)
	defer lp.Shutdown(context.Background())
	global.SetLoggerProvider(lp)

	// Create search engine
	engine := search.NewEngine(store)

	fmt.Println("✅ Storage, trace, and log exporters initialized")
	fmt.Println()

	// Simulate some traced operations with logs
	fmt.Println("📝 Generating traces with correlated logs...")
	tracer := otel.Tracer("demo-app")
	logger := global.Logger("demo-app")

	// Scenario 1: Successful payment
	fmt.Println("   Scenario 1: Successful payment")
	processPayment(tracer, logger, 100.00, false)

	// Scenario 2: Failed payment
	fmt.Println("   Scenario 2: Failed payment (declined card)")
	processPayment(tracer, logger, 250.00, true)

	// Scenario 3: Slow database query
	fmt.Println("   Scenario 3: Slow database operation")
	slowDatabaseQuery(tracer, logger)

	fmt.Println()
	time.Sleep(500 * time.Millisecond) // Wait for async export

	// Demo 1: Search for error logs
	fmt.Println("🔍 Demo 1: Search for Error Logs")
	fmt.Println("================================")
	errorQuery, _ := search.NewLogQuery().
		ErrorAndAbove().
		Last(1 * time.Hour).
		Build()

	errorResult, _ := engine.SearchLogs(errorQuery)
	fmt.Printf("Found %d error logs in %v\n", errorResult.Total, errorResult.QueryDuration)
	for _, lg := range errorResult.Logs {
		fmt.Printf("  [ERROR] %s\n", lg.Body)
		fmt.Printf("          TraceID: %s\n", lg.TraceID.String())
	}
	fmt.Println()

	// Demo 2: Get trace with all logs
	if len(errorResult.Logs) > 0 {
		firstErrorLog := errorResult.Logs[0]
		fmt.Println("🔗 Demo 2: Trace-Log Correlation")
		fmt.Println("================================")
		fmt.Printf("Getting complete trace for: %s\n", firstErrorLog.TraceID.String())
		fmt.Println()

		traceWithLogs, _ := engine.GetTraceWithLogs(firstErrorLog.TraceID)
		if traceWithLogs != nil {
			fmt.Printf("Trace Duration: %v\n", traceWithLogs.End.Sub(traceWithLogs.Start))
			fmt.Printf("Spans: %d\n", len(traceWithLogs.Spans))
			fmt.Printf("Logs: %d\n", len(traceWithLogs.Logs))
			fmt.Println()

			fmt.Println("Span Timeline:")
			for _, span := range traceWithLogs.Spans {
				duration := span.EndTime.Sub(span.StartTime)
				status := "✅"
				if codes.Code(span.StatusCode) == codes.Error {
					status = "❌"
				}
				fmt.Printf("  %s %s (%.1fms)\n", status, span.Name, float64(duration.Microseconds())/1000)
			}
			fmt.Println()

			fmt.Println("Log Timeline:")
			for _, lg := range traceWithLogs.Logs {
				severity := "INFO"
				if lg.SeverityNumber >= search.SeverityError {
					severity = "ERROR"
				} else if lg.SeverityNumber >= search.SeverityWarn {
					severity = "WARN"
				}
				fmt.Printf("  [%s] %s\n", severity, lg.Body)
			}
			fmt.Println()

			// Show correlation summary
			summary, _ := engine.GetCorrelationSummary(firstErrorLog.TraceID)
			if summary != nil {
				fmt.Println("Correlation Summary:")
				fmt.Printf("  Spans with logs: %d/%d\n", summary.SpansWithLogs, summary.TotalSpans)
				fmt.Printf("  Average logs per span: %.1f\n", summary.AverageLogsPerSpan)
			}
		}
	}
	fmt.Println()

	// Demo 3: Find traces with errors
	fmt.Println("🎯 Demo 3: Find All Traces with Errors")
	fmt.Println("=======================================")
	errorTraces, _ := engine.FindTracesWithErrors(search.TimeRange{
		Start: time.Now().Add(-1 * time.Hour),
		End:   time.Now(),
	}, 10)

	fmt.Printf("Found %d traces with errors:\n", len(errorTraces))
	for _, traceID := range errorTraces {
		fmt.Printf("  • %s\n", traceID.String())
	}
	fmt.Println()

	// Demo 4: Timeline view
	if len(errorTraces) > 0 {
		fmt.Println("📅 Demo 4: Timeline View (Spans + Logs)")
		fmt.Println("========================================")
		timeline, _ := engine.GetTimeline(errorTraces[0])
		if timeline != nil {
			fmt.Printf("Showing timeline for trace: %s\n\n", errorTraces[0].String())
			for _, event := range timeline {
				relTime := event.Timestamp.Sub(timeline[0].Timestamp)
				switch event.Type {
				case "span_start":
					fmt.Printf("  +%.1fms  ▶ START: %s\n", float64(relTime.Microseconds())/1000, event.Span.Name)
				case "span_end":
					fmt.Printf("  +%.1fms  ◀ END:   %s\n", float64(relTime.Microseconds())/1000, event.Span.Name)
				case "log":
					severity := "INFO"
					if event.Log.SeverityNumber >= search.SeverityError {
						severity = "ERROR"
					}
					fmt.Printf("  +%.1fms  📝 LOG [%s]: %s\n", float64(relTime.Microseconds())/1000, severity, event.Log.Body)
				}
			}
		}
	}

	fmt.Println()
	fmt.Println("================================")
	fmt.Println("✅ Trace-Log Correlation Demo Complete!")
	fmt.Println()
	fmt.Println("💡 Key Features Demonstrated:")
	fmt.Println("   • Automatic trace-log correlation via TraceID")
	fmt.Println("   • Search logs by severity (errors, warnings)")
	fmt.Println("   • Get all logs for a specific trace")
	fmt.Println("   • Find traces that have errors")
	fmt.Println("   • Unified timeline of spans + logs")
	fmt.Println("   • Correlation statistics")
}

func processPayment(tracer sdktrace.Tracer, logger otellog.Logger, amount float64, fail bool) {
	ctx := context.Background()

	// Start trace
	ctx, span := tracer.Start(ctx, "Payment Processing")
	defer span.End()

	span.SetAttributes(
		attribute.Float64("payment.amount", amount),
		attribute.String("payment.currency", "USD"),
	)

	// Log: Payment started
	logger.Emit(ctx, otellog.Record{
		Timestamp: time.Now(),
		Severity:  otellog.SeverityInfo,
		Body:      otellog.StringValue(fmt.Sprintf("Processing payment of $%.2f", amount)),
	})

	// Simulate validation
	time.Sleep(10 * time.Millisecond)
	logger.Emit(ctx, otellog.Record{
		Timestamp: time.Now(),
		Severity:  otellog.SeverityDebug,
		Body:      otellog.StringValue("Payment validation passed"),
	})

	// Simulate payment gateway call
	ctx, gatewaySpan := tracer.Start(ctx, "Stripe API Call")
	time.Sleep(50 * time.Millisecond)

	if fail {
		// Payment failed
		logger.Emit(ctx, otellog.Record{
			Timestamp: time.Now(),
			Severity:  otellog.SeverityWarn,
			Body:      otellog.StringValue("Payment gateway returned error"),
		})

		logger.Emit(ctx, otellog.Record{
			Timestamp: time.Now(),
			Severity:  otellog.SeverityError,
			Body:      otellog.StringValue("Payment declined: insufficient funds"),
		})

		gatewaySpan.SetStatus(codes.Error, "Payment declined")
		span.SetStatus(codes.Error, "Payment failed")
	} else {
		logger.Emit(ctx, otellog.Record{
			Timestamp: time.Now(),
			Severity:  otellog.SeverityInfo,
			Body:      otellog.StringValue("Payment successfully processed"),
		})

		gatewaySpan.SetStatus(codes.Ok, "")
		span.SetStatus(codes.Ok, "")
	}

	gatewaySpan.End()
}

func slowDatabaseQuery(tracer sdktrace.Tracer, logger otellog.Logger) {
	ctx := context.Background()

	ctx, span := tracer.Start(ctx, "Fetch User Data")
	defer span.End()

	logger.Emit(ctx, otellog.Record{
		Timestamp: time.Now(),
		Severity:  otellog.SeverityInfo,
		Body:      otellog.StringValue("Executing database query"),
	})

	// Slow query
	ctx, dbSpan := tracer.Start(ctx, "SELECT * FROM users")
	time.Sleep(time.Duration(200+rand.Intn(100)) * time.Millisecond)

	logger.Emit(ctx, otellog.Record{
		Timestamp: time.Now(),
		Severity:  otellog.SeverityWarn,
		Body:      otellog.StringValue("Query took longer than expected (250ms)"),
	})

	dbSpan.End()
	span.SetStatus(codes.Ok, "")
}
