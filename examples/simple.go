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

	workTime   = stats.Int64("ocexample.works.time", "", stats.UnitMilliseconds)
	totalWorks = stats.Int64("ocexample.works.count", "", stats.UnitDimensionless)

	exporter *wavefront.Exporter
)

func main() {
	ctx := context.Background()

	// Configure Wavefront Sender
	proxyCfg := &senders.ProxyConfiguration{
		Host:             "Your_Proxy_Host_Or_IP",
		MetricsPort:      2878,
		TracingPort:      30000,
		DistributionPort: 40000,
	}
	sender, _ := senders.NewProxySender(proxyCfg)
	defer sender.Close()

	appTags := application.New("opencensus-example", "simple-service")

	// Configure Wavefront Exporter
	qSize := 10
	var err error
	exporter, err = wavefront.NewExporter(sender, wavefront.VerboseLogging(),
		wavefront.QueueSize(qSize), wavefront.AppTags(appTags),
		wavefront.Granularity(histogram.MINUTE))
	if err != nil {
		log.Fatal("Could not create Wavefront Exporter:", err)
	}
	defer exporter.Stop()

	appHB = application.StartHeartbeatService(sender, appTags, "", "simple-service", "exporter-service")

	// Register exporter with OpenCensus
	trace.RegisterExporter(exporter)
	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})
	view.RegisterExporter(exporter)
	view.SetReportingPeriod(time.Second)

	// Register Views
	if verr := view.Register(&view.View{
		Name:        "ocexample.works.time",
		Description: "",
		Measure:     workTime,
		Aggregation: view.LastValue(),
	}); verr != nil {
		log.Fatal("Could not register view:", err)
	}
	if verr := view.Register(&view.View{
		Name:        "ocexample.works.count",
		Description: "",
		Measure:     totalWorks,
		Aggregation: view.Count(),
	}); verr != nil {
		log.Fatal("Could not register view:", err)
	}

	for { // generate metrics endlessly
		fmt.Println("Random work...")
		parent(ctx)
		time.Sleep(5 * time.Second)
	}

}

func parent(ctx context.Context) {
	ctx, span := trace.StartSpan(ctx, "main")
	startTime := time.Now()
	for i := 0; i < 5; i++ {
		stats.Record(ctx, totalWorks.M(1))
		work(ctx, i+1)
	}
	stats.Record(ctx, workTime.M(time.Since(startTime).Nanoseconds()/1e6))
	time.Sleep(500 * time.Millisecond)
	span.End()
}

func work(ctx context.Context, index int) {
	ctx, span := trace.StartSpan(ctx, fmt.Sprintf("work-%d", index))
	defer span.End()

	randTime := 100 + rand.Int63n(100)
	time.Sleep(time.Duration(randTime) * time.Millisecond) // work
}
