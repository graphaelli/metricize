package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	esv8 "github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/elastic/go-elasticsearch/v8/esutil"

	"github.com/graphaelli/metricize"
)

func minTime(ctx context.Context, es *esv8.Client, index string) (float64, error) {
	q := `{
  "size": 0,
  "aggs": {
    "start": {
      "min": {
        "field": "@timestamp"
      }
    }
  }
}`
	rsp, err := es.Search(
		es.Search.WithContext(ctx),
		es.Search.WithBody(strings.NewReader(q)),
		es.Search.WithIndex(index),
		es.Search.WithTrackTotalHits(true),
	)
	if err != nil {
		return 0, err
	}
	defer rsp.Body.Close()
	if rsp.IsError() {
		return 0, errors.New(rsp.String())
	}
	var result struct {
		Hits struct {
			Total struct {
				Value int `json:"value"`
			} `json:"total"`
		} `json:"hits"`
		Aggregations struct {
			Start struct {
				Value         json.Number `json:"value"`
				ValueAsString string      `json:"value_as_string"`
			} `json:"start"`
		} `json:"aggregations"`
	}
	if err := json.NewDecoder(rsp.Body).Decode(&result); err != nil {
		return 0, err
	}
	if result.Hits.Total.Value == 0 {
		return 0, fmt.Errorf("no metrics found in %q", index)
	}
	return result.Aggregations.Start.Value.Float64()
}

func rollup(ctx context.Context, es *esv8.Client, index string, start, end int64) (*metricize.Aggregator, error) {
	pitKeepAlive := "5m"

	rsp, err := es.OpenPointInTime(
		strings.Split(index, ","),
		pitKeepAlive,
		es.OpenPointInTime.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("while creating PIT: %w", err)
	}
	if rsp.IsError() {
		return nil, fmt.Errorf("while creating PIT: %s", rsp.String())
	}
	var pit struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(rsp.Body).Decode(&pit); err != nil {
		return nil, fmt.Errorf("while parsing PIT response: %w", err)
	}
	defer func() {
		if _, err := es.ClosePointInTime(
			es.ClosePointInTime.WithContext(ctx),
			es.ClosePointInTime.WithBody(strings.NewReader(`{"id":"`+pit.ID+`"}`)),
		); err != nil {
			log.Println("failed to close PIT: ", err)
		}
	}()

	q := struct {
		Size  int `json:"size"`
		Query struct {
			Bool struct {
				Filter []map[string]interface{} `json:"filter"`
			} `json:"bool"`
		} `json:"query"`
		PIT         map[string]interface{}   `json:"pit,omitempty"`
		SearchAfter []interface{}            `json:"search_after,omitempty"`
		Sort        []map[string]interface{} `json:"sort"`
	}{}

	q.Query.Bool.Filter = []map[string]interface{}{
		{
			"range": map[string]interface{}{
				"@timestamp": map[string]interface{}{
					"gte": start * 1000,
					"lt":  end * 1000,
				},
			},
		},
		{
			"term": map[string]interface{}{
				"metricset.name": "transaction",
			},
		},
	}

	q.Size = 100
	q.Sort = []map[string]interface{}{{
		"@timestamp": map[string]interface{}{
			"order": "asc",
		},
	}}
	q.PIT = map[string]interface{}{
		"id":         pit.ID,
		"keep_alive": pitKeepAlive,
	}
	a := metricize.NewAggregator(time.Unix(start, 0))
	for {
		body := esutil.NewJSONReader(q)
		rsp, err = es.Search(
			es.Search.WithBody(body),
		)
		if err != nil {
			return nil, fmt.Errorf("while searching with pagination query: %w", err)
		}
		if rsp.IsError() {
			return nil, fmt.Errorf("while searching with pagingation query: %s", rsp.String())
		}

		var result struct {
			PITID string `json:"pit_id"`
			Hits  struct {
				Hits []struct {
					Source metricize.MetricDoc `json:"_source"`
					Sort   []interface{}       `json:"sort"`
				} `json:"hits"`
			} `json:"hits"`
		}
		defer rsp.Body.Close()
		if err := json.NewDecoder(rsp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("while decoding pagination query: %w", err)
		}
		if len(result.Hits.Hits) == 0 {
			break
		}

		var lastSort []interface{}
		for _, d := range result.Hits.Hits {
			if err := a.Aggregate(&d.Source); err != nil {
				return nil, fmt.Errorf("while aggregating %+v: %w", d, err)
			}
			lastSort = d.Sort
		}

		q.SearchAfter = lastSort
		q.PIT = map[string]interface{}{
			"id":         result.PITID,
			"keep_alive": pitKeepAlive,
		}
	}
	return a, nil
}

func main() {
	log.Default().SetFlags(log.Ldate | log.Ltime | log.Llongfile)
	start := flag.String("start", "", "start time, now: "+time.Now().UTC().Format(time.RFC3339))
	end := flag.String("end", time.Now().UTC().Format(time.RFC3339), "end time")
	interval := flag.Duration("i", 10*time.Minute, "rollup interval size, defaults to 10m (for 10 minutes)")
	index := flag.String("index", "metrics-apm*", "Elasticsearch Index")
	skipTLSVerify := flag.Bool("k", false, "InsecureSkipVerify")
	//pitKeepAlive := flag.String("keep-alive", "5m", "PIT keep alive duration")
	flag.Parse()

	var esConfig esv8.Config
	esConfig.APIKey = os.Getenv("ELASTICSEARCH_API_KEY")
	esConfig.Transport = http.DefaultTransport
	if *skipTLSVerify {
		esConfig.Transport.(*http.Transport).TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}
	es, err := esv8.NewClient(esConfig)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	targetIndex := "metrics-apm.internal-rollup" + interval.String()
	if err := createIndex(ctx, es, targetIndex); err != nil {
		log.Fatal(err)
	}

	// figure out what time to start
	var startSec float64
	if *start != "" {
		t, err := time.Parse(time.RFC3339, *start)
		if err != nil {
			log.Fatal(err)
		}
		startSec = float64(t.Unix())
	} else {
		mt, err := minTime(ctx, es, *index)
		if err != nil {
			log.Fatal(err)
		}
		startSec = mt / 1000
	}
	startBucket := int(math.Floor(startSec/interval.Seconds()) * interval.Seconds())

	// required end time
	var endSec int64 = math.MaxInt
	t, err := time.Parse(time.RFC3339, *end)
	if err != nil {
		log.Fatal(err)
	}
	endSec = t.Unix()

	bucket := int64(startBucket)
	step := int64(interval.Seconds())
	for bucket < endSec {
		log.Printf("rolling up %s", time.Unix(bucket, 0).String())
		a, err := rollup(ctx, es, *index, bucket, bucket+step)
		if err != nil {
			log.Fatal(err)
		}
		if err := emitRollup(ctx, es, targetIndex, step, a); err != nil {
			log.Fatal(err)
		}
		bucket += step
	}
}

func createIndex(ctx context.Context, es *esv8.Client, targetIndex string) error {
	response, err := es.Indices.GetDataStream(
		es.Indices.GetDataStream.WithContext(ctx),
		es.Indices.GetDataStream.WithName(targetIndex),
	)
	if err != nil {
		return err
	}
	if response.StatusCode == http.StatusNotFound {
		response.Body.Close()
		response, err := es.Indices.CreateDataStream(targetIndex, es.Indices.CreateDataStream.WithContext(ctx))
		if err != nil {
			return err
		}
		if response.IsError() {
			io.Copy(os.Stderr, response.Body)
			response.Body.Close()
			return errors.New("creating index mapping failed")
		}
		response.Body.Close()
	} else if response.IsError() {
		io.Copy(os.Stderr, response.Body)
		response.Body.Close()
		return errors.New("creating index mapping failed")
	} else {
		response.Body.Close()
	}
	return nil
}

// copied almost wholesale from https://github.com/axw/metricate/
func emitRollup(ctx context.Context, es *esv8.Client, targetIndex string, period int64, a *metricize.Aggregator) error {
	var buf bytes.Buffer
	doBulkRequest := func() error {
		if buf.Len() == 0 {
			return nil
		}
		response, err := esapi.BulkRequest{
			Index: targetIndex,
			Body:  &buf,
		}.Do(ctx, es)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		if response.IsError() {
			io.Copy(os.Stderr, response.Body)
			return errors.New("bulk indexing failed")
		}
		var result struct {
			Errors bool            `json:"errors"`
			Items  json.RawMessage `json:"items"`
		}
		json.NewDecoder(response.Body).Decode(&result)
		// why isn't this a non-200 and triggered by response.IsError() ?
		if result.Errors {
			return fmt.Errorf("bulk indexing failed with: %s", string(result.Items))
		}
		return nil
	}
	const limit = 512 * 1024 // 512KiB limit for bulk request body
	var ndocs int
	enc := json.NewEncoder(&buf)
	for key, _ := range a.Buckets {
		doc := a.Emit(key)
		doc.Metricset.Name = "transaction_rollup"
		doc.NumericLabels.Period = period
		doc.Observer.Version = "8.5.2"
		fmt.Fprintf(&buf, `{"create": {}}`+"\n")
		ndocs++
		if err := enc.Encode(&doc); err != nil {
			return err
		}
		if buf.Len() >= limit {
			if err := doBulkRequest(); err != nil {
				return err
			}
			buf.Reset()
		}
	}
	if err := doBulkRequest(); err != nil {
		return err
	}
	log.Printf("Indexed %d metrics docs into %s", ndocs, targetIndex)
	return nil
}
