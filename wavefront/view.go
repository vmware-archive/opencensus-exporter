package wavefront

import (
	"log"
	"sync/atomic"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

const (
	unitTagKey = "unit"

	// Distribution Aggregation metric suffixes
	distMinSuffix   = ".min"
	distMaxSuffix   = ".max"
	distMeanSuffix  = ".mean"
	distCountSuffix = ".count"
	distSumSqSuffix = ".sumsq"
)

// view conversion
func (e *Exporter) processView(vd *view.Data) {
	var cmd sendCmd

	// Custom Tags & App Tags
	appTags := e.appMap
	otherTags := make(map[string]string, 2+len(appTags))
	for k, v := range appTags {
		otherTags[k] = v
	}
	unit := vd.View.Measure.Unit()
	if unit != "" && unit != stats.UnitDimensionless {
		otherTags[unitTagKey] = unit
	}

	for _, row := range vd.Rows {
		pointTags := makePointTags(row.Tags, otherTags)
		timestamp := vd.End.UnixNano() / nanoToMillis

		switch agg := row.Data.(type) {
		case *view.CountData:
			value := float64(agg.Value)
			cmd = func() {
				e.logError("Error sending metric", e.sender.SendMetric(
					vd.View.Name,
					value, timestamp, e.Source,
					pointTags,
				))
			}

		case *view.LastValueData:
			value := agg.Value
			cmd = func() {
				e.logError("Error sending metric", e.sender.SendMetric(
					vd.View.Name,
					value, timestamp, e.Source,
					pointTags,
				))
			}

		case *view.SumData:
			value := agg.Value
			cmd = func() {
				e.logError("Error sending metric:", e.sender.SendMetric(
					vd.View.Name,
					value, timestamp, e.Source,
					pointTags,
				))
			}

		case *view.DistributionData:
			cmd = func() {
				// Output OpenCensus distribution as a set of metrics
				e.logError("Error sending histogram",
					e.sender.SendMetric(vd.View.Name+distCountSuffix, float64(agg.Count), timestamp, e.Source, pointTags),
					e.sender.SendMetric(vd.View.Name+distMinSuffix, agg.Min, timestamp, e.Source, pointTags),
					e.sender.SendMetric(vd.View.Name+distMaxSuffix, agg.Max, timestamp, e.Source, pointTags),
					e.sender.SendMetric(vd.View.Name+distMeanSuffix, agg.Mean, timestamp, e.Source, pointTags),
					e.sender.SendMetric(vd.View.Name+distSumSqSuffix, agg.SumOfSquaredDev, timestamp, e.Source, pointTags))
			}

		default:
			log.Printf("Unsupported Aggregation type: %T", vd.View.Aggregation)
		}

		if !e.queueCmd(cmd) {
			atomic.AddUint64(&e.metricsDropped, 1)
		}
	}
}

func makePointTags(tags []tag.Tag, otherTags map[string]string) map[string]string {
	pointTags := make(map[string]string, len(tags)+len(otherTags))
	for _, t := range tags {
		pointTags[t.Key.Name()] = t.Value
	}
	for k, v := range otherTags { //otherTags first?
		pointTags[k] = v
	}
	return pointTags
}
