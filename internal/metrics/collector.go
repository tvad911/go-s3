package metrics

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
)

var (
	// Counters
	RequestsTotal        sync.Map // key: "method|bucket|status" -> *atomic.Uint64
	RequestDurationTotal sync.Map // key: "method|bucket" -> *atomic.Uint64 (stores duration in milliseconds to preserve precision, exported as seconds)
	BytesUploadedTotal   sync.Map // key: "bucket" -> *atomic.Uint64
	BytesDownloadedTotal sync.Map // key: "bucket" -> *atomic.Uint64

	// Gauges
	ObjectsTotal sync.Map // key: "bucket" -> *atomic.Int64
	StorageBytes sync.Map // key: "bucket" -> *atomic.Int64
)

func getCounter(m *sync.Map, key string) *atomic.Uint64 {
	v, ok := m.Load(key)
	if ok {
		return v.(*atomic.Uint64)
	}
	newCounter := &atomic.Uint64{}
	actual, _ := m.LoadOrStore(key, newCounter)
	return actual.(*atomic.Uint64)
}

func getGauge(m *sync.Map, key string) *atomic.Int64 {
	v, ok := m.Load(key)
	if ok {
		return v.(*atomic.Int64)
	}
	newGauge := &atomic.Int64{}
	actual, _ := m.LoadOrStore(key, newGauge)
	return actual.(*atomic.Int64)
}

// IncRequest increments the request counter
func IncRequest(method, bucket, status string) {
	key := method + "|" + bucket + "|" + status
	getCounter(&RequestsTotal, key).Add(1)
}

// AddDuration adds request duration in milliseconds
func AddDuration(method, bucket string, ms int64) {
	key := method + "|" + bucket
	getCounter(&RequestDurationTotal, key).Add(uint64(ms))
}

// AddBytesUploaded adds to the uploaded bytes counter
func AddBytesUploaded(bucket string, bytes int64) {
	getCounter(&BytesUploadedTotal, bucket).Add(uint64(bytes))
}

// AddBytesDownloaded adds to the downloaded bytes counter
func AddBytesDownloaded(bucket string, bytes int64) {
	getCounter(&BytesDownloadedTotal, bucket).Add(uint64(bytes))
}

// SetObjectsTotal sets the total objects gauge
func SetObjectsTotal(bucket string, count int64) {
	getGauge(&ObjectsTotal, bucket).Store(count)
}

// SetStorageBytes sets the storage bytes gauge
func SetStorageBytes(bucket string, bytes int64) {
	getGauge(&StorageBytes, bucket).Store(bytes)
}

// Handler returns an http.Handler that outputs metrics in Prometheus format
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")

		fmt.Fprintln(w, "# HELP gos3_requests_total Total number of requests")
		fmt.Fprintln(w, "# TYPE gos3_requests_total counter")
		RequestsTotal.Range(func(key, value any) bool {
			parts := strings.Split(key.(string), "|")
			if len(parts) == 3 {
				fmt.Fprintf(w, "gos3_requests_total{method=\"%s\",bucket=\"%s\",status=\"%s\"} %d\n", parts[0], parts[1], parts[2], value.(*atomic.Uint64).Load())
			}
			return true
		})

		fmt.Fprintln(w, "# HELP gos3_request_duration_seconds_total Total duration of requests in seconds")
		fmt.Fprintln(w, "# TYPE gos3_request_duration_seconds_total counter")
		RequestDurationTotal.Range(func(key, value any) bool {
			parts := strings.Split(key.(string), "|")
			if len(parts) == 2 {
				// Convert ms back to seconds
				secs := float64(value.(*atomic.Uint64).Load()) / 1000.0
				fmt.Fprintf(w, "gos3_request_duration_seconds_total{method=\"%s\",bucket=\"%s\"} %f\n", parts[0], parts[1], secs)
			}
			return true
		})

		fmt.Fprintln(w, "# HELP gos3_bytes_uploaded_total Total bytes uploaded")
		fmt.Fprintln(w, "# TYPE gos3_bytes_uploaded_total counter")
		BytesUploadedTotal.Range(func(key, value any) bool {
			fmt.Fprintf(w, "gos3_bytes_uploaded_total{bucket=\"%s\"} %d\n", key.(string), value.(*atomic.Uint64).Load())
			return true
		})

		fmt.Fprintln(w, "# HELP gos3_bytes_downloaded_total Total bytes downloaded")
		fmt.Fprintln(w, "# TYPE gos3_bytes_downloaded_total counter")
		BytesDownloadedTotal.Range(func(key, value any) bool {
			fmt.Fprintf(w, "gos3_bytes_downloaded_total{bucket=\"%s\"} %d\n", key.(string), value.(*atomic.Uint64).Load())
			return true
		})

		fmt.Fprintln(w, "# HELP gos3_objects_total Total number of objects")
		fmt.Fprintln(w, "# TYPE gos3_objects_total gauge")
		ObjectsTotal.Range(func(key, value any) bool {
			fmt.Fprintf(w, "gos3_objects_total{bucket=\"%s\"} %d\n", key.(string), value.(*atomic.Int64).Load())
			return true
		})

		fmt.Fprintln(w, "# HELP gos3_storage_bytes Total storage used in bytes")
		fmt.Fprintln(w, "# TYPE gos3_storage_bytes gauge")
		StorageBytes.Range(func(key, value any) bool {
			fmt.Fprintf(w, "gos3_storage_bytes{bucket=\"%s\"} %d\n", key.(string), value.(*atomic.Int64).Load())
			return true
		})
	})
}
