package wavefront

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/wavefronthq/wavefront-sdk-go/application"
	"github.com/wavefronthq/wavefront-sdk-go/event"
	"github.com/wavefronthq/wavefront-sdk-go/histogram"
	"github.com/wavefronthq/wavefront-sdk-go/senders"

	"go.opencensus.io/metric/metricdata"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.opencensus.io/trace"
)

// Fake Sender
type FakeSender struct {
	Echo         bool
	Error        error
	FailureCount int64
	TestData     map[string][]interface{}
	mutex        sync.Mutex
}

var fakeExp, benchExp *Exporter
var vd1, vd2, vd3, vd4 *view.Data
var sd1, sd2 *trace.SpanData
var senderErr *FakeSender
var appTags application.Tags

func newFake() *FakeSender {
	return &FakeSender{
		Echo:     false,
		TestData: nil,
		mutex:    sync.Mutex{},
	}
}

func (s *FakeSender) update(name string, have []interface{}) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.TestData == nil {
		return
	}

	if _, set := s.TestData[name]; !set {
		s.TestData["^^^^"+name] = have
		return
	}

	if reflect.DeepEqual(s.TestData[name], have) {
		delete(s.TestData, name)
		fmt.Println("delete", name)
	} else {
		fmt.Println("MISMATCH", name)
		fmt.Printf("HAVE %#v\n", have)
		fmt.Printf("WANT %#v\n", s.TestData[name])
	}

}

func (s *FakeSender) verify() bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.TestData == nil || len(s.TestData) == 0 {
		return true
	}

	for k, v := range s.TestData {
		fmt.Printf("%s => %#v\n", k, v)
	}
	return false
}

func (s *FakeSender) SendMetric(name string, value float64, ts int64, source string, tags map[string]string) error {
	have := []interface{}{name, value, ts, source, tags}
	s.update(name, have)

	if s.Echo {
		fmt.Println(name, value, ts, source, tags)
	}
	return s.Error
}

func (s *FakeSender) SendDeltaCounter(name string, value float64, source string, tags map[string]string) error {
	have := []interface{}{name, value, source, tags}
	s.update(name, have)

	if s.Echo {
		fmt.Println(name, value, source, tags)
	}
	return s.Error
}

func (s *FakeSender) SendDistribution(name string, centroids []histogram.Centroid, hgs map[histogram.Granularity]bool, ts int64, source string, tags map[string]string) error {
	have := []interface{}{name, centroids, hgs, ts, source, tags}
	s.update(name, have)

	if s.Echo {
		fmt.Println(name, centroids, hgs, ts, source, tags)
	}
	return s.Error
}

func (s *FakeSender) SendSpan(name string, startMillis, durationMillis int64, source, traceID, spanID string, parents, followsFrom []string, tags []senders.SpanTag, spanLogs []senders.SpanLog) error {
	sort.SliceStable(tags, func(i1, i2 int) bool { return tags[i1].Key < tags[i2].Key })

	have := []interface{}{name, startMillis, durationMillis, source, traceID, spanID, parents, followsFrom, tags, spanLogs}
	s.update(name, have)

	if s.Echo {
		fmt.Println(name, startMillis, durationMillis, source, traceID, spanID, parents, followsFrom, tags, spanLogs)
	}
	return s.Error
}

func (s *FakeSender) SendEvent(name string, startMillis, endMillis int64, source string, tags []string, setters ...event.Option) error {
	return nil
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

	sender := newFake()
	benchExp, _ = NewExporter(sender, Source("FakeSource"), AppTags(appTags), QueueSize(0), DisableSelfHealth()) // Drop msgs when benching

	senderErr = newFake()
	senderErr.Error = errors.New("FakeError")

	vd1 = &view.Data{
		Start: time.Unix(12345, 0),
		End:   time.Unix(67890, 1e6),
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
		Start: time.Unix(12345, 0),
		End:   time.Unix(67890, 1e6),
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
		Start: time.Unix(12345, 0),
		End:   time.Unix(67890, 1e6),
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
		Start: time.Unix(12345, 0),
		End:   time.Unix(67890, 1e6),
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
					ExemplarsPerBucket: []*metricdata.Exemplar{
						&metricdata.Exemplar{Value: 9},
						&metricdata.Exemplar{Value: 27},
						&metricdata.Exemplar{Value: 37},
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
		StartTime:    time.Unix(12345, 0),
		EndTime:      time.Unix(67890, 1e6),
		ParentSpanID: trace.SpanID{},
		Name:         "span",
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
			{Time: time.Unix(12345, 0), Message: "1.500000", Attributes: map[string]interface{}{"key1": float32(1.0)}},
			{Time: time.Unix(12345, 0), Message: "Annotate", Attributes: map[string]interface{}{"key2": uint8(5)}},
		},
		MessageEvents: []trace.MessageEvent{
			{Time: time.Unix(12345, 0), EventType: 2, MessageID: 0x3, UncompressedByteSize: 0x190, CompressedByteSize: 0x12c},
			{Time: time.Unix(12345, 0), EventType: 1, MessageID: 0x1, UncompressedByteSize: 0xc8, CompressedByteSize: 0x64},
		},
	}

	sd2 = &trace.SpanData{
		SpanContext: trace.SpanContext{
			TraceID:      trace.TraceID{1, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
			SpanID:       trace.SpanID{8, 9, 10, 11, 12, 13, 14},
			TraceOptions: 0x1,
		},
		ParentSpanID: trace.SpanID{0, 1, 2, 3, 4, 5, 6, 7},
		Name:         "span2",
		StartTime:    time.Unix(12345, 0),
		EndTime:      time.Unix(67890, 1e6),
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
			{Time: time.Unix(12345, 0), Message: "1.500000", Attributes: map[string]interface{}{"key1": float32(1.0)}},
			{Time: time.Unix(12345, 0), Message: "Annotate", Attributes: map[string]interface{}{"key2": uint8(5)}},
		},
		MessageEvents: []trace.MessageEvent{
			{Time: time.Unix(12345, 0), EventType: 2, MessageID: 0x3, UncompressedByteSize: 0x190, CompressedByteSize: 0x12c},
			{Time: time.Unix(12345, 0), EventType: 1, MessageID: 0x1, UncompressedByteSize: 0xc8, CompressedByteSize: 0x64},
		},
	}

}

func TestProcessSpan(t *testing.T) {
	sender := newFake()

	sender.TestData = map[string][]interface{}{}

	sender.TestData["span"] = []interface{}{"span", int64(12345000), int64(55545001), "FakeSource", "00010203-0405-0607-0809-0a0b0c0d0e0f", "00000000-0000-0000-0001-020304050607", []string(nil), []string(nil), []senders.SpanTag{senders.SpanTag{Key: "application", Value: "test-app"}, senders.SpanTag{Key: "cluster", Value: "none"}, senders.SpanTag{Key: "error", Value: "true"}, senders.SpanTag{Key: "error_code", Value: "Unknown"}, senders.SpanTag{Key: "foo1", Value: "bar1"}, senders.SpanTag{Key: "foo2", Value: "5.25"}, senders.SpanTag{Key: "foo3", Value: "42"}, senders.SpanTag{Key: "service", Value: "test-service"}, senders.SpanTag{Key: "shard", Value: "none"}}, []senders.SpanLog{senders.SpanLog{Timestamp: 67890001, Fields: map[string]string{"event": "error", "message": "some error"}}, senders.SpanLog{Timestamp: 12345000, Fields: map[string]string{"key1": "1", "log_msg": "1.500000"}}, senders.SpanLog{Timestamp: 12345000, Fields: map[string]string{"key2": "5", "log_msg": "Annotate"}}, senders.SpanLog{Timestamp: 12345000, Fields: map[string]string{"MsgCompressedByteSize": "300", "MsgID": "3", "MsgType": "received", "MsgUncompressedByteSize": "400"}}, senders.SpanLog{Timestamp: 12345000, Fields: map[string]string{"MsgCompressedByteSize": "100", "MsgID": "1", "MsgType": "sent", "MsgUncompressedByteSize": "200"}}}}
	sender.TestData["span2"] = []interface{}{"span2", int64(12345000), int64(55545001), "FakeSource", "01010203-0405-0607-0809-0a0b0c0d0e0f", "00000000-0000-0000-0809-0a0b0c0d0e00", []string{"00000000-0000-0000-0001-020304050607"}, []string(nil), []senders.SpanTag{senders.SpanTag{Key: "application", Value: "test-app"}, senders.SpanTag{Key: "cluster", Value: "none"}, senders.SpanTag{Key: "error", Value: "true"}, senders.SpanTag{Key: "error_code", Value: "Unknown"}, senders.SpanTag{Key: "foo1", Value: "bar1"}, senders.SpanTag{Key: "foo2", Value: "5.25"}, senders.SpanTag{Key: "foo3", Value: "42"}, senders.SpanTag{Key: "service", Value: "test-service"}, senders.SpanTag{Key: "shard", Value: "none"}}, []senders.SpanLog{senders.SpanLog{Timestamp: 67890001, Fields: map[string]string{"event": "error", "message": "some error"}}, senders.SpanLog{Timestamp: 12345000, Fields: map[string]string{"key1": "1", "log_msg": "1.500000"}}, senders.SpanLog{Timestamp: 12345000, Fields: map[string]string{"key2": "5", "log_msg": "Annotate"}}, senders.SpanLog{Timestamp: 12345000, Fields: map[string]string{"MsgCompressedByteSize": "300", "MsgID": "3", "MsgType": "received", "MsgUncompressedByteSize": "400"}}, senders.SpanLog{Timestamp: 12345000, Fields: map[string]string{"MsgCompressedByteSize": "100", "MsgID": "1", "MsgType": "sent", "MsgUncompressedByteSize": "200"}}}}

	fakeExp, _ := NewExporter(sender, Source("FakeSource"), AppTags(appTags), Granularity(histogram.MINUTE), DisableSelfHealth())

	fakeExp.processSpan(sd1)
	fakeExp.processSpan(sd2)
	fakeExp.Stop()

	assert.True(t, sender.verify())
	assert.EqualValues(t, 0, fakeExp.SenderErrors())
}

func TestProcessView(tt *testing.T) {
	sender := newFake()

	sender.TestData = map[string][]interface{}{}
	sender.TestData["v1"] = []interface{}{"v1", float64(4.5), int64(67890001), "FakeSource", map[string]string{"application": "test-app", "cluster": "none", "service": "test-service", "shard": "none", "unit": "v1unit"}}
	sender.TestData["v2"] = []interface{}{"v2", float64(4), int64(67890001), "FakeSource", map[string]string{"application": "test-app", "cluster": "none", "service": "test-service", "shard": "none", "unit": "v1unit"}}
	sender.TestData["v3"] = []interface{}{"v3", float64(4), int64(67890001), "FakeSource", map[string]string{"application": "test-app", "cluster": "none", "service": "test-service", "shard": "none", "unit": "v3unit"}}
	sender.TestData["v4.max"] = []interface{}{"v4.max", float64(27), int64(67890001), "FakeSource", map[string]string{"application": "test-app", "cluster": "none", "service": "test-service", "shard": "none", "unit": "v4unit"}}
	sender.TestData["v4.mean"] = []interface{}{"v4.mean", float64(18), int64(67890001), "FakeSource", map[string]string{"application": "test-app", "cluster": "none", "service": "test-service", "shard": "none", "unit": "v4unit"}}
	sender.TestData["v4.count"] = []interface{}{"v4.count", float64(2), int64(67890001), "FakeSource", map[string]string{"application": "test-app", "cluster": "none", "service": "test-service", "shard": "none", "unit": "v4unit"}}
	sender.TestData["v4.sumsq"] = []interface{}{"v4.sumsq", float64(0), int64(67890001), "FakeSource", map[string]string{"application": "test-app", "cluster": "none", "service": "test-service", "shard": "none", "unit": "v4unit"}}
	sender.TestData["v4.min"] = []interface{}{"v4.min", float64(9), int64(67890001), "FakeSource", map[string]string{"application": "test-app", "cluster": "none", "service": "test-service", "shard": "none", "unit": "v4unit"}}

	fakeExp, _ := NewExporter(sender, Source("FakeSource"), AppTags(appTags), Granularity(histogram.MINUTE), DisableSelfHealth())

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

	fakeExp.Stop()

	if !sender.verify() || fakeExp.SenderErrors() > 0 {
		tt.Fail()
	}
}

func TestNegativeQueueSize(t *testing.T) {
	sender := newFake()
	errorExp, err := NewExporter(sender, Source("FakeSource"), QueueSize(-10))
	if errorExp != nil || err == nil {
		t.FailNow()
	}
}

func TestQueueErrors(t *testing.T) {
	sender := newFake()
	errorExp, _ := NewExporter(sender, Source("FakeSource"), AppTags(appTags), Granularity(histogram.MINUTE), QueueSize(0), VerboseLogging(), DisableSelfHealth())
	errorExp.reportStart(500 * time.Millisecond)

	errorExp.Stop()
	errorExp.processSpan(sd1)
	errorExp.processView(vd1)
	errorExp.processView(vd4)
	time.Sleep(time.Second)

	assert.EqualValues(t, 1, errorExp.SpansDropped())
	assert.EqualValues(t, 2, errorExp.MetricsDropped())
}

func TestSenderErrors(t *testing.T) {
	errorExp, _ := NewExporter(senderErr, Source("FakeSource"), AppTags(appTags), Granularity(histogram.MINUTE), VerboseLogging(), DisableSelfHealth())
	errorExp.reportStart(500 * time.Millisecond)

	errorExp.processSpan(sd1)
	errorExp.processView(vd1)
	errorExp.processView(vd4)
	time.Sleep(time.Second)
	errorExp.Stop()

	if errorExp.SenderErrors() < 3 {
		t.FailNow()
	}
}

func TestProcessViewRaceCondition(tt *testing.T) {
	sender := newFake()

	fakeExp, _ := NewExporter(sender, Source("FakeSource"), AppTags(appTags), Granularity(histogram.MINUTE), DisableSelfHealth())

	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		fakeExp.processView(vd1)
	}()
	go func() {
		defer wg.Done()
		fakeExp.Stop()
	}()
	wg.Wait()
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
			benchExp.processView(vd4)
		}
	})
}
