package core

import "crypto/rand"

type Pair struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Request struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Method  string `json:"method"`
	URL     string `json:"url"`
	Headers []Pair `json:"headers,omitempty"`
	Query   []Pair `json:"query,omitempty"`
	Body    string `json:"body,omitempty"`
}

type Environment struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Variables map[string]string `json:"variables"`
}

type State struct {
	Version           int           `json:"version"`
	Requests          []Request     `json:"requests"`
	Environments      []Environment `json:"environments"`
	ActiveEnvironment string        `json:"active_environment,omitempty"`
	ActiveRequest     string        `json:"active_request,omitempty"`
}

func NewRequest() Request {
	return Request{ID: rand.Text(), Name: "New request", Method: "GET"}
}
