package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	fuzz "github.com/google/gofuzz"
	"github.com/gorilla/websocket"
)

const (
	relayAddr        = "localhost:8080"
	websocketPath    = "/ws"
	startupTimeout   = 5 * time.Second
	requestTimeout   = 2 * time.Second
	shutdownTimeout  = 3 * time.Second
	defaultHandshake = "connection:established"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	verbose := flag.Bool("verbose", false, "emit every payload/response pair")
	flag.Parse()

	root, err := findRepoRoot()
	if err != nil {
		fail("🧭", "Failed to locate repo root: %v", err)
	}

	relayPath := filepath.Join(root, "bin", "relay")
	if _, err := os.Stat(relayPath); err != nil {
		fail("🔨", "Relay binary missing at %s (try `make build` first): %v", relayPath, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, relayPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	announce("🚀", "Booting relay from %s. Hold onto your socks.", relayPath)
	if err := cmd.Start(); err != nil {
		fail("💥", "Relay refused to start: %v", err)
	}

	defer func() {
		cancel()

		done := make(chan struct{})
		go func() {
			_ = cmd.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(shutdownTimeout):
			warn("🪓", "Relay is clinging to life. Applying gentle persuasion…")
			_ = cmd.Process.Kill()
			<-done
		}
	}()

	if err := waitForPort(relayAddr, startupTimeout); err != nil {
		fail("⌛", "Relay never opened %s: %v", relayAddr, err)
	}

	if err := runSmokeTest(*verbose); err != nil {
		fail("😬", "Smoke test imploded: %v", err)
	}

	success("🎉", "Smoke test passed. Confidence restored (for now).")
}

func runSmokeTest(verbose bool) error {
	u := url.URL{Scheme: "ws", Host: relayAddr, Path: websocketPath}

	dialer := *websocket.DefaultDialer
	dialer.HandshakeTimeout = requestTimeout

	conn, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to dial relay: %w", err)
	}
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(requestTimeout))

	var handshake map[string]interface{}
	if err := conn.ReadJSON(&handshake); err != nil {
		return fmt.Errorf("failed to read handshake: %w", err)
	}
	debug(verbose, "📥", "Handshake payload: %s", stringify(handshake))
	if handshake["type"] != defaultHandshake {
		return fmt.Errorf("unexpected handshake type: %+v", handshake)
	}
	success("🤝", "Handshake received: %s", stringify(handshake))

	// 1. Echo happy path
	echoPayload := map[string]interface{}{
		"version": "1.0",
		"type":    "echo",
		"payload": "smoke test message",
	}
	debug(verbose, "📤", "Sending echo payload: %s", stringify(echoPayload))
	if err := writeJSON(conn, echoPayload); err != nil {
		return fmt.Errorf("failed to send echo: %w", err)
	}

	resp, err := readJSON(conn)
	if err != nil {
		return fmt.Errorf("failed to read echo response: %w", err)
	}
	debug(verbose, "📬", "Echo response: %s", stringify(resp))
	if resp["type"] != "echo" || resp["payload"] != echoPayload["payload"] {
		return fmt.Errorf("unexpected echo response: %s", stringify(resp))
	}
	if _, ok := resp["timestamp"].(string); !ok {
		return fmt.Errorf("echo response missing timestamp: %s", stringify(resp))
	}
	success("🔁", "Echo behaved as advertised: %s", stringify(resp))

	// 2. Recoverable validation error (missing version)
	badPayload := map[string]interface{}{
		"type":    "echo",
		"payload": "missing version field",
	}
	debug(verbose, "📤", "Sending recoverable invalid payload: %s", stringify(badPayload))
	if err := writeJSON(conn, badPayload); err != nil {
		return fmt.Errorf("failed to send invalid payload: %w", err)
	}

	errResp, err := readJSON(conn)
	if err != nil {
		return fmt.Errorf("failed to read validation error: %w", err)
	}
	debug(verbose, "📬", "Recoverable error response: %s", stringify(errResp))
	if errType := errResp["type"]; errType != "error" {
		return fmt.Errorf("expected error response, got: %s", stringify(errResp))
	}
	errorDetail, ok := errResp["error"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("error payload missing detail: %s", stringify(errResp))
	}
	if errorDetail["code"] != "INVALID_MESSAGE" || errorDetail["recoverable"] != true {
		return fmt.Errorf("unexpected validation error: %s", stringify(errResp))
	}
	warn("🩹", "Recoverable error looked correct: %s", stringify(errResp))

	if err := fuzzMessages(conn, 100, verbose); err != nil {
		return err
	}

	// 3. Non-recoverable error (version mismatch) should close connection
	versionMismatch := map[string]interface{}{
		"version": "0.9",
		"type":    "echo",
		"payload": "old protocol version",
	}
	debug(verbose, "📤", "Sending non-recoverable payload: %s", stringify(versionMismatch))
	if err := writeJSON(conn, versionMismatch); err != nil {
		return fmt.Errorf("failed to send version mismatch payload: %w", err)
	}

	closeResp, err := readJSON(conn)
	if err != nil {
		return fmt.Errorf("failed to read version mismatch response: %w", err)
	}
	debug(verbose, "📬", "Non-recoverable response: %s", stringify(closeResp))
	closeDetail, ok := closeResp["error"].(map[string]interface{})
	if !(ok && closeDetail["code"] == "VERSION_MISMATCH" && closeDetail["recoverable"] == false) {
		return fmt.Errorf("unexpected version mismatch response: %s", stringify(closeResp))
	}
	warn("🧨", "Non-recoverable error triggered correctly: %s", stringify(closeResp))

	// Connection should now be closed by the server — next read should fail
	if _, err := readJSON(conn); err == nil {
		return errors.New("expected connection closure after non-recoverable error, but read succeeded")
	} else if !isClosedError(err) {
		return fmt.Errorf("expected close error after non-recoverable response, got %v", err)
	}

	return nil
}

func writeJSON(conn *websocket.Conn, payload interface{}) error {
	_ = conn.SetWriteDeadline(time.Now().Add(requestTimeout))
	return conn.WriteJSON(payload)
}

func readJSON(conn *websocket.Conn) (map[string]interface{}, error) {
	_ = conn.SetReadDeadline(time.Now().Add(requestTimeout))

	var resp map[string]interface{}
	if err := conn.ReadJSON(&resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func stringify(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%+v", v)
	}
	return string(data)
}

func waitForPort(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for %s: %w", addr, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("go.mod not found in parent directories")
		}
		dir = parent
	}
}

func isClosedError(err error) bool {
	if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
		return true
	}
	var closeErr *websocket.CloseError
	if errors.As(err, &closeErr) {
		return true
	}
	return false
}

const (
	colorReset  = "\033[0m"
	colorCyan   = "\033[36m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorBlue   = "\033[34m"
)

func announce(icon, format string, args ...interface{}) {
	fmt.Printf("%s%s %s%s\n", colorCyan, icon, fmt.Sprintf(format, args...), colorReset)
}

func warn(icon, format string, args ...interface{}) {
	fmt.Printf("%s%s %s%s\n", colorYellow, icon, fmt.Sprintf(format, args...), colorReset)
}

func success(icon, format string, args ...interface{}) {
	fmt.Printf("%s%s %s%s\n", colorGreen, icon, fmt.Sprintf(format, args...), colorReset)
}

func fail(icon, format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "%s%s %s%s\n", colorRed, icon, fmt.Sprintf(format, args...), colorReset)
	os.Exit(1)
}

func debug(verbose bool, icon, format string, args ...interface{}) {
	if !verbose {
		return
	}
	fmt.Printf("%s%s %s%s\n", colorBlue, icon, fmt.Sprintf(format, args...), colorReset)
}

type fuzzCase struct {
	ProvideVersion bool
	ProvideType    bool
	ProvidePayload bool
	Type           string
	Payload        string
}

func fuzzMessages(conn *websocket.Conn, count int, verbose bool) error {
	fuzzer := newPayloadFuzzer()
	var echoes, recoverables int

	for i := 0; i < count; i++ {
		var c fuzzCase
		fuzzer.Fuzz(&c)

		// Bias every third payload toward being fully valid to keep coverage balanced.
		if i%3 == 0 {
			c.ProvideVersion = true
			c.ProvideType = true
		}

		c.Type = strings.TrimSpace(c.Type)
		c.Payload = strings.TrimSpace(c.Payload)

		if c.ProvideType && c.Type == "" {
			// Empty string acts like a missing field for validation, so treat it as missing.
			c.ProvideType = false
		}

		// Build outbound message based on flags.
		msg := make(map[string]interface{})
		if c.ProvideVersion {
			msg["version"] = "1.0"
		}
		if c.ProvideType {
			msg["type"] = c.Type
		}
		if c.ProvidePayload {
			msg["payload"] = c.Payload
		}

		// Ensure we send at least one field so JSON isn't just "{}".
		if len(msg) == 0 {
			msg["payload"] = fmt.Sprintf("lonely-fuzz-%02d", i)
			c.ProvidePayload = true
		}

		expectEcho := c.ProvideVersion && c.ProvideType

		// Valid path: expect echoed payload with timestamp.
		if expectEcho {
			if !c.ProvidePayload {
				// Supply a payload so we can assert equality on the echo response.
				msg["payload"] = fmt.Sprintf("auto-echo-%02d", i)
			}

			debug(verbose, "📤", "Fuzz #%d send (echo candidate): %s", i, stringify(msg))
			if err := writeJSON(conn, msg); err != nil {
				return fmt.Errorf("fuzz #%d failed to write echo candidate: %w", i, err)
			}
			resp, err := readJSON(conn)
			if err != nil {
				return fmt.Errorf("fuzz #%d failed to read echo response: %w", i, err)
			}
			debug(verbose, "📬", "Fuzz #%d recv (echo): %s", i, stringify(resp))

			if resp["type"] != c.Type {
				return fmt.Errorf("fuzz #%d expected type %q, got %s", i, c.Type, stringify(resp))
			}
			if msgPayload, ok := msg["payload"].(string); ok && resp["payload"] != msgPayload {
				return fmt.Errorf("fuzz #%d payload mismatch: want %q got %v", i, msgPayload, resp["payload"])
			}
			if _, ok := resp["timestamp"].(string); !ok {
				return fmt.Errorf("fuzz #%d missing timestamp in echo: %s", i, stringify(resp))
			}
			echoes++
			continue
		}

		// Invalid path: expect recoverable validation error.
		debug(verbose, "📤", "Fuzz #%d send (invalid): %s", i, stringify(msg))
		if err := writeJSON(conn, msg); err != nil {
			return fmt.Errorf("fuzz #%d failed to write invalid payload: %w", i, err)
		}
		resp, err := readJSON(conn)
		if err != nil {
			return fmt.Errorf("fuzz #%d failed to read invalid response: %w", i, err)
		}
		debug(verbose, "📬", "Fuzz #%d recv (invalid): %s", i, stringify(resp))
		if !isRecoverableInvalid(resp) {
			return fmt.Errorf("fuzz #%d expected recoverable validation error, got %s", i, stringify(resp))
		}
		recoverables++
	}

	success("🌀", "Fuzzed %d payloads (%d lovely echoes, %d teachable errors).", count, echoes, recoverables)
	return nil
}

func isRecoverableInvalid(resp map[string]interface{}) bool {
	if resp["type"] != "error" {
		return false
	}
	detail, ok := resp["error"].(map[string]interface{})
	if !ok {
		return false
	}
	return detail["code"] == "INVALID_MESSAGE" && detail["recoverable"] == true
}

func newPayloadFuzzer() *fuzz.Fuzzer {
	letters := []rune("abcdefghijklmnopqrstuvwxyz0123456789-_ ")
	f := fuzz.New().NilChance(0.0).Funcs(func(s *string, c fuzz.Continue) {
		length := c.Intn(10)
		if length == 0 {
			*s = ""
			return
		}
		buf := make([]rune, length)
		for i := range buf {
			buf[i] = letters[c.Intn(len(letters))]
		}
		*s = strings.TrimSpace(string(buf))
	})
	f.RandSource(rand.NewSource(time.Now().UnixNano()))
	return f
}
