package lsp

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

var (
	errServerNotInitialized = jsonrpc2.NewError(jsonrpc2.ServerNotInitialized, "server not initialized")
	errInitializeOnce       = jsonrpc2.NewError(jsonrpc2.InvalidRequest, "initialize may only be sent once")
)

type ExitError struct{ Code int }

func (e *ExitError) Error() string {
	return fmt.Sprintf("lsp exit code %d", e.Code)
}

type serverState int

const (
	stateNew                 serverState = iota // before the initialize request has been answered
	stateInitialized                            // an InitializeResult was sent
	stateShutdownRequested                      // shutdown was received
	stateExited                                 // exit arrived without a prior shutdown
	stateExitedAfterShutdown                    // exit arrived after shutdown
)

func (st serverState) ExitCode() int {
	switch st {
	case stateExitedAfterShutdown:
		return 0
	default:
		return 1
	}
}

// lifecycle gates the connection on the initialize request: before it, only
// initialize and exit are served; everything else answers ServerNotInitialized
// and notifications are dropped. Every later message is served — like gopls,
// the server trusts its client, so shutdown and exit leave subsequent requests
// to fail naturally. The handlers themselves enforce initialize-once and the
// exit code.
func (s *server) lifecycle(next jsonrpc2.Handler) jsonrpc2.Handler {
	return func(ctx context.Context, req *jsonrpc2.Request) (any, error) {
		s.stateMu.Lock()
		st := s.state
		s.stateMu.Unlock()
		if st != stateNew || req.Method() == protocol.MethodInitialize || req.Method() == protocol.MethodExit {
			return next(ctx, req)
		}
		return rejectCall(errServerNotInitialized, req)
	}
}

// rejectCall answers a call with err and drops a notification.
func rejectCall(err error, req *jsonrpc2.Request) (any, error) {
	if req.IsCall() {
		return nil, err
	}
	return nil, nil // notification dropped
}

// parentPollInterval is how often watchParent probes the parent process.
const parentPollInterval = 2 * time.Second

// watchParent exits the server when the parent process named in the initialize
// params dies, as the spec's processId semantics require. Liveness is probed
// with signal 0, which is unix-only; elsewhere the stream EOF that follows a
// parent death covers it. The first probe runs immediately, so a parent that
// is already dead is detected at once. Exit runs the same lifecycle path as
// the exit notification, so the exit code follows the shutdown phase.
func (s *server) watchParent(pid int32) {
	if pid <= 0 || runtime.GOOS == "windows" || runtime.GOOS == "plan9" {
		return
	}
	go func() {
		t := time.NewTicker(parentPollInterval)
		defer t.Stop()
		for {
			// Only ESRCH proves death; nil (alive) and other errors leave it open.
			if err := syscall.Kill(int(pid), 0); err == nil || !errors.Is(err, syscall.ESRCH) {
				<-t.C
				continue
			}
			s.Exit(context.Background())
			return
		}
	}()
}
