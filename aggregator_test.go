package metricize

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestAggregateSingle(t *testing.T) {
	doc := &MetricDoc{
		Timestamp: time.Time{},
		DocCount:  1,
		Transaction: Transaction{
			Name: "GET /",
			DurationHistogram: DurationHistogram{
				Counts: []int64{1},
				Values: []int64{1},
			},
		},
	}

	key := newTransactionAggregationKey(doc)
	require.NotEmpty(t, key)
	a := NewAggregator(time.Time{})
	require.NoError(t, a.Aggregate(doc))
	require.Contains(t, a.Buckets, key)
	require.Len(t, a.Buckets, 1)
}

func TestAggregateMultiple(t *testing.T) {
	doc := &MetricDoc{
		Timestamp: time.Time{},
		DocCount:  1,
	}
	dists := []struct {
		name           string
		counts, values []int64
	}{
		{name: "foo", counts: []int64{1, 10}, values: []int64{100000, 110000}},
		{name: "foo", counts: []int64{1, 10}, values: []int64{100000, 110000}},
		{name: "bar", counts: []int64{1, 10}, values: []int64{100000, 110000}},
	}

	a := NewAggregator(time.Time{})
	for _, dist := range dists {
		doc.Transaction.Name = dist.name
		doc.Transaction.DurationHistogram.Counts = dist.counts
		doc.Transaction.DurationHistogram.Values = dist.values
		require.NoError(t, a.Aggregate(doc))
	}
	require.Len(t, a.Buckets, 2)
}
