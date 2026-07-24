package readiness

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type Check struct {
	Name string
	Run  func(context.Context) error
}

type Checker struct {
	checks []Check
}

func New(checks ...Check) (*Checker, error) {
	validated := make([]Check, 0, len(checks))
	names := make(map[string]struct{}, len(checks))

	for _, check := range checks {
		name := strings.TrimSpace(check.Name)

		if name == "" {
			return nil, fmt.Errorf(
				"readiness check name is required",
			)
		}

		if check.Run == nil {
			return nil, fmt.Errorf(
				"readiness check %q function is required",
				name,
			)
		}

		if _, exists := names[name]; exists {
			return nil, fmt.Errorf(
				"duplicate readiness check %q",
				name,
			)
		}

		names[name] = struct{}{}

		validated = append(
			validated,
			Check{
				Name: name,
				Run:  check.Run,
			},
		)
	}

	return &Checker{
		checks: validated,
	}, nil
}

func (c *Checker) Check(ctx context.Context) error {
	if c == nil {
		return errors.New("readiness checker is nil")
	}

	var checkErrors []error

	for _, check := range c.checks {
		if err := check.Run(ctx); err != nil {
			checkErrors = append(
				checkErrors,
				fmt.Errorf(
					"%s: %w",
					check.Name,
					err,
				),
			)
		}
	}

	return errors.Join(checkErrors...)
}
