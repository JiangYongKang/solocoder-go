package mockgen

import (
	"fmt"
	"reflect"
	"sync"
)

type Matcher func(actual interface{}) bool

type Expectation struct {
	methodName string
	argMatchers []Matcher
	returnValues []interface{}
	returnFunc interface{}
	minCalls int
	maxCalls int
	callCount int
	mu sync.Mutex
}

func NewExpectation(methodName string) *Expectation {
	return &Expectation{
		methodName: methodName,
		argMatchers: make([]Matcher, 0),
		minCalls:   -1,
		maxCalls:   -1,
	}
}

func (e *Expectation) Matches(args []interface{}) bool {
	if len(e.argMatchers) != len(args) {
		return false
	}
	for i, matcher := range e.argMatchers {
		if !matcher(args[i]) {
			return false
		}
	}
	return true
}

func (e *Expectation) IncrementCallCount() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.callCount++
}

func (e *Expectation) CallCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.callCount
}

func (e *Expectation) Verify() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.minCalls >= 0 && e.callCount < e.minCalls {
		return fmt.Errorf("%w: method %q expected at least %d calls, got %d",
			ErrCallCountMismatch, e.methodName, e.minCalls, e.callCount)
	}
	if e.maxCalls >= 0 && e.callCount > e.maxCalls {
		return fmt.Errorf("%w: method %q expected at most %d calls, got %d",
			ErrCallCountMismatch, e.methodName, e.maxCalls, e.callCount)
	}
	return nil
}

type Mock struct {
	expectations map[string][]*Expectation
	unmatchedCalls []UnmatchedCall
	mu sync.Mutex
}

type UnmatchedCall struct {
	MethodName string
	Args       []interface{}
}

func NewMock() *Mock {
	return &Mock{
		expectations:   make(map[string][]*Expectation),
		unmatchedCalls: make([]UnmatchedCall, 0),
	}
}

func (m *Mock) AddExpectation(methodName string, exp *Expectation) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expectations[methodName] = append(m.expectations[methodName], exp)
}

func (m *Mock) FindMatchingExpectation(methodName string, args []interface{}) (*Expectation, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	expectations, ok := m.expectations[methodName]
	if !ok {
		return nil, false
	}

	for _, exp := range expectations {
		if exp.Matches(args) {
			return exp, true
		}
	}
	return nil, false
}

func (m *Mock) RecordUnmatchedCall(methodName string, args []interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.unmatchedCalls = append(m.unmatchedCalls, UnmatchedCall{
		MethodName: methodName,
		Args:       args,
	})
}

func (m *Mock) UnmatchedCalls() []UnmatchedCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]UnmatchedCall{}, m.unmatchedCalls...)
}

func (m *Mock) Expectations() map[string][]*Expectation {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make(map[string][]*Expectation)
	for k, v := range m.expectations {
		result[k] = append([]*Expectation{}, v...)
	}
	return result
}

func (m *Mock) Verify() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.unmatchedCalls) > 0 {
		return fmt.Errorf("%w: %d unmatched calls", ErrNoMatchingExpectation, len(m.unmatchedCalls))
	}

	for methodName, exps := range m.expectations {
		for _, exp := range exps {
			if err := exp.Verify(); err != nil {
				return err
			}
			_ = methodName
		}
	}

	return nil
}

func (m *Mock) VerifyVerbose() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	var report string

	if len(m.unmatchedCalls) > 0 {
		report += fmt.Sprintf("Unmatched calls (%d):\n", len(m.unmatchedCalls))
		for i, call := range m.unmatchedCalls {
			report += fmt.Sprintf("  %d. Method: %s\n", i+1, call.MethodName)
			report += fmt.Sprintf("     Args: %v\n", call.Args)

			if exps, ok := m.expectations[call.MethodName]; ok && len(exps) > 0 {
				report += fmt.Sprintf("     Registered expectations for this method (%d):\n", len(exps))
				for j, exp := range exps {
					report += fmt.Sprintf("       %d. matchers: %d, calls: %d\n", j+1, len(exp.argMatchers), exp.callCount)
				}
			} else {
				report += "     No registered expectations for this method\n"
			}
		}
	}

	for methodName, exps := range m.expectations {
		for _, exp := range exps {
			if err := exp.Verify(); err != nil {
				report += fmt.Sprintf("Call count mismatch for %s: %v\n", methodName, err)
			}
		}
	}

	return report
}

func exactMatcher(expected interface{}) Matcher {
	return func(actual interface{}) bool {
		return reflect.DeepEqual(expected, actual)
	}
}

func anyMatcher() Matcher {
	return func(actual interface{}) bool {
		return true
	}
}
