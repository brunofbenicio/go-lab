package retryhttp_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"time"

	"github.com/brunofbenicio/go-lab/pkg/retryhttp"
)

func ExampleNew() {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		io.WriteString(w, "hello")
	}))
	defer srv.Close()

	client := &http.Client{Transport: retryhttp.New(srv.Client().Transport,
		retryhttp.WithSleep(func(context.Context, time.Duration) error { return nil }))}
	resp, err := client.Get(srv.URL)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Println(resp.StatusCode, string(body), calls.Load())
	// Output: 200 hello 2
}
