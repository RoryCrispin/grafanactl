// Copyright 2015 The Hugo Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// Original file: https://github.com/gohugoio/hugo/blob/89bd025ebfd2c559039826641702941fc35a7fdb/livereload/livereload.go

package livereload

import (
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/grafana/grafanactl/internal/resources"
)

const reloadDebounceWindow = 200 * time.Millisecond
const reloadQueueSize = 128

var reloadLoopOnce sync.Once
var reloadQueue = make(chan *resources.Resource, reloadQueueSize)

// Initialize starts the Websocket Hub handling live reloads.
// Original: https://github.com/gohugoio/hugo/blob/89bd025ebfd2c559039826641702941fc35a7fdb/livereload/livereload.go#L107
func Initialize() {
	go wsHub.run()
	reloadLoopOnce.Do(func() {
		go reloadLoop()
	})
}

// Handler is a HandlerFunc handling the livereload
// Websocket interaction.
// Original: https://github.com/gohugoio/hugo/blob/89bd025ebfd2c559039826641702941fc35a7fdb/livereload/livereload.go#L93-L105
// Our version is modified to accept a websocket upgrader coming from the server.
func Handler(upgrader *websocket.Upgrader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		c := &connection{send: make(chan []byte, 256), ws: ws}
		wsHub.register <- c
		defer func() { wsHub.unregister <- c }()
		go c.writer()
		c.reader()
	}
}

func ReloadResource(r *resources.Resource) {
	select {
	case reloadQueue <- r:
	default:
		// If we're already backed up, we can safely drop since a reload is pending.
	}
}

func reloadLoop() {
	var (
		timer       *time.Timer
		pending     bool
		pendingLast *resources.Resource
		pendingN    int
	)

	for {
		if !pending {
			r := <-reloadQueue
			pending = true
			pendingLast = r
			pendingN = 1
			timer = time.NewTimer(reloadDebounceWindow)
			continue
		}

		select {
		case r := <-reloadQueue:
			pendingLast = r
			pendingN++
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(reloadDebounceWindow)
		case <-timer.C:
			triggerReload(pendingLast, pendingN)
			pending = false
			pendingLast = nil
			pendingN = 0
		}
	}
}

func triggerReload(r *resources.Resource, n int) {
	// Send reload command. The path is informational for debugging.
	// The client will reload the current page when it receives this message.
	msg := fmt.Sprintf(`{"command": "reload", "path": "/d/%s/slug"}`, r.UID())
	if n > 1 {
		slog.Info("livereload: coalesced resource changes, triggering reload",
			slog.Int("changes", n),
			slog.String("resource", r.Name()),
			slog.String("uid", r.UID()),
			slog.String("kind", r.Kind()),
		)
	} else {
		slog.Info("livereload: resource changed, triggering reload",
			slog.String("resource", r.Name()),
			slog.String("uid", r.UID()),
			slog.String("kind", r.Kind()),
		)
	}
	wsHub.broadcast <- []byte(msg)
}
