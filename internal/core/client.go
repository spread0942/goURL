package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const MaxBody = 5 * 1024 * 1024

var variablePattern = regexp.MustCompile(`\{\{\s*([^{}]+?)\s*\}\}`)

func Resolve(source Request, variables map[string]string) (Request, error) {
	missing := map[string]bool{}
	expand := func(value string) string {
		return variablePattern.ReplaceAllStringFunc(value, func(match string) string {
			name := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(match, "{{"), "}}"))
			value, exists := variables[name]
			if !exists {
				missing[name] = true
			}
			return value
		})
	}
	source.URL = expand(source.URL)
	source.Body = expand(source.Body)
	source.Headers = slices.Clone(source.Headers)
	source.Query = slices.Clone(source.Query)
	for _, pairs := range [][]Pair{source.Headers, source.Query} {
		for index := range pairs {
			pairs[index].Key = expand(pairs[index].Key)
			pairs[index].Value = expand(pairs[index].Value)
		}
	}
	if len(missing) > 0 {
		names := make([]string, 0, len(missing))
		for name := range missing {
			names = append(names, name)
		}
		slices.Sort(names)
		return Request{}, fmt.Errorf("missing variables: %s", strings.Join(names, ", "))
	}
	return source, nil
}

type Response struct {
	Status    string
	Headers   http.Header
	Body      []byte
	Duration  time.Duration
	Truncated bool
}

func Execute(ctx context.Context, client *http.Client, source Request) (Response, error) {
	var result Response
	request, err := http.NewRequestWithContext(ctx, source.Method, source.URL, strings.NewReader(source.Body))
	if err != nil {
		return result, fmt.Errorf("invalid request: %w", err)
	}
	if (request.URL.Scheme != "http" && request.URL.Scheme != "https") || request.URL.Hostname() == "" {
		return result, fmt.Errorf("URL must have an http:// or https:// host")
	}
	query, err := queryValues(request.URL.RawQuery)
	if err != nil {
		return result, err
	}
	for _, pair := range source.Query {
		query.Add(pair.Key, pair.Value)
	}
	request.URL.RawQuery = query.Encode()
	for _, pair := range source.Headers {
		if strings.EqualFold(pair.Key, "Host") {
			request.Host = pair.Value
		} else {
			request.Header.Add(pair.Key, pair.Value)
		}
	}
	start := time.Now()
	response, err := client.Do(request)
	if err != nil {
		return result, err
	}
	defer response.Body.Close()
	result.Status = response.Status
	result.Headers = response.Header.Clone()
	result.Body, err = io.ReadAll(io.LimitReader(response.Body, MaxBody+1))
	result.Duration = time.Since(start)
	if len(result.Body) > MaxBody {
		result.Body = result.Body[:MaxBody]
		result.Truncated = true
	}
	return result, err
}

func DisplayBody(body []byte) string {
	if !utf8.Valid(body) || bytes.IndexByte(body, 0) >= 0 {
		preview := body[:min(len(body), 256)]
		return fmt.Sprintf("Binary response (%d bytes); first %d bytes:\n%x", len(body), len(preview), preview)
	}
	var formatted bytes.Buffer
	if json.Indent(&formatted, body, "", "  ") == nil {
		body = formatted.Bytes()
	}
	return SafeText(string(body))
}

func SafeText(value string) string {
	return strings.Map(func(character rune) rune {
		if unicode.IsControl(character) && character != '\n' && character != '\t' {
			return -1
		}
		return character
	}, value)
}
