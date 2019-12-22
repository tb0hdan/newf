package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/mux"
)

// Handler - HTTP handler with bound methods
type Handler struct {
	client   http.Client
	upstream string
}

// New - create new HTTP handler
func New(upstream string) *Handler {
	_, port, err := net.SplitHostPort(upstream)
	if err != nil && !strings.HasSuffix(err.Error(), "missing port in address") {
		panic(err)
	}

	if len(port) == 0 {
		upstream += ":80"
	}

	return &Handler{
		client: http.Client{
			Timeout: 1 * time.Hour,
		},
		upstream: upstream,
	}
}

func (h *Handler) CopyHeaders(src http.Header, dst http.Header) {
	for s := range src {
		if len(s) == 0 {
			fmt.Println("Req key empty!")
			continue
		}

		v := src.Get(s)

		if len(v) == 0 {
			fmt.Println("Req val empty!")
			continue
		}
		// Skip Connection header: https://golang.org/src/net/http/httputil/reverseproxy.go:212
		if s == "Connection" {
			continue
		}
		// Skip accept-encoding as we don't support gzip yet
		// gzip.NewReader(r)
		if s == "Accept-Encoding" {
			continue
		}

		dst.Set(s, v)
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var (
		requestURI string
	)

	parsed, err := url.Parse(r.Referer())

	if err != nil {
		panic(err)
	}

	if strings.HasSuffix(parsed.Path, "/") {
		requestURI = r.URL.Path
	} else {
		requestURI = parsed.Path + r.URL.Path
	}

	remoteURL := fmt.Sprintf("http://%s%s", h.upstream, requestURI)

	fmt.Println(r.RemoteAddr, remoteURL)

	URL, err := url.Parse(remoteURL)
	if err != nil {
		panic(err)
	}

	req := &http.Request{Method: r.Method, URL: URL}
	req.Header = make(map[string][]string)

	// Copy request headers
	h.CopyHeaders(r.Header, req.Header)

	resp, err := h.client.Do(req)

	if err != nil {
		panic(err)
	} else {
		defer resp.Body.Close()
	}

	// Copy response headers
	h.CopyHeaders(resp.Header, w.Header())
	w.WriteHeader(resp.StatusCode)

	written, err := io.Copy(w, resp.Body)
	if err != nil {
		fmt.Println(err, written)
	}
}

type NewfSecurityProxy struct {
	Gorilla *mux.Router
}

type NewfAPI struct {

}

func (n *NewfAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Welcome to Newf!")
}

func NewNewf() (api *NewfAPI) {
	return &NewfAPI{}
}


func NewSecurityProxy(upstream string) (gorilla *mux.Router){
	handler := New(upstream)
	newf := NewNewf()
	proxy := &NewfSecurityProxy{Gorilla: mux.NewRouter()}
	proxy.Gorilla.PathPrefix("/newf").HandlerFunc(newf.ServeHTTP)
	proxy.Gorilla.PathPrefix("/").HandlerFunc(handler.ServeHTTP)
	return proxy.Gorilla
}
func main() {
	upstream := flag.String("upstream", "", "HTTP upstream, e.g. 192.168.3.1:81 or just 192.168.3.1")
	bind := flag.String("bind", "0.0.0.0:8000", "Bind addr, e.g. 0.0.0.0:8000")
	flag.Parse()

	if *upstream == "" {
		fmt.Println("upstream cannot be empty")
		os.Exit(1)
	}

	proxy := NewSecurityProxy(*upstream)

	srv := http.Server{
		Addr:         *bind,
		Handler:      proxy,
		ReadTimeout:  35 * time.Second,
		WriteTimeout: 65 * time.Second,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic(fmt.Sprintf("Listen failed with: %v\n", err))
		}
	}()
	<-done

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)

	defer func() {
		cancel()
	}()

	if err := srv.Shutdown(ctx); err != nil {
		panic(fmt.Sprintf("Couldn't perform shutdown:%+v", err))
	}
}
