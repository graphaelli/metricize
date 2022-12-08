package metricize

import (
	"time"
)

type DurationHistogram struct {
	Counts []int64 `json:"counts"`
	Values []int64 `json:"values"`
}

type Transaction struct {
	Name              string `json:"name"`
	Root              bool   `json:"root,omitempty"`
	Result            string `json:"result,omitempty"`
	Type              string `json:"type"`
	DurationHistogram `json:"duration.histogram"`
}

type MetricDoc struct {
	Timestamp time.Time `json:"@timestamp"`
	DocCount  int64     `json:"_doc_count,omitempty"`
	//Labels    struct {
	//	CountryCode string `json:"country_code"`
	//	InEu        string `json:"in_eu"`
	//	City        string `json:"city"`
	//	IP          string `json:"ip"`
	//	Lang        string `json:"lang"`
	//} `json:"labels"`
	Agent struct {
		Name string `json:"name,omitempty"`
	} `json:"agent,omitempty"`
	Cloud struct {
		Provider         string `json:"provider,omitempty"`
		Region           string `json:"region,omitempty"`
		AvailabilityZone string `json:"availability_zone,omitempty"`
		Account          struct {
			ID   string `json:"id,omitempty"`
			Name string `json:"name,omitempty"`
		} `json:"account,omitempty"`
		Machine struct {
			Type string `json:"type,omitempty"`
		} `json:"machine,omitempty"`
		Project struct {
			ID   string `json:"id,omitempty"`
			Name string `json:"name,omitempty"`
		} `json:"project,omitempty"`
	} `json:"cloud,omitempty"`
	Container struct {
		ID string `json:"id,omitempty"`
	} `json:"container,omitempty"`
	Host struct {
		Name     string `json:"name,omitempty"`
		Hostname string `json:"hostname,omitempty"`
		OS       struct {
			Platform string `json:"platform,omitempty"`
		} `json:"os,omitempty"`
	} `json:"host,omitempty"`
	Event struct {
		Outcome string `json:"outcome,omitempty"`
	} `json:"event,omitempty"`
	Kubernetes struct {
		Pod struct {
			Name string `json:"name,omitempty"`
		} `json:"pod,omitempty,omitempty"`
	} `json:"kubernetes,omitempty"`
	Metricset struct {
		Name string `json:"name"`
	} `json:"metricset"`
	NumericLabels struct {
		Period int64 `json:"rollup_period"`
	} `json:"numeric_labels"`
	Observer struct {
		Version string `json:"version"`
	} `json:"observer"`
	Service struct {
		Environment string `json:"environment,omitempty"`
		Name        string `json:"name,omitempty"`
		Node        struct {
			Name string `json:"name,omitempty"`
		} `json:"node,omitempty"`
		Language struct {
			Name    string `json:"name,omitempty"`
			Version string `json:"version,omitempty"`
		} `json:"language,omitempty"`
		Runtime struct {
			Name    string `json:"name,omitempty"`
			Version string `json:"version,omitempty"`
		} `json:"runtime,omitempty"`
		Version string `json:"version,omitempty"`
	} `json:"service,omitempty"`
	Transaction `json:"transaction"`
}
