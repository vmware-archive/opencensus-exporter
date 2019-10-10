// Package wavefront provides OpenCensus trace and stats support
// to push metrics, histograms and traces into Wavefront.
package wavefront

import (
	"errors"
	"log"
	"sync"
	"sync/atomic"

	"github.com/wavefronthq/wavefront-sdk-go/application"
	"github.com/wavefronthq/wavefront-sdk-go/histogram"
	"github.com/wavefronthq/wavefront-sdk-go/senders"

	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"
)

const (
	// DefaultQueueSize is used when QueueSize option is not specified
	DefaultQueueSize = 1000

	defaultSource       = ""
	nanoToMillis  int64 = 1e6
)

// Options is all the configurable options
type Options struct {
	Source            string
	Hgs               map[histogram.Granularity]bool
	appMap            map[string]string
	qSize             int
	VerboseLogging    bool
	DisableSelfHealth bool
}

// Option allows customization
type Option func(*Options)

type sendCmd func()

// Source overrides the deault source
func Source(source string) Option {
	return func(o *Options) {
		o.Source = source
	}
}

// Granularity enables specified granularities when
// sending Wavefront histograms
func Granularity(hgs ...histogram.Granularity) Option {
	return func(o *Options) {
		for _, g := range hgs {
			o.Hgs[g] = true
		}
	}
}

// AppTags allows setting Application, Service, etc...
// Shown in Wavefront UI
func AppTags(app application.Tags) Option {
	return func(o *Options) {
		o.appMap = app.Map()
	}
}

// QueueSize sets the maximum number of queued metrics and spans.
// Spans/Metrics are dropped if the Queue is full
func QueueSize(queueSize int) Option {
	return func(o *Options) {
		o.qSize = queueSize
	}
}

// DisableSelfHealth disables sending exporter health metrics
// such as dropped metrics and spans
func DisableSelfHealth() Option {
	return func(o *Options) {
		o.DisableSelfHealth = true
	}
}

// VerboseLogging enables logging of errors per span/metric.
// Logs to stderr or equivalent
func VerboseLogging() Option {
	return func(o *Options) {
		o.VerboseLogging = true
	}
}

// Exporter is the main exporter
type Exporter struct {
	sender senders.Sender

	wg      sync.WaitGroup
	lock    sync.RWMutex
	running bool

	// Embeddings
	Options
	_SelfMetrics
}

// NewExporter returns a trace.Exporter configured to upload traces and views
// to the configured wavefront instance (via Wavefront Sender)
//
// Documentation for Wavefront Sender is available at
// https://github.com/wavefrontHQ/wavefront-sdk-go
//
// Option... add additional options to the exporter.
func NewExporter(sender senders.Sender, option ...Option) (*Exporter, error) {
	defOptions := Options{
		Source: defaultSource,
		Hgs:    map[histogram.Granularity]bool{},
		qSize:  DefaultQueueSize,
	}

	for _, o := range option {
		o(&defOptions)
	}

	if defOptions.qSize < 0 {
		return nil, errors.New("QueueSize cannot be negative")
	}

	exp := &Exporter{
		sender:  sender,
		Options: defOptions,
		running: true,
	}

	if !exp.DisableSelfHealth {
		exp.ReportSelfHealth() // Disable by default?
	}

	return exp, nil
}

// Stop the exporter and flushes the sender
func (e *Exporter) Stop() {
	e.lock.Lock()
	e.running = false
	e.lock.Unlock()
	e.StopSelfHealth()
	e.wg.Wait()
	e.sender.Flush()
}

// ExportSpan exports given span to Wavefront
func (e *Exporter) ExportSpan(spanData *trace.SpanData) {
	e.processSpan(spanData)
}

// ExportView exports given view to Wavefront
func (e *Exporter) ExportView(viewData *view.Data) {
	e.processView(viewData)
}

// Helpers

func (e *Exporter) queueCmd(cmd sendCmd) bool {
	e.lock.RLock()
	defer e.lock.RUnlock()
	if !e.running {
		return false
	}
	e.wg.Add(1)
	go func() {
		cmd()
		e.wg.Done()
	}()
	return true
}

func (e *Exporter) logError(msg string, errs ...error) {
	for ei, err := range errs {
		if err != nil {
			atomic.AddUint64(&e.senderErrors, 1)
			if e.VerboseLogging {
				log.Printf("%s (%d): %s", msg, ei, err)
			}
		}
	}
}
