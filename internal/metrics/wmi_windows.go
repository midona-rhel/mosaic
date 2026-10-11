//go:build windows

package metrics

import (
	"context"

	"github.com/yusufpapurcu/wmi"
)

// WMI calls cannot be cancelled mid-COM call. Bound callers by their context
// and permit only one outstanding sensor query, including after a timeout.
// Each worker owns its destination, so a late response cannot mutate a report.
var sensorWMI = make(chan struct{}, 1)

func querySensors[T any](ctx context.Context, namespace, query string) ([]T, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case sensorWMI <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	type result struct {
		rows []T
		err  error
	}
	done := make(chan result, 1)
	go func() {
		defer func() { <-sensorWMI }()
		var rows []T
		err := wmi.QueryNamespace(query, &rows, namespace)
		done <- result{rows, err}
	}()
	select {
	case r := <-done:
		return r.rows, r.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
