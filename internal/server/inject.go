package server

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
)

// livereloadScriptTag returns the script tag to inject into HTML responses.
func livereloadScriptTag(port int) []byte {
	return []byte(fmt.Sprintf(`<script src="/grafanactl/assets/livereload.js" data-port="%d"></script>`, port))
}

// newHTMLInjector returns a response modifier that injects a livereload script
// into HTML responses. It detects HTML by checking the Content-Type header,
// injects the script before the </body> tag, and updates the Content-Length.
func newHTMLInjector(port int) func(*http.Response) error {
	scriptTag := livereloadScriptTag(port)

	return func(resp *http.Response) error {
		contentType := resp.Header.Get("Content-Type")
		if !strings.Contains(contentType, "text/html") {
			return nil
		}

		slog.Info("inject: processing HTML response",
			slog.String("url", resp.Request.URL.String()),
			slog.String("content-type", contentType),
		)

		// Check if response is gzip encoded
		isGzip := resp.Header.Get("Content-Encoding") == "gzip"

		var body []byte
		var err error

		if isGzip {
			gzReader, err := gzip.NewReader(resp.Body)
			if err != nil {
				return fmt.Errorf("failed to create gzip reader: %w", err)
			}
			body, err = io.ReadAll(gzReader)
			gzReader.Close()
			if err != nil {
				return fmt.Errorf("failed to read gzip body: %w", err)
			}
		} else {
			body, err = io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("failed to read response body: %w", err)
			}
		}
		resp.Body.Close()

		// Find </body> tag (case-insensitive)
		bodyLower := bytes.ToLower(body)
		bodyTagIdx := bytes.LastIndex(bodyLower, []byte("</body>"))

		if bodyTagIdx == -1 {
			slog.Warn("inject: no </body> tag found, skipping injection",
				slog.String("url", resp.Request.URL.String()),
			)
			// No </body> tag found, return original response
			if isGzip {
				var buf bytes.Buffer
				gzWriter := gzip.NewWriter(&buf)
				gzWriter.Write(body)
				gzWriter.Close()
				body = buf.Bytes()
			}
			resp.Body = io.NopCloser(bytes.NewReader(body))
			return nil
		}

		slog.Info("inject: injecting livereload script",
			slog.String("url", resp.Request.URL.String()),
			slog.Int("body_size", len(body)),
		)

		// Inject script before </body>
		newBody := make([]byte, 0, len(body)+len(scriptTag)+1)
		newBody = append(newBody, body[:bodyTagIdx]...)
		newBody = append(newBody, scriptTag...)
		newBody = append(newBody, '\n')
		newBody = append(newBody, body[bodyTagIdx:]...)

		if isGzip {
			var buf bytes.Buffer
			gzWriter := gzip.NewWriter(&buf)
			gzWriter.Write(newBody)
			gzWriter.Close()
			newBody = buf.Bytes()
		}

		resp.Body = io.NopCloser(bytes.NewReader(newBody))
		resp.ContentLength = int64(len(newBody))
		resp.Header.Set("Content-Length", strconv.Itoa(len(newBody)))

		return nil
	}
}
