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
	Root              bool   `json:"root"`
	Result            string `json:"result"`
	Type              string `json:"type"`
	DurationHistogram `json:"duration.histogram"`
}

type MetricDoc struct {
	Timestamp time.Time `json:"@timestamp"`
	DocCount  int64     `json:"_doc_count"`
	//Labels    struct {
	//	CountryCode string `json:"country_code"`
	//	InEu        string `json:"in_eu"`
	//	City        string `json:"city"`
	//	IP          string `json:"ip"`
	//	Lang        string `json:"lang"`
	//} `json:"labels"`
	Agent struct {
		Name string `json:"name"`
	} `json:"agent"`
	Cloud struct {
		Provider         string `json:"provider"`
		Region           string `json:"region"`
		AvailabilityZone string `json:"availability_zone"`
		Account          struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"account"`
		Machine struct {
			Type string `json:"type"`
		} `json:"machine"`
		Project struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"project"`
	}
	Container struct {
		ID string `json:"id"`
	} `json:"container"`
	Host struct {
		Name     string `json:"name"`
		Hostname string `json:"hostname"`
		OS       struct {
			Platform string `json:"platform"`
		} `json:"os"`
	}
	Event struct {
		Outcome string `json:"outcome"`
	}
	Kubernetes struct {
		Pod struct {
			Name string `json:"name"`
		} `json:"pod,omitempty"`
	}
	Service struct {
		Environment string `json:"environment"`
		Name        string `json:"name"`
		Node        struct {
			Name string `json:"name"`
		} `json:"node"`
		Language struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"language"`
		Runtime struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"runtime"`
		Version string `json:"version"`
	} `json:"service"`
	Transaction `json:"transaction"`
}
