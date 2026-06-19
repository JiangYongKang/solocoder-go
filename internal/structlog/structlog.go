package structlog

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const pkgPrefix = "solocoder-go/internal/structlog."

type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

type sharedState struct {
	output         io.Writer
	level          atomic.Int32
	samplingRates  [4]atomic.Int32
	sampleCounters [4]atomic.Int64
	mu             sync.Mutex
}

type Logger struct {
	state  *sharedState
	fields map[string]interface{}
}

func New(output io.Writer, level Level) *Logger {
	if output == nil {
		output = os.Stdout
	}
	s := &sharedState{
		output: output,
	}
	s.level.Store(int32(level))
	return &Logger{
		state:  s,
		fields: make(map[string]interface{}),
	}
}

func (l *Logger) SetLevel(level Level) {
	l.state.level.Store(int32(level))
}

func (l *Logger) GetLevel() Level {
	return Level(l.state.level.Load())
}

func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	newFields := make(map[string]interface{}, len(l.fields)+len(fields))
	for k, v := range l.fields {
		newFields[k] = v
	}
	for k, v := range fields {
		newFields[k] = v
	}
	return &Logger{
		state:  l.state,
		fields: newFields,
	}
}

func (l *Logger) SetSamplingRate(level Level, rate int) {
	if level < LevelDebug || level > LevelError {
		return
	}
	l.state.samplingRates[level].Store(int32(rate))
	l.state.sampleCounters[level].Store(0)
}

func (l *Logger) Debug(msg string) {
	l.log(LevelDebug, msg)
}

func (l *Logger) Info(msg string) {
	l.log(LevelInfo, msg)
}

func (l *Logger) Warn(msg string) {
	l.log(LevelWarn, msg)
}

func (l *Logger) Error(msg string) {
	l.log(LevelError, msg)
}

func (l *Logger) shouldSample(level Level) bool {
	rate := l.state.samplingRates[level].Load()
	if rate <= 0 {
		return false
	}
	counter := l.state.sampleCounters[level].Add(1)
	return (counter-1)%int64(rate) != 0
}

func (l *Logger) log(level Level, msg string) {
	currentLevel := Level(l.state.level.Load())
	if level < currentLevel {
		return
	}

	if l.shouldSample(level) {
		return
	}

	entry := make(map[string]interface{}, len(l.fields)+5)
	entry["ts"] = time.Now().UTC().Format(time.RFC3339Nano)
	entry["level"] = level.String()
	entry["msg"] = msg

	for k, v := range l.fields {
		entry[k] = v
	}

	caller := captureCaller()
	if caller != "" {
		entry["caller"] = caller
	}

	if level == LevelError {
		stack := captureStack()
		if len(stack) > 0 {
			entry["stack"] = stack
		}
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	l.state.mu.Lock()
	defer l.state.mu.Unlock()
	l.state.output.Write(data)
	l.state.output.Write([]byte("\n"))
}

func captureCaller() string {
	pcs := make([]uintptr, 32)
	n := runtime.Callers(2, pcs)
	if n == 0 {
		return ""
	}
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if !strings.HasPrefix(frame.Function, pkgPrefix) {
			return fmt.Sprintf("%s:%d", frame.File, frame.Line)
		}
		if !more {
			break
		}
	}
	return ""
}

func captureStack() []string {
	pcs := make([]uintptr, 32)
	n := runtime.Callers(2, pcs)
	if n == 0 {
		return nil
	}
	frames := runtime.CallersFrames(pcs[:n])
	var stack []string
	for {
		frame, more := frames.Next()
		if !strings.HasPrefix(frame.Function, pkgPrefix) {
			stack = append(stack, fmt.Sprintf("%s:%d", frame.File, frame.Line))
		}
		if !more {
			break
		}
	}
	return stack
}
