package core

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestResolveAndExecute(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		if request.Method != "POST" || string(body) != `{"text":"hello"}` || request.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("unexpected request: %s, %s, %s", request.Method, body, request.Header.Get("Authorization"))
		}
		if values := request.URL.Query()["tag"]; !reflect.DeepEqual(values, []string{"first", "a & b"}) {
			t.Errorf("query values: %v", values)
		}
		writer.WriteHeader(http.StatusTeapot)
		io.WriteString(writer, `{"ok":true}`)
	}))
	defer server.Close()
	source := Request{Method: "POST", URL: "{{ base }}/test?tag=first", Body: `{"text":"{{text}}"}`, Headers: []Pair{{"Authorization", "Bearer {{token}}"}}, Query: []Pair{{"tag", "{{tag}}"}}}
	resolved, err := Resolve(source, map[string]string{"base": server.URL, "text": "hello", "token": "secret", "tag": "a & b"})
	if err != nil {
		t.Fatal(err)
	}
	if source.Headers[0].Value != "Bearer {{token}}" || source.Query[0].Value != "{{tag}}" {
		t.Fatal("modified original template")
	}
	response, err := Execute(context.Background(), server.Client(), resolved)
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != "418 I'm a teapot" || !strings.Contains(DisplayBody(response.Body), "\n") {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestVariables(t *testing.T) {
	for _, item := range []struct {
		name, input, want string
		values            map[string]string
		fail              bool
	}{
		{"empty", "{{ empty }}", "", map[string]string{"empty": ""}, false},
		{"repeated", "{{key}}/{{key}}", "value/value", map[string]string{"key": "value"}, false},
		{"nonrecursive", "{{key}}", "{{other}}", map[string]string{"key": "{{other}}"}, false},
		{"missing", "{{absent}}", "", nil, true},
	} {
		t.Run(item.name, func(t *testing.T) {
			result, err := Resolve(Request{URL: item.input}, item.values)
			if (err != nil) != item.fail || (!item.fail && result.URL != item.want) {
				t.Fatalf("got %q, %v", result.URL, err)
			}
		})
	}
}

func TestCancelAndLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/wait" {
			<-request.Context().Done()
			return
		}
		io.WriteString(writer, strings.Repeat("x", MaxBody+20))
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Execute(ctx, server.Client(), Request{Method: "GET", URL: server.URL}); err == nil {
		t.Fatal("expected cancellation")
	}
	client := &http.Client{Timeout: 20 * time.Millisecond}
	if _, err := Execute(context.Background(), client, Request{Method: "GET", URL: server.URL + "/wait"}); err == nil {
		t.Fatal("expected timeout")
	}
	result, err := Execute(context.Background(), server.Client(), Request{Method: "GET", URL: server.URL})
	if err != nil || !result.Truncated || len(result.Body) != MaxBody {
		t.Fatalf("limit: %d, %v, %v", len(result.Body), result.Truncated, err)
	}
	if _, err := Execute(context.Background(), client, Request{URL: "file:///etc/passwd"}); err == nil {
		t.Fatal("expected invalid URL")
	}
}

func TestStore(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "private")
	state, err := Load(dir)
	if err != nil || state.Version != 1 {
		t.Fatalf("new state: %v", err)
	}
	state.Requests = []Request{NewRequest()}
	state.Environments = []Environment{{ID: "env", Name: "dev", Variables: map[string]string{"token": "secret"}}}
	if err := Save(dir, state); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(dir)
	if err != nil || !reflect.DeepEqual(state, loaded) {
		t.Fatalf("round trip: %v", err)
	}
	for _, item := range []struct {
		path string
		mode os.FileMode
	}{{dir, 0700}, {filepath.Join(dir, "state.json"), 0600}} {
		info, err := os.Stat(item.path)
		if err != nil || info.Mode().Perm() != item.mode {
			t.Fatalf("permissions on %s: %v", item.path, err)
		}
	}
	for _, data := range []string{`broken`, `{"version":99}`, `{}`, `null`} {
		if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(dir); err == nil {
			t.Fatal("expected load error")
		}
	}
	if err := Save(filepath.Join(dir, "state.json"), state); err == nil {
		t.Fatal("expected write failure")
	}
}

func TestPairsAndDisplay(t *testing.T) {
	pairs, err := ParsePairs("tag=a=b\ntag=two", "=", false)
	if err != nil || len(pairs) != 2 || pairs[0].Value != "a=b" {
		t.Fatalf("pairs: %v %v", pairs, err)
	}
	if _, err := ParsePairs("key=one\nkey=two", "=", true); err == nil {
		t.Fatal("expected duplicate error")
	}
	if _, err := ParsePairs("missing", "=", false); err == nil {
		t.Fatal("expected syntax error")
	}
	if SafeText("a\x1b\x00b") != "ab" {
		t.Fatal("unsafe controls")
	}
	if !strings.HasPrefix(DisplayBody([]byte{0, 1, 2}), "Binary response") {
		t.Fatal("expected binary preview")
	}
}
