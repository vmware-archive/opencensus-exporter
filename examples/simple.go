package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/wavefronthq/opencensus-exporter/wavefront"
	"github.com/wavefronthq/wavefront-sdk-go/application"
	"github.com/wavefronthq/wavefront-sdk-go/histogram"
	"github.com/wavefronthq/wavefront-sdk-go/senders"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"
)

var (
	appHB application.HeartbeatService

	workTime   = stats.Int64("work-times.sum", "", stats.UnitMilliseconds)
	totalWorks = stats.Int64("works.count", "", stats.UnitDimensionless)

	exporter *wavefront.Exporter
)

func main() {
	ctx := context.Background()

	// Configure Wavefront Sender
	proxyCfg := &senders.ProxyConfiguration{
		Host:             "Your_Proxy_Host_Or_IP",
		MetricsPort:      2878,
		DistributionPort: 40000,
		TracingPort:      50000,
	}
	sender, _ := senders.NewProxySender(proxyCfg)
	defer sender.Close()

	appTags := application.New("opencensus-example", "example-service")

	// Configure Wavefront Exporter
	qSize := 10
	var err error
	exporter, err = wavefront.NewExporter(sender, wavefront.VerboseLogging(),
		wavefront.QueueSize(qSize), wavefront.AppTags(appTags),
		wavefront.Granularity(histogram.MINUTE))
	if err != nil {
		log.Fatal("Could not create Wavefront Exporter:", err)
	}
	defer func() {
		exporter.StopSelfHealth()
		exporter.Flush()
	}()

	appHB = application.StartHeartbeatService(sender, appTags, "", "example-service", "exporter-service")

	// Register exporter with OpenCensus
	trace.RegisterExporter(exporter)
	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})
	view.RegisterExporter(exporter)
	view.SetReportingPeriod(time.Second)

	// Register Views
	if verr := view.Register(&view.View{
		Name:        "work-times.sum",
		Description: "",
		Measure:     workTime,
		Aggregation: view.Sum(),
	}); verr != nil {
		log.Fatal("Could not register view:", err)
	}
	if verr := view.Register(&view.View{
		Name:        "works.count",
		Description: "",
		Measure:     totalWorks,
		Aggregation: view.Count(),
	}); verr != nil {
		log.Fatal("Could not register view:", err)
	}

	for { // generate metrics endlessly
		parent(ctx)
		time.Sleep(5 * time.Second)
	}

}

func parent(ctx context.Context) {
	ctx, span := trace.StartSpan(ctx, "main")
	for i := 0; i < 5; i++ {
		stats.Record(ctx, totalWorks.M(1))
		work(ctx, i+1)
	}
	time.Sleep(500 * time.Millisecond)
	span.End()
}

func work(ctx context.Context, index int) {
	ctx, span := trace.StartSpan(ctx, fmt.Sprintf("work-%d", index))
	defer span.End()

	rand_time := 100 + rand.Int63n(100)
	stats.Record(ctx, workTime.M(rand_time))
	time.Sleep(time.Duration(rand_time) * time.Millisecond) // work
}
