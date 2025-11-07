package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	goprocv1 "goproc/api/proto/goproc/v1"
)

// AddParams configures PID registration.
type AddParams struct {
	PID     int
	Tags    []string
	Groups  []string
	Name    string
	Timeout time.Duration
}

// AddResult reports the daemon response.
type AddResult struct {
	ID             uint64
	AlreadyExists  bool
	ExistingReason string
}

// Add registers a running PID with the daemon.
func (a *App) Add(ctx context.Context, params AddParams) (AddResult, error) {
	var result AddResult

	if params.PID <= 0 {
		return result, fmt.Errorf("invalid pid %d", params.PID)
	}

	name := strings.TrimSpace(params.Name)
	tags := append([]string(nil), params.Tags...)
	groups := append([]string(nil), params.Groups...)

	err := a.withClient(ctx, params.Timeout, func(ctx context.Context, client goprocv1.GoProcClient) error {
		resp, err := client.Add(ctx, &goprocv1.AddRequest{
			Pid:    int32(params.PID),
			Tags:   tags,
			Groups: groups,
			Name:   name,
		})
		if err != nil {
			if st, ok := status.FromError(err); ok && st.Code() == codes.AlreadyExists {
				result.AlreadyExists = true
				result.ExistingReason = st.Message()
				return nil
			}
			return fmt.Errorf("daemon add RPC failed: %w", err)
		}
		result.ID = resp.GetId()
		return nil
	})
	return result, err
}
