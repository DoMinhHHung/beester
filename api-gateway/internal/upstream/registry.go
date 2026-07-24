package upstream

import (
	"errors"
	"fmt"
	"strings"

	"github.com/DoMinhHHung/beester/api-gateway/internal/grpcclient"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type Spec struct {
	Name   string
	Target string
}

type Registry struct {
	conns map[string]*grpc.ClientConn
	names []string
}

func New(
	specs []Spec,
	transportCredentials credentials.TransportCredentials,
) (*Registry, error) {
	if len(specs) > 0 && transportCredentials == nil {
		return nil, errors.New(
			"gRPC transport credentials are required",
		)
	}

	registry := &Registry{
		conns: make(
			map[string]*grpc.ClientConn,
			len(specs),
		),
		names: make([]string, 0, len(specs)),
	}

	for _, spec := range specs {
		name := strings.TrimSpace(spec.Name)
		target := strings.TrimSpace(spec.Target)

		if name == "" {
			_ = registry.Close()

			return nil, errors.New(
				"upstream name is required",
			)
		}

		if target == "" {
			_ = registry.Close()

			return nil, fmt.Errorf(
				"upstream %q target is required",
				name,
			)
		}

		if _, exists := registry.conns[name]; exists {
			_ = registry.Close()

			return nil, fmt.Errorf(
				"duplicate upstream %q",
				name,
			)
		}

		conn, err := grpcclient.New(
			target,
			transportCredentials.Clone(),
		)
		if err != nil {
			closeErr := registry.Close()

			createErr := fmt.Errorf(
				"create upstream %q: %w",
				name,
				err,
			)

			if closeErr != nil {
				return nil, errors.Join(
					createErr,
					fmt.Errorf(
						"cleanup upstream connections: %w",
						closeErr,
					),
				)
			}

			return nil, createErr
		}

		registry.conns[name] = conn
		registry.names = append(
			registry.names,
			name,
		)
	}

	return registry, nil
}

func (r *Registry) Conn(
	name string,
) (*grpc.ClientConn, bool) {
	if r == nil || r.conns == nil {
		return nil, false
	}

	conn, ok := r.conns[name]

	return conn, ok
}

func (r *Registry) Names() []string {
	if r == nil {
		return nil
	}

	return append(
		[]string(nil),
		r.names...,
	)
}

func (r *Registry) Close() error {
	if r == nil || r.conns == nil {
		return nil
	}

	var closeErrors []error

	for name, conn := range r.conns {
		if err := conn.Close(); err != nil {
			closeErrors = append(
				closeErrors,
				fmt.Errorf(
					"close upstream %q: %w",
					name,
					err,
				),
			)
		}
	}

	r.conns = nil
	r.names = nil

	return errors.Join(closeErrors...)
}
