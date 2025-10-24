package main

import (
	"bytes"
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

var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

func main() {
	verbose := flag.Bool("verbose", false, "emit every payload/response pair")
	fuzzCount := flag.Int("fuzz", 100, "number of fuzzed payloads to hurl at the relay")
	maxPayload := flag.Int("max-payload", 512*1024, "maximum payload size (bytes) used in fuzz cases")
	seed := flag.Int64("seed", 0, "seed for fuzzing (0 = random)")
	flag.Parse()

	if *seed == 0 {
		*seed = time.Now().UnixNano()
	}
	rng = rand.New(rand.NewSource(*seed))

	debug(*verbose, "🧮", "Fuzz seed=%d maxPayload=%d fuzzCount=%d", *seed, *maxPayload, *fuzzCount)

	if err := ensurePortAvailable(relayAddr); err != nil {
		fail("🛑", "Relay port %s is already in use (%v). Stop the running relay or choose a different port before running the smoke test.", relayAddr, err)
	}

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

	cmd := exec.CommandContext(ctx, relayPath) // #nosec G204 -- relayPath is determined by the repository layout, not user input
	if *verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
	}

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

	if err := runSmokeTest(*verbose, *fuzzCount, *maxPayload); err != nil {
		fail("😬", "Smoke test imploded: %v", err)
	}

	success("🎉", "Smoke test passed. Confidence restored (for now).")
}

// runSmokeTest orchestrates the full WebSocket → fuzz → teardown flow.
//nolint:gocyclo // High complexity is expected due to the sequential scenario orchestration.
func runSmokeTest(verbose bool, fuzzCount int, maxPayload int) error {
	u := url.URL{Scheme: "ws", Host: relayAddr, Path: websocketPath}

	dialer := *websocket.DefaultDialer
	dialer.HandshakeTimeout = requestTimeout

	conn, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to dial relay: %w", err)
	}
	defer func() {
		if cerr := conn.Close(); cerr != nil {
			warn("⚠️", "Failed to close WebSocket: %v", cerr)
		}
	}()

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

	if fuzzCount < 0 {
		fuzzCount = 0
	}

	result, err := fuzzMessages(conn, fuzzCount, verbose, maxPayload)
	if err != nil {
		return err
	}
	for _, fuzzErr := range result.errors {
		warn("⚠️", "Fuzz issue: %v", fuzzErr)
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

	if len(result.errors) > 0 {
		return fmt.Errorf("%d fuzz cases failed (see warnings above)", len(result.errors))
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
    return errors.As(err, &closeErr)
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

type fuzzStats struct {
	echoes        int
	recoverables  int
	malformedJSON int
	malformedUTF8 int
	oversized     int
	duplicateKeys int
	binary        int
}

type fuzzSummary struct {
	stats  fuzzStats
	errors []error
}

// fuzzMessages blasts randomized payloads covering numerous failure classes.
//nolint:gocyclo // Intentionally explores many branches to exercise the relay.
func fuzzMessages(conn *websocket.Conn, count int, verbose bool, maxPayload int) (fuzzSummary, error) {
	summary := fuzzSummary{}
	if count == 0 {
		success("🌀", "Skipped fuzzing (count=0).")
		return summary, nil
	}

	if maxPayload < 1024 {
		maxPayload = 1024
	}

	for i := 0; i < count; i++ {
		mode := rng.Intn(12)
		var caseErr error

		switch mode {
		case 0, 1: // Valid echo with extra chaos
			msg := buildValidMessage(maxPayload)
			debug(verbose, "📤", "Fuzz #%d send (echo w/ extras): %s", i, stringify(msg))
			if e := writeJSON(conn, msg); e != nil {
				caseErr = fmt.Errorf("fuzz #%d failed to write echo message: %w", i, e)
			} else if resp, e := readJSON(conn); e != nil {
				caseErr = fmt.Errorf("fuzz #%d failed to read echo response: %w", i, e)
			} else if e := ensureEchoMatch(msg, resp); e != nil {
				caseErr = fmt.Errorf("fuzz #%d echo mismatch: %w", i, e)
			} else {
				summary.stats.echoes++
			}

		case 2: // Missing version
			msg := buildInvalidMessage(false, true)
			if e := expectInvalid(conn, msg, verbose, i); e != nil {
				caseErr = fmt.Errorf("fuzz #%d %w", i, e)
			} else {
				summary.stats.recoverables++
			}

		case 3: // Missing type
			msg := buildInvalidMessage(true, false)
			if e := expectInvalid(conn, msg, verbose, i); e != nil {
				caseErr = fmt.Errorf("fuzz #%d %w", i, e)
			} else {
				summary.stats.recoverables++
			}

		case 4: // Missing both
			msg := buildInvalidMessage(false, false)
			if e := expectInvalid(conn, msg, verbose, i); e != nil {
				caseErr = fmt.Errorf("fuzz #%d %w", i, e)
			} else {
				summary.stats.recoverables++
			}

		case 5: // Wrong types + junk fields
			msg := buildTypeViolatingMessage()
			if e := expectInvalid(conn, msg, verbose, i); e != nil {
				caseErr = fmt.Errorf("fuzz #%d %w", i, e)
			} else {
				summary.stats.recoverables++
			}

		case 6: // Oversized payload missing required fields
			msg := buildOversizedMessage(maxPayload)
			if e := expectInvalid(conn, msg, verbose, i); e != nil {
				caseErr = fmt.Errorf("fuzz #%d %w", i, e)
			} else {
				summary.stats.oversized++
			}

		case 7: // Malformed JSON (broken syntax)
			if e := sendMalformedFrame(conn, verbose, i, maxPayload, false); e != nil {
				caseErr = fmt.Errorf("fuzz #%d %w", i, e)
			} else {
				summary.stats.malformedJSON++
			}

		case 8: // Malformed UTF-8 inside JSON
			if e := sendMalformedFrame(conn, verbose, i, maxPayload, true); e != nil {
				caseErr = fmt.Errorf("fuzz #%d %w", i, e)
			} else {
				summary.stats.malformedUTF8++
			}

		case 9: // Duplicate keys via raw string
			if e := sendDuplicateKeyJSON(conn, verbose, i); e != nil {
				caseErr = fmt.Errorf("fuzz #%d %w", i, e)
			} else {
				summary.stats.duplicateKeys++
			}

		case 10: // Root array of mixed junk
			msg := buildArrayMessage(maxPayload)
			if e := expectInvalid(conn, msg, verbose, i); e != nil {
				caseErr = fmt.Errorf("fuzz #%d %w", i, e)
			} else {
				summary.stats.recoverables++
			}

		case 11: // Binary frame with random bytes
			if e := sendBinaryFrame(conn, verbose, i, maxPayload); e != nil {
				caseErr = fmt.Errorf("fuzz #%d %w", i, e)
			} else {
				summary.stats.binary++
			}
		}

		if caseErr != nil {
			summary.errors = append(summary.errors, caseErr)
			if isClosedError(caseErr) {
				break
			}
		}
	}

	totalInvalid := summary.stats.recoverables + summary.stats.malformedJSON + summary.stats.malformedUTF8 + summary.stats.binary + summary.stats.oversized
	success("🌀", "Fuzzed %d payloads (echo:%d invalid:%d malformed-json:%d malformed-utf8:%d duplicate:%d binary:%d oversized:%d).",
		count,
		summary.stats.echoes,
		totalInvalid,
		summary.stats.malformedJSON,
		summary.stats.malformedUTF8,
		summary.stats.duplicateKeys,
		summary.stats.binary,
		summary.stats.oversized,
	)
	return summary, nil
}

func buildValidMessage(maxPayload int) map[string]interface{} {
	msg := map[string]interface{}{
		"version": "1.0",
		"type":    randomType(),
		"payload": randomString(1, clamp(maxPayload/64, 16, 2048), true),
	}

	// Sprinkle extra nested chaos
	extra := rng.Intn(4)
	for i := 0; i < extra; i++ {
		msg[randomKey()] = randomJSONValue(0)
	}

	return msg
}

func buildInvalidMessage(includeVersion, includeType bool) map[string]interface{} {
	msg := make(map[string]interface{})
	if includeVersion {
		msg["version"] = "1.0"
	}
	if includeType {
		msg["type"] = randomType()
	}
	// Random payload shape
	msg[randomKey()] = randomJSONValue(0)
	return msg
}

func buildTypeViolatingMessage() map[string]interface{} {
	msg := map[string]interface{}{
		"version": 1.0,                            // Wrong type
		"type":    map[string]interface{}{"x": 1}, // Wrong type again
		"payload": randomJSONValue(0),
	}
	msg[randomKey()] = randomJSONValue(1)
	return msg
}

func buildOversizedMessage(maxPayload int) map[string]interface{} {
	size := clamp(rng.Intn(maxPayload/2)+maxPayload/2, 1024, maxPayload)
	msg := map[string]interface{}{
		"payload": randomString(size, size+1, false),
	}
	if rng.Intn(2) == 0 {
		// Sometimes include version but omit type, sometimes vice versa.
		msg["version"] = "1.0"
	} else {
		msg["type"] = randomType()
	}
	return msg
}

func expectInvalid(conn *websocket.Conn, msg map[string]interface{}, verbose bool, idx int) error {
	debug(verbose, "📤", "Fuzz #%d send (invalid): %s", idx, stringify(msg))
	if err := writeJSON(conn, msg); err != nil {
		return fmt.Errorf("fuzz #%d failed to write invalid payload: %w", idx, err)
	}
	resp, err := readJSON(conn)
	if err != nil {
		return fmt.Errorf("fuzz #%d failed to read invalid response: %w", idx, err)
	}
	debug(verbose, "📬", "Fuzz #%d recv (invalid): %s", idx, stringify(resp))
	if !isRecoverableInvalid(resp) {
		return fmt.Errorf("fuzz #%d expected recoverable validation error, got %s", idx, stringify(resp))
	}
	return nil
}

func sendMalformedFrame(conn *websocket.Conn, verbose bool, idx int, maxPayload int, badUTF8 bool) error {
	size := clamp(rng.Intn(256)+64, 64, maxPayload)
	var garbage string
	if badUTF8 {
		garbage = string([]byte{'{', '"', 'v', 'e', 'r', 's', 'i', 'o', 'n', '"', ':', '"', 0xff, 0xfe, '"', '}'})
	} else {
		garbage = randomString(size, size+1, false)
	}
	debug(verbose, "📤", "Fuzz #%d send (malformed json=%t): %q", idx, !badUTF8, garbage)

	_ = conn.SetWriteDeadline(time.Now().Add(requestTimeout))
	if err := conn.WriteMessage(websocket.TextMessage, []byte(garbage)); err != nil {
		return fmt.Errorf("fuzz #%d failed to write malformed frame: %w", idx, err)
	}

	resp, err := readJSON(conn)
	if err != nil {
		return fmt.Errorf("fuzz #%d failed to read malformed response: %w", idx, err)
	}
	debug(verbose, "📬", "Fuzz #%d recv (malformed): %s", idx, stringify(resp))
	if !isRecoverableInvalid(resp) {
		return fmt.Errorf("fuzz #%d expected invalid response to malformed frame, got %s", idx, stringify(resp))
	}
	return nil
}

func ensureEchoMatch(expected, resp map[string]interface{}) error {
	ts, ok := resp["timestamp"].(string)
	if !ok || ts == "" {
		return fmt.Errorf("missing timestamp in response: %s", stringify(resp))
	}

	clone := make(map[string]interface{}, len(resp))
	for k, v := range resp {
		if k == "timestamp" {
			continue
		}
		clone[k] = v
	}

	if !jsonEqual(expected, clone) {
		return fmt.Errorf("response did not echo request\nexpected=%s\n   actual=%s",
			stringify(expected), stringify(clone))
	}
	return nil
}

func jsonEqual(a, b interface{}) bool {
	aj, err := json.Marshal(a)
	if err != nil {
		return false
	}
	bj, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return string(aj) == string(bj)
}

func ensurePortAvailable(addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err == nil {
		return listener.Close()
	}

	_, port, splitErr := net.SplitHostPort(addr)
	if splitErr != nil {
		return err
	}

	details, detailErr := lookupPortOwner(port)
	if detailErr != nil || details == "" {
		return err
	}

	return fmt.Errorf("%w\nPort %s appears to be in use by:\n%s", err, port, details)
}

func lookupPortOwner(port string) (string, error) {
	lsofCmd := exec.Command("lsof", "-iTCP:"+port, "-sTCP:LISTEN", "-n", "-P")
	var stdout bytes.Buffer
	lsofCmd.Stdout = &stdout
	if err := lsofCmd.Run(); err == nil {
		output := strings.TrimSpace(stdout.String())
		if output != "" {
			return output, nil
		}
	}

	ssCmd := exec.Command("ss", "-tulpn")
	stdout.Reset()
	ssCmd.Stdout = &stdout
	if err := ssCmd.Run(); err == nil {
		lines := strings.Split(stdout.String(), "\n")
		var matches []string
		needle := ":" + port
		for _, line := range lines {
			if strings.Contains(line, needle) {
				matches = append(matches, strings.TrimSpace(line))
			}
		}
		if len(matches) > 0 {
			return strings.Join(matches, "\n"), nil
		}
	}

	netstatCmd := exec.Command("netstat", "-anp")
	stdout.Reset()
	netstatCmd.Stdout = &stdout
	if err := netstatCmd.Run(); err == nil {
		lines := strings.Split(stdout.String(), "\n")
		var matches []string
		needle := ":" + port
		for _, line := range lines {
			if strings.Contains(line, needle) && strings.Contains(strings.ToUpper(line), "LISTEN") {
				matches = append(matches, strings.TrimSpace(line))
			}
		}
		if len(matches) > 0 {
			return strings.Join(matches, "\n"), nil
		}
	}

	return "", fmt.Errorf("no diagnostic tool revealed owner of port %s", port)
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

func randomType() string {
	base := []string{"echo", "session:create", "agent:message", "telemetry"}
	if rng.Intn(5) == 0 {
		return randomString(3, 12, true)
	}
	return base[rng.Intn(len(base))]
}

func randomKey() string {
	return randomString(3, 10, true)
}

func buildArrayMessage(maxPayload int) map[string]interface{} {
	arrLen := clamp(rng.Intn(5)+1, 1, 8)
	arr := make([]interface{}, arrLen)
	for i := range arr {
		arr[i] = randomJSONValue(0)
	}
	return map[string]interface{}{
		"version": "1.0",
		"payload": arr,
	}
}

func sendDuplicateKeyJSON(conn *websocket.Conn, verbose bool, idx int) error {
	raw := fmt.Sprintf("{\"version\":\"1.0\",\"type\":\"echo\",\"type\":\"shadow\",\"payload\":\"dup-%d\"}", idx)
	debug(verbose, "📤", "Fuzz #%d send (duplicate keys): %s", idx, raw)

	_ = conn.SetWriteDeadline(time.Now().Add(requestTimeout))
	if err := conn.WriteMessage(websocket.TextMessage, []byte(raw)); err != nil {
		return fmt.Errorf("fuzz #%d failed to write duplicate-key payload: %w", idx, err)
	}

	resp, err := readJSON(conn)
	if err != nil {
		return fmt.Errorf("fuzz #%d failed to read duplicate-key response: %w", idx, err)
	}
	debug(verbose, "📬", "Fuzz #%d recv (duplicate keys): %s", idx, stringify(resp))

	expected := map[string]interface{}{
		"version": "1.0",
		"type":    "shadow",
		"payload": fmt.Sprintf("dup-%d", idx),
	}

	if err := ensureEchoMatch(expected, resp); err != nil {
		return fmt.Errorf("fuzz #%d duplicate-key echo mismatch: %w", idx, err)
	}

	return nil
}

func sendBinaryFrame(conn *websocket.Conn, verbose bool, idx int, maxPayload int) error {
	size := clamp(rng.Intn(256)+32, 32, maxPayload)
	buf := make([]byte, size)
	if _, err := rng.Read(buf); err != nil {
		return fmt.Errorf("fuzz #%d failed to generate binary blob: %w", idx, err)
	}
	debug(verbose, "📤", "Fuzz #%d send (binary %d bytes)", idx, size)

	_ = conn.SetWriteDeadline(time.Now().Add(requestTimeout))
	if err := conn.WriteMessage(websocket.BinaryMessage, buf); err != nil {
		return fmt.Errorf("fuzz #%d failed to write binary frame: %w", idx, err)
	}

	resp, err := readJSON(conn)
	if err != nil {
		return fmt.Errorf("fuzz #%d failed to read binary response: %w", idx, err)
	}
	debug(verbose, "📬", "Fuzz #%d recv (binary): %s", idx, stringify(resp))
	if !isRecoverableInvalid(resp) {
		return fmt.Errorf("fuzz #%d expected invalid response to binary frame, got %s", idx, stringify(resp))
	}
	return nil
}

func randomJSONValue(depth int) interface{} {
	if depth > 2 {
		return randomString(1, 16, true)
	}

	switch rng.Intn(6) {
	case 0:
		return randomString(0, 24, true)
	case 1:
		return float64(rng.Intn(1_000_000))
	case 2:
		return rng.Intn(2) == 0
	case 3:
		return nil
	case 4:
		length := rng.Intn(4)
		arr := make([]interface{}, length)
		for i := range arr {
			arr[i] = randomJSONValue(depth + 1)
		}
		return arr
	default:
		length := rng.Intn(4)
		obj := make(map[string]interface{}, length)
		for i := 0; i < length; i++ {
			obj[randomKey()] = randomJSONValue(depth + 1)
		}
		return obj
	}
}

func randomString(min, max int, allowWeird bool) string {
	if max <= min {
		max = min + 1
	}
	n := rng.Intn(max-min) + min
	if n <= 0 {
		return ""
	}

	charset := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_- ")
	weird := []rune{'🔥', '✨', '漢', '💥', 'Ω', 'ß', '¿', '→', '\a'}
	builder := strings.Builder{}
	builder.Grow(n * 4)
	for i := 0; i < n; i++ {
		if allowWeird && rng.Intn(8) == 0 {
			builder.WriteRune(weird[rng.Intn(len(weird))])
			continue
		}
		builder.WriteRune(charset[rng.Intn(len(charset))])
	}
	out := builder.String()
	if allowWeird {
		out = strings.TrimSpace(out)
	}
	if out == "" {
		return "x"
	}
	return out
}

func clamp(val, min, max int) int {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}
