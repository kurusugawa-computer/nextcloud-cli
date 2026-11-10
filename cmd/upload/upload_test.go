package upload

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kurusugawa-computer/nextcloud-cli/lib/nextcloud"
	"github.com/thamaji/pbpool"
)

type uploadTransport func(*http.Request) (*http.Response, error)

func (f uploadTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func uploadResponse(code int) *http.Response {
	return &http.Response{
		StatusCode: code,
		Status:     fmt.Sprintf("%d %s", code, http.StatusText(code)),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader("")),
	}
}

func TestUploadFragmentRetryProgress(t *testing.T) {
	const content = "0123456789abcdef"
	for _, tc := range []struct {
		name      string
		status    int
		firstRead int64
		offset    int64
		size      int64
		pool      bool
		failAgain bool
	}{
		{name: "partial upload", status: 502, firstRead: 4, size: 16, pool: true},
		{name: "complete upload rejected", status: 413, firstRead: 16, size: 16, pool: true},
		{name: "fragment", status: 502, firstRead: 3, offset: 5, size: 7, pool: true},
		{name: "create directory", status: 404, firstRead: 16, size: 16, pool: true},
		{name: "create directory for fragment", status: 404, firstRead: 7, offset: 5, size: 7, pool: true},
		{name: "retry exhausted", status: 502, firstRead: 16, size: 16, pool: true, failAgain: true},
		{name: "without terminal", status: 502, firstRead: 16, size: 16},
		{name: "create directory without terminal", status: 404, firstRead: 16, size: 16},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := filepath.Join(t.TempDir(), "source")
			if err := ioutil.WriteFile(src, []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			attempts := 0
			var previous *pbpool.ProgressBar
			client := &http.Client{Transport: uploadTransport(func(req *http.Request) (*http.Response, error) {
				switch req.Method {
				case "PROPFIND":
					return uploadResponse(404), nil
				case "MKCOL":
					return uploadResponse(201), nil
				case http.MethodPut:
				default:
					t.Errorf("unexpected method: %s", req.Method)
					return uploadResponse(500), nil
				}
				attempts++
				f := req.Body.(*file)
				if tc.pool {
					if f.bar == nil {
						t.Error("missing progress bar")
					} else {
						if got := f.bar.Get(); got != 0 {
							t.Errorf("attempt %d starts at %d bytes, want 0", attempts, got)
						}
						if f.bar == previous {
							t.Error("retry reused previous progress bar and its timing information")
						}
						if previous != nil && !previous.IsFinished() {
							t.Error("previous progress bar was not finished")
						}
						if f.bar.Total != tc.size {
							t.Errorf("total=%d, want %d", f.bar.Total, tc.size)
						}
					}
				} else if f.bar != nil {
					t.Error("unexpected progress bar without terminal")
				}
				// Hide os.File.WriteTo so all supported Go versions exercise file.Read.
				reader := struct{ io.Reader }{req.Body}
				limit := tc.size
				if attempts == 1 {
					limit = tc.firstRead
				}
				data, err := ioutil.ReadAll(io.LimitReader(reader, limit))
				if err != nil {
					t.Error(err)
				}
				if got, want := string(data), content[tc.offset:tc.offset+limit]; got != want {
					t.Errorf("attempt %d read %q, want %q", attempts, got, want)
				}
				if tc.pool && f.bar != nil {
					if got := f.bar.Get(); got != limit {
						t.Errorf("attempt %d progress=%d, want %d", attempts, got, limit)
					}
				}
				previous = f.bar
				if attempts == 1 || tc.failAgain {
					return uploadResponse(tc.status), nil
				}
				return uploadResponse(201), nil
			})}
			c := &ctx{n: nextcloud.New("http://example.invalid", client, nil), sem: make(chan struct{}, 1), wg: &sync.WaitGroup{}, m: &sync.Mutex{}, retry: 2}
			if tc.pool {
				c.pool = pbpool.New()
			}
			uploadFragment(c, "uploads", src, tc.offset, tc.size, "uploads/source", "source")
			c.wg.Wait()
			if attempts != 2 {
				t.Errorf("attempts=%d, want 2", attempts)
			}
			if (c.err != nil) != tc.failAgain {
				t.Errorf("error=%v, want failure=%t", c.err, tc.failAgain)
			}
			if previous != nil && !previous.IsFinished() {
				t.Error("final progress bar was not finished")
			}
		})
	}
}

func TestUploadFragmentRetryResetsElapsedTime(t *testing.T) {
	src := filepath.Join(t.TempDir(), "source")
	if err := ioutil.WriteFile(src, []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	attempts := 0
	client := &http.Client{Transport: uploadTransport(func(req *http.Request) (*http.Response, error) {
		attempts++
		f := req.Body.(*file)
		if attempts == 2 {
			f.bar.ShowElapsedTime = true
			f.bar.ShowCounters = false
			f.bar.ShowPercent = false
			f.bar.ShowBar = false
			f.bar.ShowTimeLeft = false
			f.bar.Update()
			if fields := strings.Fields(f.bar.String()); len(fields) != 2 || fields[1] != "0s" {
				t.Errorf("retry elapsed time includes previous attempt or delay: %q", f.bar.String())
			}
		}
		_, err := io.Copy(ioutil.Discard, struct{ io.Reader }{req.Body})
		if err != nil {
			t.Error(err)
		}
		if attempts == 1 {
			return uploadResponse(502), nil
		}
		return uploadResponse(201), nil
	})}
	c := &ctx{n: nextcloud.New("http://example.invalid", client, nil), sem: make(chan struct{}, 1), wg: &sync.WaitGroup{}, m: &sync.Mutex{}, pool: pbpool.New(), retry: 2, delay: 1100 * time.Millisecond}
	uploadFragment(c, "uploads", src, 0, 4, "uploads/source", "source")
	c.wg.Wait()
	if c.err != nil {
		t.Fatal(c.err)
	}
	if attempts != 2 {
		t.Errorf("attempts=%d, want 2", attempts)
	}
}
