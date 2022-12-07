package metricize

import (
	"github.com/elastic/go-hdrhistogram"
	"time"
)

const (
	minDuration                    time.Duration = 0
	maxDuration                    time.Duration = time.Hour
	hdrHistogramSignificantFigures               = 2
)

type transactionMetrics struct {
	earliest, latest time.Time
	hist             *hdrhistogram.Histogram
}

/*
apm-server does:

func (m *transactionMetrics) histogramBuckets() (totalCount int64, counts []int64, values []float64) {
	// From https://www.elastic.co/guide/en/elasticsearch/reference/current/histogram.html:
	//
	// "For the High Dynamic Range (HDR) histogram mode, the values array represents
	// fixed upper limits of each bucket interval, and the counts array represents
	// the number of values that are attributed to each interval."
	distribution := m.histogram.Distribution()
	counts = make([]int64, 0, len(distribution))
	values = make([]float64, 0, len(distribution))
	for _, b := range distribution {
		if b.Count <= 0 {
			continue
		}
		count := int64(math.Round(float64(b.Count) / histogramCountScale))
		counts = append(counts, count)
		values = append(values, float64(b.To))
		totalCount += count
	}
	return totalCount, counts, values
}

*/
func (t *transactionMetrics) Emit() DurationHistogram {
	dist := t.hist.Distribution()
	counts := make([]int64, len(dist))
	values := make([]int64, len(dist))
	for i, bar := range dist {
		counts[i] = bar.Count
		values[i] = bar.To
	}
	return DurationHistogram{
		Counts: counts,
		Values: values,
	}
}

type aggregator struct {
	start   time.Time
	buckets map[transactionAggregationKey]*transactionMetrics
}

type aggKeyDims struct {
	faasColdstart          *bool
	faasID                 string
	faasName               string
	faasVersion            string
	agentName              string
	hostOSPlatform         string
	kubernetesPodName      string
	cloudProvider          string
	cloudRegion            string
	cloudAvailabilityZone  string
	cloudServiceName       string
	cloudAccountID         string
	cloudAccountName       string
	cloudMachineType       string
	cloudProjectID         string
	cloudProjectName       string
	serviceEnvironment     string
	serviceName            string
	serviceVersion         string
	serviceNodeName        string
	serviceRuntimeName     string
	serviceRuntimeVersion  string
	serviceLanguageName    string
	serviceLanguageVersion string
	transactionName        string
	transactionResult      string
	transactionType        string
	eventOutcome           string
	faasTriggerType        string
	hostHostname           string
	hostName               string
	containerID            string
	traceRoot              bool
}

type transactionAggregationKey struct {
	//labels.AggregatedGlobalLabels
	aggKeyDims
}

func newTransactionAggregationKey(m *MetricDoc) transactionAggregationKey {
	return transactionAggregationKey{
		aggKeyDims: aggKeyDims{
			faasColdstart:          nil,
			faasID:                 "",
			faasName:               "",
			faasVersion:            "",
			agentName:              m.Agent.Name,
			hostOSPlatform:         m.Host.OS.Platform,
			kubernetesPodName:      m.Kubernetes.Pod.Name,
			cloudProvider:          m.Cloud.Provider,
			cloudRegion:            m.Cloud.Region,
			cloudAvailabilityZone:  m.Cloud.AvailabilityZone,
			cloudServiceName:       "",
			cloudAccountID:         m.Cloud.Account.ID,
			cloudAccountName:       m.Cloud.Account.Name,
			cloudMachineType:       m.Cloud.Machine.Type,
			cloudProjectID:         m.Cloud.Project.ID,
			cloudProjectName:       m.Cloud.Project.Name,
			serviceEnvironment:     m.Service.Environment,
			serviceName:            m.Service.Name,
			serviceVersion:         m.Service.Version,
			serviceNodeName:        m.Service.Node.Name,
			serviceRuntimeName:     m.Service.Runtime.Name,
			serviceRuntimeVersion:  m.Service.Runtime.Version,
			serviceLanguageName:    m.Service.Language.Name,
			serviceLanguageVersion: m.Service.Language.Version,
			transactionName:        m.Transaction.Name,
			transactionResult:      m.Transaction.Result,
			transactionType:        m.Transaction.Type,
			eventOutcome:           m.Event.Outcome,
			faasTriggerType:        "",
			hostHostname:           m.Host.Name,
			hostName:               m.Host.Hostname,
			containerID:            m.Container.ID,
			traceRoot:              m.Transaction.Root,
		},
	}
}

func (t *transactionAggregationKey) Emit(start time.Time, dh DurationHistogram) MetricDoc {
	m := MetricDoc{Timestamp: start}

	m.Agent.Name = t.agentName
	m.Cloud.Provider = t.cloudProvider
	m.Cloud.Region = t.cloudRegion
	m.Cloud.AvailabilityZone = t.cloudAvailabilityZone
	m.Cloud.Account.ID = t.cloudAccountID
	m.Cloud.Account.Name = t.cloudAccountName
	m.Cloud.Machine.Type = t.cloudMachineType
	m.Cloud.Project.ID = t.cloudProjectID
	m.Cloud.Project.Name = t.cloudProjectName
	m.Container.ID = t.containerID
	m.Kubernetes.Pod.Name = t.kubernetesPodName
	m.Service.Environment = t.serviceEnvironment
	m.Service.Name = t.serviceName
	m.Service.Version = t.serviceVersion
	m.Service.Node.Name = t.serviceNodeName
	m.Service.Runtime.Name = t.serviceRuntimeName
	m.Service.Runtime.Version = t.serviceRuntimeVersion
	m.Service.Language.Name = t.serviceLanguageName
	m.Service.Language.Version = t.serviceLanguageVersion
	m.Event.Outcome = t.eventOutcome
	m.Transaction = Transaction{
		Name:              t.transactionName,
		Result:            t.transactionResult,
		Root:              t.traceRoot,
		Type:              t.transactionType,
		DurationHistogram: dh,
	}
	m.Host.Name = t.hostName
	m.Host.Hostname = t.hostHostname
	m.Host.OS.Platform = t.hostOSPlatform

	return m
}

func newAggregator(start time.Time) *aggregator {
	return &aggregator{start: start, buckets: make(map[transactionAggregationKey]*transactionMetrics)}
}

func (a *aggregator) aggregate(doc *MetricDoc) error {
	key := newTransactionAggregationKey(doc)
	bucket, ok := a.buckets[key]
	if !ok {
		bucket = &transactionMetrics{
			earliest: doc.Timestamp,
			latest:   doc.Timestamp,
			hist: hdrhistogram.New(
				minDuration.Microseconds(),
				maxDuration.Microseconds(),
				hdrHistogramSignificantFigures,
			),
		}
		a.buckets[key] = bucket
	} else {
		if doc.Timestamp.Before(bucket.earliest) {
			bucket.earliest = doc.Timestamp
		}
		if doc.Timestamp.After(bucket.latest) {
			bucket.latest = doc.Timestamp
		}
	}
	for i, v := range doc.Transaction.DurationHistogram.Values {
		if err := bucket.hist.RecordValues(v, doc.Transaction.DurationHistogram.Counts[i]); err != nil {
			return err
		}
	}
	return nil
}

func (a *aggregator) Emit(key transactionAggregationKey) MetricDoc {
	return key.Emit(a.start, a.buckets[key].Emit())
}
