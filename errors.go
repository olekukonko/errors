package errors

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
)

const (
	ctxTimeout = "[error] timeout"
	ctxRetry   = "[error] retry"
)

type ErrorCategory string

type ErrorOpts struct {
	SkipStack int
}

type Config struct {
	StackDepth     int
	ContextSize    int
	DisableStack   bool
	DisablePooling bool
	FilterInternal bool
}

var (
	configMu sync.RWMutex
	config   = Config{
		StackDepth:     32,
		ContextSize:    2,
		DisableStack:   false,
		DisablePooling: false,
		FilterInternal: true,
	}
	poolHits   uint64
	poolMisses uint64
)

func Configure(cfg Config) {
	configMu.Lock()
	defer configMu.Unlock()
	config = cfg
}

type contextItem struct {
	key   string
	value interface{}
}

type Error struct {
	msg          string
	name         string
	template     string
	context      map[string]interface{}
	cause        error
	stack        []uintptr
	smallContext []contextItem
	smallCount   int
	callback     func()
	code         int
	category     string
	count        uint64
}

var errorPool = sync.Pool{
	New: func() interface{} {
		configMu.RLock()
		defer configMu.RUnlock()
		return &Error{smallContext: make([]contextItem, 0, config.ContextSize)}
	},
}

func getPooledError() *Error {
	configMu.RLock()
	defer configMu.RUnlock()
	if config.DisablePooling {
		return &Error{smallContext: make([]contextItem, 0, config.ContextSize)}
	}
	if err := errorPool.Get(); err != nil {
		atomic.AddUint64(&poolHits, 1)
		e := err.(*Error)
		e.Reset()
		return e
	}
	atomic.AddUint64(&poolMisses, 1)
	return &Error{smallContext: make([]contextItem, 0, config.ContextSize)}
}

func WarmPool(count int) {
	configMu.RLock()
	defer configMu.RUnlock()
	if config.DisablePooling {
		return
	}
	for i := 0; i < count; i++ {
		errorPool.Put(&Error{smallContext: make([]contextItem, 0, config.ContextSize)})
	}
}

func Empty() *Error {
	err := getPooledError()
	configMu.RLock()
	defer configMu.RUnlock()
	if !config.DisableStack {
		err.stack = captureStack(1)
	}
	return err
}

func New(text string) *Error {
	err := getPooledError()
	err.msg = text
	configMu.RLock()
	defer configMu.RUnlock()
	if !config.DisableStack {
		err.stack = captureStack(1)
	}
	return err
}

func Newf(format string, args ...interface{}) *Error {
	err := getPooledError()
	err.msg = fmt.Sprintf(format, args...)
	configMu.RLock()
	defer configMu.RUnlock()
	if !config.DisableStack {
		err.stack = captureStack(1)
	}
	return err
}

func Named(name string) *Error {
	err := getPooledError()
	err.name = name
	configMu.RLock()
	defer configMu.RUnlock()
	if !config.DisableStack {
		err.stack = captureStack(1)
	}
	return err
}

func Fast(text string) *Error {
	err := getPooledError()
	err.msg = text
	return err
}

func Wrapf(err error, format string, args ...interface{}) *Error {
	return Newf(format, args...).Wrap(err)
}

func FromContext(ctx context.Context, err error) *Error {
	if err == nil {
		return nil
	}
	e := New(err.Error())
	if ctx.Err() == context.DeadlineExceeded {
		e.WithTimeout()
	}
	return e
}

func (e *Error) Error() string {
	if e.callback != nil {
		e.callback()
	}
	if e.msg != "" {
		return e.msg
	}
	if e.template != "" {
		return e.template
	}
	if e.name != "" {
		return e.name
	}
	return "unknown error"
}

// Name returns the error name if err is enhanced, empty string otherwise.
func (e *Error) Name() string {
	return e.name
}

func (e *Error) HasError() bool {
	return e != nil && (e.msg != "" || e.template != "" || e.name != "" || e.cause != nil)
}

func (e *Error) Is(target error) bool {
	if e == nil || target == nil {
		return e == target
	}
	if te, ok := target.(*Error); ok {
		if e.name != "" && e.name == te.name {
			return true
		}
	} else if e.cause != nil {
		return errors.Is(e.cause, target)
	}
	if e.cause != nil {
		return Is(e.cause, target)
	}
	return false
}

func (e *Error) As(target interface{}) bool {
	if e == nil {
		return false
	}
	if targetPtr, ok := target.(**Error); ok {
		if e.name != "" {
			*targetPtr = e
			return true
		}
		if e.cause != nil {
			if ce, ok := e.cause.(*Error); ok {
				*targetPtr = ce
				return true
			}
		}
	}
	if e.cause != nil {
		return As(e.cause, target)
	}
	return false
}

func (e *Error) Unwrap() error {
	return e.cause
}

func (e *Error) Count() uint64 {
	return e.count
}

func (e *Error) Increment() *Error {
	atomic.AddUint64(&e.count, 1)
	return e
}

func (e *Error) Stack() []string {
	configMu.RLock()
	defer configMu.RUnlock()
	if e.stack == nil && !config.DisableStack {
		e.stack = captureStack(1)
	}
	if e.stack == nil {
		return nil
	}
	frames := runtime.CallersFrames(e.stack)
	var trace []string
	for i := 0; i < config.StackDepth; i++ {
		frame, more := frames.Next()
		if config.FilterInternal && strings.Contains(frame.Function, "github.com/olekukonko/errors") {
			if !more {
				break
			}
			continue
		}
		trace = append(trace, fmt.Sprintf("%s\n\t%s:%d", frame.Function, frame.File, frame.Line))
		if !more {
			break
		}
	}
	return trace
}

func (e *Error) With(key string, value interface{}) *Error {
	configMu.RLock()
	defer configMu.RUnlock()
	if e.smallCount < cap(e.smallContext) {
		e.smallContext = append(e.smallContext[:e.smallCount], contextItem{key, value})
		e.smallCount++
		return e
	}
	if e.context == nil {
		e.context = make(map[string]interface{}, config.ContextSize+2)
		for i := 0; i < e.smallCount; i++ {
			e.context[e.smallContext[i].key] = e.smallContext[i].value
		}
	}
	e.context[key] = value
	return e
}

func (e *Error) Wrap(cause error) *Error {
	e.cause = cause
	return e
}

func (e *Error) WrapNotNil(cause error) *Error {
	if cause != nil {
		e.cause = cause
	}
	return e
}

func (e *Error) Msgf(format string, args ...interface{}) *Error {
	e.msg = fmt.Sprintf(format, args...)
	return e
}

func (e *Error) Context() map[string]interface{} {
	if e.smallCount > 0 && e.context == nil {
		e.context = make(map[string]interface{}, e.smallCount)
		for i := 0; i < e.smallCount; i++ {
			e.context[e.smallContext[i].key] = e.smallContext[i].value
		}
	}
	return e.context
}

func (e *Error) Trace() *Error {
	if e.stack == nil {
		e.stack = captureStack(1)
	}
	return e
}

func (e *Error) Copy() *Error {
	newErr := getPooledError()
	newErr.msg = e.msg
	newErr.name = e.name
	newErr.template = e.template
	newErr.cause = e.cause
	newErr.code = e.code
	newErr.category = e.category
	newErr.count = e.count
	if e.smallCount > 0 || e.context != nil {
		for k, v := range e.Context() {
			newErr.With(k, v)
		}
	}
	return newErr
}

func (e *Error) WithName(name string) *Error {
	e.name = name
	return e
}

func (e *Error) WithTemplate(template string) *Error {
	e.template = template
	return e
}

func (e *Error) WithCode(code int) *Error {
	e.code = code
	return e
}

func (e *Error) WithTimeout() *Error {
	return e.With(ctxTimeout, true)
}

func (e *Error) WithRetryable() *Error {
	return e.With(ctxRetry, true)
}

func (e *Error) WithCategory(category ErrorCategory) *Error {
	e.category = string(category)
	return e
}

func (e *Error) Callback(fn func()) *Error {
	e.callback = fn
	return e
}

func (e *Error) Code() int {
	return e.code
}

func (e *Error) MarshalJSON() ([]byte, error) {
	type jsonError struct {
		Name    string                 `json:"name,omitempty"`
		Message string                 `json:"message,omitempty"`
		Context map[string]interface{} `json:"context,omitempty"`
		Cause   interface{}            `json:"cause,omitempty"`
		Stack   []string               `json:"stack,omitempty"`
	}
	je := jsonError{
		Name:    e.name,
		Message: e.msg,
		Context: e.Context(),
		Stack:   e.Stack(),
	}
	if e.cause != nil {
		switch c := e.cause.(type) {
		case *Error:
			je.Cause = c
		case json.Marshaler:
			je.Cause = c
		default:
			je.Cause = c.Error()
		}
	}
	return json.Marshal(je)
}

func (e *Error) Reset() {
	e.msg = ""
	e.name = ""
	e.template = ""
	e.context = nil
	e.cause = nil
	if e.stack != nil {
		stackPool.Put(e.stack)
	}
	e.stack = nil
	e.smallCount = 0
	e.smallContext = e.smallContext[:0]
	e.callback = nil
	e.code = 0
	e.category = ""
	e.count = 0
}

func (e *Error) Free() {
	configMu.RLock()
	defer configMu.RUnlock()
	e.Reset()
	if !config.DisablePooling {
		errorPool.Put(e)
	}
}

func IsError(err error) bool {
	_, ok := err.(*Error)
	return ok
}

func Stack(err error) []string {
	if e, ok := err.(*Error); ok {
		return e.Stack()
	}
	return nil
}

func Context(err error) map[string]interface{} {
	if e, ok := err.(*Error); ok {
		return e.Context()
	}
	return nil
}

func Count(err error) uint64 {
	if e, ok := err.(*Error); ok {
		return e.Count()
	}
	return 0
}

func Code(err error) int {
	if e, ok := err.(*Error); ok {
		return e.Code()
	}
	return 500
}

func Name(err error) string {
	if e, ok := err.(*Error); ok {
		return e.name
	}
	return ""
}

func With(err error, key string, value interface{}) error {
	if e, ok := err.(*Error); ok {
		return e.With(key, value)
	}
	return err
}

func Wrap(wrapper, cause error) error {
	if e, ok := wrapper.(*Error); ok {
		return e.Wrap(cause)
	}
	return wrapper
}

func IsTimeout(err error) bool {
	if e, ok := err.(*Error); ok {
		if val, ok := e.Context()[ctxTimeout].(bool); ok {
			return val
		}
	}
	return strings.Contains(strings.ToLower(err.Error()), "timeout")
}

func IsRetryable(err error) bool {
	if e, ok := err.(*Error); ok {
		if val, ok := e.Context()[ctxRetry].(bool); ok {
			return val
		}
	}
	return IsTimeout(err) || strings.Contains(strings.ToLower(err.Error()), "retry")
}

func GetCategory(err error) string {
	if e, ok := err.(*Error); ok {
		return e.category
	}
	return ""
}

func Is(err, target error) bool {
	if e, ok := err.(*Error); ok {
		return e.Is(target)
	}
	return errors.Is(err, target)
}

func As(err error, target interface{}) bool {
	if e, ok := err.(*Error); ok {
		return e.As(target)
	}
	return errors.As(err, target)
}
