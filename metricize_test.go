package metricize

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAggregateMetricDocs(t *testing.T) {
	doc1 := []byte(`
{
	"@timestamp": "2022-12-07T03:15:00.000Z",
	"_doc_count": 1,
	"labels": {
		"country_code": "US",
		"in_eu": "false",
		"city": "conroe",
		"ip": "98.96.68.28",
		"lang": "en-US,en;q=0.9"
	},
	"service": {
		"environment": "https://www.elastic.co",
		"name": "elastic-co-frontend",
		"language": {
		  "name": "javascript"
		},
		"version": "1.0.0"
	},
	"transaction": {
		"root": true,
		"name": "POST /event",
		"type": "http-request",
		"duration.histogram": {
			"counts": [
				1
			],
			"values": [
				3
			]
		}
	}
}`)

	doc2 := []byte(`
{
	"@timestamp": "2022-12-07T03:15:00.000Z",
	"_doc_count": 1,
	"labels": {
		"country_code": "US",
		"in_eu": "false",
		"city": "conroe",
		"ip": "98.96.68.28",
		"lang": "en-US,en;q=0.9"
	},
	"service": {
		"environment": "https://www.elastic.co",
		"name": "elastic-co-frontend",
		"language": {
		  "name": "javascript"
		},
		"version": "1.0.0"
	},
	"transaction": {
		"root": true,
		"name": "POST /event",
		"type": "http-request",
		"duration.histogram": {
			"counts": [
				1
			],
			"values": [
				3
			]
		}
	}
}`)
	var ms1, ms2 MetricDoc
	require.NoError(t, json.Unmarshal(doc1, &ms1))
	require.NoError(t, json.Unmarshal(doc2, &ms2))

	a := NewAggregator(time.Time{})
	require.NoError(t, a.Aggregate(&ms1))
	require.Len(t, a.Buckets, 1)
	require.NoError(t, a.Aggregate(&ms2))
	require.Len(t, a.Buckets, 1)

	expectedMetricDoc := ms1
	expectedMetricDoc.Timestamp = time.Time{}
	expectedMetricDoc.DocCount = 0
	expectedMetricDoc.DurationHistogram = DurationHistogram{
		Counts: []int64{0, 0, 0, 2},
		Values: []int64{0, 1, 2, 3},
	}

	key := newTransactionAggregationKey(&ms1)
	require.Equal(t, expectedMetricDoc, a.Emit(key))
}
