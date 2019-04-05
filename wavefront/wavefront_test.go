package wavefront

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/wavefronthq/wavefront-sdk-go/application"
	"github.com/wavefronthq/wavefront-sdk-go/histogram"
	"github.com/wavefronthq/wavefront-sdk-go/senders"

	"go.opencensus.io/exemplar"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.opencensus.io/trace"
)

var fakeExp, benchExp *Exporter
var vd1, vd2, vd3, vd4 *view.Data
var sd1, sd2 *trace.SpanData
var senderErr senders.Sender
var appTags application.Tags

// Fake Sender
type FakeSender struct {
	Echo         bool
	Error        error
	FailureCount int64
}

func (s *FakeSender) SendMetric(name string, value float64, ts int64, source string, tags map[string]string) error {
	if s.Echo {
		fmt.Println(name, value, ts, source, tags)
	}
	return s.Error
}

func (s *FakeSender) SendDeltaCounter(name string, value float64, source string, tags map[string]string) error {
	if s.Echo {
		fmt.Println(name, value, source, tags)
	}
	return s.Error
}

func (s *FakeSender) SendDistribution(name string, centroids []histogram.Centroid, hgs map[histogram.Granularity]bool, ts int64, source string, tags map[string]string) error {
	if s.Echo {
		fmt.Println(name, centroids, hgs, ts, source, tags)
	}
	return s.Error
}

func (s *FakeSender) SendSpan(name string, startMillis, durationMillis int64, source, traceId, spanId string, parents, followsFrom []string, tags []senders.SpanTag, spanLogs []senders.SpanLog) error {
	if s.Echo {
		fmt.Println(name, startMillis, durationMillis, source, traceId, spanId, parents, followsFrom, tags, spanLogs)
	}
	return s.Error
}

func (s *FakeSender) GetFailureCount() int64 {
	return s.FailureCount
}

func (s *FakeSender) Start() {

}
func (s *FakeSender) Flush() error {
	return s.Error
}
func (s *FakeSender) Close() {
}

func init() {
	appTags = application.New("test-app", "test-service")
	sender := &FakeSender{Echo: true}
	fakeExp, _ = NewExporter(sender, Source("FakeSource"), AppTags(appTags), Granularity(histogram.MINUTE), DisableSelfHealth())

	sender2 := &FakeSender{Echo: false}
	benchExp, _ = NewExporter(sender2, Source("FakeSource"), AppTags(appTags), QueueSize(0), DisableSelfHealth()) // Drop msgs when benching

	senderErr = &FakeSender{Echo: true, Error: errors.New("FakeError")}

	vd1 = &view.Data{
		Start: time.Now(),
		End:   time.Now(),
		View: &view.View{
			Name:        "v1",
			Description: "v1",
			Measure:     stats.Float64("v1", "v1", "v1unit"),
			Aggregation: view.LastValue(),
		},
		Rows: []*view.Row{
			&view.Row{
				Tags: []tag.Tag{},
				Data: &view.LastValueData{
					Value: 4.5,
				},
			},
		},
	}

	vd2 = &view.Data{
		Start: time.Now(),
		End:   time.Now(),
		View: &view.View{
			Name:        "v2",
			Description: "v2",
			Measure:     stats.Int64("v2", "v2", "v1unit"),
			Aggregation: view.Sum(),
		},
		Rows: []*view.Row{
			&view.Row{
				Tags: []tag.Tag{},
				Data: &view.SumData{
					Value: 4,
				},
			},
		},
	}

	vd3 = &view.Data{
		Start: time.Now(),
		End:   time.Now(),
		View: &view.View{
			Name:        "v3",
			Description: "v3",
			Measure:     stats.Int64("v3", "v3", "v3unit"),
			Aggregation: view.Count(),
		},
		Rows: []*view.Row{
			&view.Row{
				Tags: []tag.Tag{},
				Data: &view.CountData{
					Value: 4,
				},
			},
		},
	}

	vd4 = &view.Data{
		Start: time.Now(),
		End:   time.Now(),
		View: &view.View{
			Name:        "v4",
			Description: "v4",
			Measure:     stats.Int64("v4", "v4", "v4unit"),
			Aggregation: view.Distribution(10, 30),
		},
		Rows: []*view.Row{
			&view.Row{
				Tags: []tag.Tag{},
				Data: &view.DistributionData{
					Count: 2, Min: 9, Max: 27, Mean: 18, SumOfSquaredDev: 0,
					CountPerBucket: []int64{1, 2, 1},
					ExemplarsPerBucket: []*exemplar.Exemplar{
						&exemplar.Exemplar{Value: 9},
						&exemplar.Exemplar{Value: 27},
						&exemplar.Exemplar{Value: 37},
					},
				},
			},
		},
	}

	sd1 = &trace.SpanData{
		SpanContext: trace.SpanContext{
			TraceID:      trace.TraceID{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
			SpanID:       trace.SpanID{0, 1, 2, 3, 4, 5, 6, 7},
			TraceOptions: 0x1,
		},
		ParentSpanID: trace.SpanID{},
		Name:         "span",
		StartTime:    time.Now(),
		EndTime:      time.Now(),
		Status: trace.Status{
			Code:    trace.StatusCodeUnknown,
			Message: "some error",
		},
		Attributes: map[string]interface{}{
			"foo1": "bar1",
			"foo2": 5.25,
			"foo3": 42,
		},
		Annotations: []trace.Annotation{
			{Message: "1.500000", Attributes: map[string]interface{}{"key1": float32(1.0)}},
			{Message: "Annotate", Attributes: map[string]interface{}{"key2": uint8(5)}},
		},
		MessageEvents: []trace.MessageEvent{
			{EventType: 2, MessageID: 0x3, UncompressedByteSize: 0x190, CompressedByteSize: 0x12c},
			{EventType: 1, MessageID: 0x1, UncompressedByteSize: 0xc8, CompressedByteSize: 0x64},
		},
	}

	sd2 = &trace.SpanData{
		SpanContext: trace.SpanContext{
			TraceID:      trace.TraceID{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
			SpanID:       trace.SpanID{8, 9, 10, 11, 12, 13, 14},
			TraceOptions: 0x1,
		},
		ParentSpanID: trace.SpanID{0, 1, 2, 3, 4, 5, 6, 7},
		Name:         "span",
		StartTime:    time.Now(),
		EndTime:      time.Now(),
		Status: trace.Status{
			Code:    trace.StatusCodeUnknown,
			Message: "some error",
		},
		Attributes: map[string]interface{}{
			"foo1": "bar1",
			"foo2": 5.25,
			"foo3": 42,
		},
		Annotations: []trace.Annotation{
			{Message: "1.500000", Attributes: map[string]interface{}{"key1": float32(1.0)}},
			{Message: "Annotate", Attributes: map[string]interface{}{"key2": uint8(5)}},
		},
		MessageEvents: []trace.MessageEvent{
			{EventType: 2, MessageID: 0x3, UncompressedByteSize: 0x190, CompressedByteSize: 0x12c},
			{EventType: 1, MessageID: 0x1, UncompressedByteSize: 0xc8, CompressedByteSize: 0x64},
		},
	}
}

func TestProcessSpan(t *testing.T) {
	fakeExp.ExportSpan(sd1)
	fakeExp.ExportSpan(sd2)
	fakeExp.Flush()
}

func TestProcessView(tt *testing.T) {
	tt.Run("MetricLV", func(t *testing.T) {
		fakeExp.processView(vd1)
	})
	tt.Run("MetricSum", func(t *testing.T) {
		fakeExp.processView(vd2)
	})
	tt.Run("MetricCount", func(t *testing.T) {
		fakeExp.processView(vd3)
	})
	tt.Run("Distribution", func(t *testing.T) {
		fakeExp.processView(vd4)
	})

	fakeExp.Flush()
}

func TestNegativeQueueSize(t *testing.T) {
	sender := &FakeSender{Echo: false}
	errorExp, err := NewExporter(sender, Source("FakeSource"), QueueSize(-10))
	if errorExp != nil || err == nil {
		t.FailNow()
	}
}

func TestQueueErrors(t *testing.T) {
	sender := &FakeSender{Echo: false}
	errorExp, _ := NewExporter(sender, Source("FakeSource"), AppTags(appTags), Granularity(histogram.MINUTE), QueueSize(0), VerboseLogging())
	errorExp.selfHealthTicker.Stop()
	errorExp.selfHealthTicker = time.NewTicker(500 * time.Millisecond)

	errorExp.ExportSpan(sd1)
	errorExp.ExportView(vd1)
	errorExp.ExportView(vd4)
	time.Sleep(time.Second)
	errorExp.Stop()
	if errorExp.SpansDropped() != 1 || errorExp.MetricsDropped() != 2 {
		t.FailNow()
	}
}

func TestSenderErrors(t *testing.T) {
	errorExp, _ := NewExporter(senderErr, Source("FakeSource"), AppTags(appTags), Granularity(histogram.MINUTE), VerboseLogging())
	errorExp.selfHealthTicker.Stop()
	errorExp.selfHealthTicker = time.NewTicker(500 * time.Millisecond)

	errorExp.ExportSpan(sd1)
	errorExp.ExportView(vd1)
	errorExp.ExportView(vd4)
	time.Sleep(time.Second)
	errorExp.Stop()

	if errorExp.SenderErrors() < 3 {
		t.FailNow()
	}
}

func BenchmarkProcessSpan(bb *testing.B) {

	exs := &trace.SpanData{
		SpanContext: trace.SpanContext{
			TraceID:      trace.TraceID{},
			SpanID:       trace.SpanID{},
			TraceOptions: 0x1,
		},
		ParentSpanID: trace.SpanID{},
		Name:         "span",
		StartTime:    time.Now(),
		EndTime:      time.Now(),
	}

	bb.Run("Basic", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			benchExp.processSpan(exs)
		}
	})

	exs.Status = trace.Status{
		Code:    trace.StatusCodeOK,
		Message: "all ok",
	}

	bb.Run("+OkStatus", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			benchExp.processSpan(exs)
		}
	})

	exs.Status = trace.Status{
		Code:    trace.StatusCodeUnknown,
		Message: "some error",
	}

	bb.Run("+ErrorStatus", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			benchExp.processSpan(exs)
		}
	})

	exs.Attributes = map[string]interface{}{
		"foo1": "bar1",
		"foo2": 5.25,
		"foo3": 42,
	}

	bb.Run("+Attributes", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			benchExp.processSpan(exs)
		}
	})

	exs.Annotations = []trace.Annotation{
		{Message: "1.500000", Attributes: map[string]interface{}{"key1": "value1"}},
		{Message: "Annotate", Attributes: map[string]interface{}{"key2": 1.234}},
	}

	bb.Run("+Annotations", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			benchExp.processSpan(exs)
		}
	})

	exs.MessageEvents = []trace.MessageEvent{
		{EventType: 2, MessageID: 0x3, UncompressedByteSize: 0x190, CompressedByteSize: 0x12c},
		{EventType: 1, MessageID: 0x1, UncompressedByteSize: 0xc8, CompressedByteSize: 0x64},
	}

	bb.Run("+MsgEvents", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			benchExp.processSpan(exs)
		}
	})
}

func BenchmarkProcessView(bb *testing.B) {

	bb.Run("Metric", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			benchExp.processView(vd1)
		}
	})
	bb.Run("Distribution", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			benchExp.processView(vd2)
		}
	})
}
