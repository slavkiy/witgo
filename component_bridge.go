package witgo

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/slavkiy/witgo/internal/bridgebin"
)

type componentBridge struct {
	cmd            *exec.Cmd
	input          io.WriteCloser
	output         *bufio.Reader
	imports        map[string]HostFunc
	maxResultBytes uint64
	mu             sync.Mutex
}

type bridgeMessage struct {
	Type      string `json:"type"`
	Error     string `json:"error,omitempty"`
	Interface string `json:"interface,omitempty"`
	Function  string `json:"function,omitempty"`
	Args      []any  `json:"args,omitempty"`
	Values    []any  `json:"values,omitempty"`
	Value     any    `json:"value,omitempty"`
}

func startComponentBridge(component string, options RuntimeOptions, imports []HostImport) (*componentBridge, error) {
	path, err := resolveBridge(options.BridgePath)
	if err != nil {
		return nil, err
	}
	registered := make(map[string]HostFunc, len(imports))
	type importSpec struct {
		Interface string   `json:"interface"`
		Functions []string `json:"functions"`
	}
	grouped := make(map[string][]string)
	for _, item := range imports {
		if item.Interface == "" || item.Function == "" || item.Call == nil {
			return nil, errors.New("host import interface, function, and Call are required")
		}
		key := item.Interface + "#" + item.Function
		if _, exists := registered[key]; exists {
			return nil, fmt.Errorf("duplicate host import %q", key)
		}
		registered[key] = item.Call
		grouped[item.Interface] = append(grouped[item.Interface], item.Function)
	}
	specs := make([]importSpec, 0, len(grouped))
	for name, functions := range grouped {
		specs = append(specs, importSpec{name, functions})
	}
	cmd := exec.Command(path)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start component bridge %q: %w", path, err)
	}
	b := &componentBridge{cmd: cmd, input: stdin, output: bufio.NewReader(stdout), imports: registered, maxResultBytes: options.MaxResultBytes}
	init := map[string]any{
		"type": "init", "component": component, "imports": specs,
		"options": map[string]any{"fuel": options.Fuel, "fuel_per_call": options.FuelPerCall, "timeout_millis": options.Timeout.Milliseconds(), "memory_limit_bytes": options.MemoryLimitBytes, "instance_limit": options.InstanceLimit},
	}
	if err := b.write(init); err != nil {
		_ = b.abort()
		return nil, err
	}
	message, err := b.read()
	if err != nil {
		_ = b.abort()
		if stderr.Len() > 0 {
			err = fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
		}
		return nil, err
	}
	if message.Type != "ready" {
		_ = b.abort()
		return nil, messageError(message)
	}
	return b, nil
}

func resolveBridge(explicit string) (string, error) {
	if explicit == "" {
		explicit = os.Getenv("WITGO_COMPONENT_BRIDGE")
	}
	if explicit != "" {
		return explicit, nil
	}
	if path, err := bridgebin.Executable(); err == nil {
		return path, nil
	}
	name := "witgo-component-host"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}
	candidates := []string{filepath.Join("bridge", "target", "release", name), filepath.Join("bridge", "target", "debug", name)}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("component bridge not found; build ./bridge or set WITGO_COMPONENT_BRIDGE")
}

func (b *componentBridge) call(name string, args []any) ([]any, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if args == nil {
		args = []any{}
	}
	if err := b.write(map[string]any{"type": "call", "name": name, "args": args}); err != nil {
		return nil, err
	}
	for {
		message, err := b.read()
		if err != nil {
			return nil, err
		}
		switch message.Type {
		case "host_call":
			fn := b.imports[message.Interface+"#"+message.Function]
			if fn == nil {
				_ = b.write(map[string]any{"type": "host_result", "error": "host import is not registered"})
				continue
			}
			result, callErr := fn(message.Args)
			if callErr != nil {
				_ = b.write(map[string]any{"type": "host_result", "error": callErr.Error()})
				continue
			}
			values := []any{}
			if result != nil {
				values = append(values, result)
			}
			if err := b.write(map[string]any{"type": "host_result", "values": values}); err != nil {
				return nil, err
			}
		case "result":
			return message.Values, nil
		case "error", "fatal":
			return nil, messageError(message)
		default:
			return nil, fmt.Errorf("unexpected bridge message %q", message.Type)
		}
	}
}

func (b *componentBridge) fuel() (uint64, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.write(map[string]any{"type": "fuel"}); err != nil {
		return 0, err
	}
	message, err := b.read()
	if err != nil {
		return 0, err
	}
	if message.Type != "result" {
		return 0, messageError(message)
	}
	switch value := message.Value.(type) {
	case json.Number:
		return parseUint(value.String())
	case float64:
		return uint64(value), nil
	default:
		return 0, fmt.Errorf("invalid fuel value %T", value)
	}
}

func (b *componentBridge) setFuel(fuel uint64) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.write(map[string]any{"type": "set_fuel", "fuel": fuel}); err != nil {
		return err
	}
	message, err := b.read()
	if err != nil {
		return err
	}
	if message.Type != "result" {
		return messageError(message)
	}
	return nil
}

func (b *componentBridge) write(value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if b.maxResultBytes > 0 && uint64(len(data)) > b.maxResultBytes {
		return fmt.Errorf("%w: %d bytes, limit is %d", ErrResultTooLarge, len(data), b.maxResultBytes)
	}
	data = append(data, '\n')
	_, err = b.input.Write(data)
	return err
}

func (b *componentBridge) read() (bridgeMessage, error) {
	line, err := b.output.ReadBytes('\n')
	if err != nil {
		return bridgeMessage{}, fmt.Errorf("read component bridge: %w", err)
	}
	if b.maxResultBytes > 0 && uint64(len(line)) > b.maxResultBytes {
		return bridgeMessage{}, fmt.Errorf("%w: %d bytes, limit is %d", ErrResultTooLarge, len(line), b.maxResultBytes)
	}
	decoder := json.NewDecoder(strings.NewReader(string(line)))
	decoder.UseNumber()
	var message bridgeMessage
	if err := decoder.Decode(&message); err != nil {
		return message, fmt.Errorf("decode component bridge response: %w", err)
	}
	return message, nil
}

func (b *componentBridge) close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	_ = b.write(map[string]any{"type": "close"})
	_ = b.input.Close()
	return b.cmd.Wait()
}
func (b *componentBridge) abort() error {
	_ = b.input.Close()
	if b.cmd.Process != nil {
		_ = b.cmd.Process.Kill()
	}
	return b.cmd.Wait()
}
func messageError(message bridgeMessage) error {
	if message.Error == "" {
		return fmt.Errorf("component bridge returned %q", message.Type)
	}
	return errors.New(message.Error)
}
func parseUint(value string) (uint64, error) {
	var result uint64
	_, err := fmt.Sscan(value, &result)
	return result, err
}
