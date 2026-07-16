package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/exporter"
	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/storage"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func main() {
	fmt.Println("🗄️  LTT with Persistent Storage Example")
	fmt.Println("📊 Spans are saved to BadgerDB and survive restarts")
	fmt.Println()

	// Create config with persistent storage enabled
	config := exporter.DefaultConfig()
	storageConfig := storage.DefaultConfig()
	storageConfig.Path = "./ltt-data"
	storageConfig.TTL = 24 * time.Hour
	config.Storage = &storageConfig

	// Create LTT exporter
	exp, err := exporter.New(config)
	if err != nil {
		log.Fatalf("Failed to create exporter: %v", err)
	}
	defer exp.Shutdown(context.Background())

	// Check if we have historical data
	store := exp.GetStore()
	if store != nil {
		count, err := store.Count()
		if err == nil && count > 0 {
			fmt.Printf("📚 Found %d historical spans in database\n", count)
			fmt.Println()
		}
	}

	// Create trace provider with syncer for immediate export
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exp),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	defer tp.Shutdown(context.Background())

	otel.SetTracerProvider(tp)

	tracer := tp.Tracer("storage-example")

	fmt.Println("✅ Persistent storage enabled at ./ltt-data")
	fmt.Println("💡 Spans will survive application restarts")
	fmt.Println("🕐 TTL: 24 hours")
	fmt.Println()
	fmt.Println("Generating traces...")

	// Generate some traces
	for i := 0; i < 50; i++ {
		ctx := context.Background()

		// Root span
		ctx, span := tracer.Start(ctx, fmt.Sprintf("Order-Processing-%d", i))
		span.SetAttributes(
			attribute.String("order.id", fmt.Sprintf("ORD-%d", 1000+i)),
			attribute.String("customer.id", fmt.Sprintf("CUST-%d", rand.Intn(100))),
			attribute.Float64("order.amount", float64(rand.Intn(500))+10.99),
		)

		// Simulate order processing
		simulateOrderProcessing(ctx, tracer)

		// Randomly fail some orders
		if rand.Float64() < 0.1 {
			span.SetStatus(codes.Error, "Payment failed")
			span.SetAttributes(attribute.String("error", "Insufficient funds"))
		} else {
			span.SetStatus(codes.Ok, "Order completed")
		}

		span.End()

		// Print stats
		stats := exp.GetStats()
		if store != nil {
			count, _ := store.Count()
			fmt.Printf("\rSpans: %d | In DB: %d | Buffer: %.1f%% | Connected: %v  ",
				stats.ExportedSpans,
				count,
				stats.BufferUsage,
				exp.IsConnected(),
			)
		}

		time.Sleep(200 * time.Millisecond)
	}

	fmt.Println("\n")
	fmt.Println("✅ Done! Generated 50 spans")

	// Show database stats
	if store != nil {
		count, err := store.Count()
		if err == nil {
			fmt.Printf("📊 Total spans in database: %d\n", count)
		}

		// Show recent spans
		fmt.Println()
		fmt.Println("📝 Recent spans (last 5):")
		recent, err := store.GetRecent(5)
		if err == nil {
			for i, span := range recent {
				fmt.Printf("  %d. %s (%.1fms)\n",
					i+1,
					span.Name,
					float64(span.EndTime.Sub(span.StartTime).Microseconds())/1000.0,
				)
			}
		}
	}

	fmt.Println()
	fmt.Println("💡 TIP: Run this program again to see historical spans loaded!")
	fmt.Println("💡 TIP: Check ./ltt-data directory for BadgerDB files")
	fmt.Println()
	fmt.Println("Press Ctrl+C to exit...")
	select {}
}

func simulateOrderProcessing(ctx context.Context, tracer trace.Tracer) {
	// Validate order
	_, validateSpan := tracer.Start(ctx, "Validate Order")
	validateSpan.SetAttributes(
		attribute.String("validation.status", "passed"),
	)
	time.Sleep(time.Duration(5+rand.Intn(10)) * time.Millisecond)
	validateSpan.End()

	// Check inventory
	_, inventorySpan := tracer.Start(ctx, "Check Inventory")
	inventorySpan.SetAttributes(
		attribute.String("warehouse.id", "WH-01"),
		attribute.Int("items.available", rand.Intn(100)),
	)
	time.Sleep(time.Duration(10+rand.Intn(20)) * time.Millisecond)
	inventorySpan.End()

	// Process payment
	_, paymentSpan := tracer.Start(ctx, "Process Payment")
	paymentSpan.SetAttributes(
		attribute.String("payment.method", "credit_card"),
		attribute.String("payment.gateway", "stripe"),
	)
	time.Sleep(time.Duration(20+rand.Intn(30)) * time.Millisecond)
	paymentSpan.End()

	// Ship order
	_, shipSpan := tracer.Start(ctx, "Ship Order")
	shipSpan.SetAttributes(
		attribute.String("carrier", "FedEx"),
		attribute.String("tracking", fmt.Sprintf("TRK-%d", rand.Intn(999999))),
	)
	time.Sleep(time.Duration(5+rand.Intn(10)) * time.Millisecond)
	shipSpan.End()
}
