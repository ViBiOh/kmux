package delay

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Handler for delay request. Should be use with net/http
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.Trim(r.URL.Path, "/")
		delay, err := strconv.ParseInt(path, 10, 64)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		time.Sleep(time.Duration(delay) * time.Second)
	})
}
