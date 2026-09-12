package handlers

import "net/http"

// Health is a liveness probe target: "is the process alive at all".
func Health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// Ready is a readiness probe target: "can this pod take traffic right now".
// Wire in real dependency checks (DB ping, etc.) as projects add them.
func Ready(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ready"))
}
