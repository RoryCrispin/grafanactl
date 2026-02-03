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
// Original file:  https://github.com/gohugoio/hugo/blob/89bd025ebfd2c559039826641702941fc35a7fdb/livereload/hub.go

package livereload

import "log/slog"

type hub struct {
	// Registered connections.
	connections map[*connection]bool

	// Inbound messages from the connections.
	broadcast chan []byte

	// Register requests from the connections.
	register chan *connection

	// Unregister requests from connections.
	unregister chan *connection
}

//nolint:gochecknoglobals
var wsHub = hub{
	broadcast:   make(chan []byte),
	register:    make(chan *connection),
	unregister:  make(chan *connection),
	connections: make(map[*connection]bool),
}

func (h *hub) run() {
	for {
		select {
		case c := <-h.register:
			h.connections[c] = true
			slog.Info("livereload: client connected", slog.Int("total_connections", len(h.connections)))
		case c := <-h.unregister:
			delete(h.connections, c)
			c.close()
			slog.Info("livereload: client disconnected", slog.Int("total_connections", len(h.connections)))
		case m := <-h.broadcast:
			slog.Info("livereload: broadcasting reload", slog.String("message", string(m)), slog.Int("connections", len(h.connections)))
			for c := range h.connections {
				select {
				case c.send <- m:
				default:
					delete(h.connections, c)
					c.close()
				}
			}
		}
	}
}
