package e2b

import (
	"context"
	"fmt"
	"net/http"

	"connectrpc.com/connect"
	"station/pkg/harness/sandbox/e2b/proto/gen/process"
	"station/pkg/harness/sandbox/e2b/proto/gen/processconnect"
)

type StreamingClient struct {
	processClient processconnect.ProcessClient
	accessToken   string
}

func NewStreamingClient(envdURL, accessToken string) *StreamingClient {
	httpClient := &http.Client{}

	opts := []connect.ClientOption{
		connect.WithInterceptors(&authInterceptor{token: accessToken}),
	}

	return &StreamingClient{
		processClient: processconnect.NewProcessClient(httpClient, envdURL, opts...),
		accessToken:   accessToken,
	}
}

type authInterceptor struct {
	token string
}

func (i *authInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		req.Header().Set("X-Access-Token", i.token)
		return next(ctx, req)
	}
}

func (i *authInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		conn := next(ctx, spec)
		conn.RequestHeader().Set("X-Access-Token", i.token)
		return conn
	}
}

func (i *authInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}

type StreamExecOptions struct {
	Cmd      string
	Args     []string
	Cwd      string
	Env      map[string]string
	OnStdout func(data []byte)
	OnStderr func(data []byte)
	Stdin    bool
}

type StreamExecHandle struct {
	stream    *connect.ServerStreamForClient[process.StartResponse]
	pid       uint32
	onStdout  func([]byte)
	onStderr  func([]byte)
	done      chan struct{}
	exitCode  int32
	exitError string
	err       error
}

func (c *StreamingClient) ExecStream(ctx context.Context, opts StreamExecOptions) (*StreamExecHandle, error) {
	envs := make(map[string]string)
	for k, v := range opts.Env {
		envs[k] = v
	}

	req := &process.StartRequest{
		Process: &process.ProcessConfig{
			Cmd:  opts.Cmd,
			Args: opts.Args,
			Envs: envs,
			Cwd:  &opts.Cwd,
		},
		Stdin: &opts.Stdin,
	}

	stream, err := c.processClient.Start(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, fmt.Errorf("start process: %w", err)
	}

	handle := &StreamExecHandle{
		stream:   stream,
		onStdout: opts.OnStdout,
		onStderr: opts.OnStderr,
		done:     make(chan struct{}),
	}

	go handle.consumeEvents()

	return handle, nil
}

func (h *StreamExecHandle) consumeEvents() {
	defer close(h.done)

	for h.stream.Receive() {
		resp := h.stream.Msg()
		if resp.Event == nil {
			continue
		}

		switch ev := resp.Event.Event.(type) {
		case *process.ProcessEvent_Start:
			h.pid = ev.Start.Pid

		case *process.ProcessEvent_Data:
			switch out := ev.Data.Output.(type) {
			case *process.ProcessEvent_DataEvent_Stdout:
				if h.onStdout != nil {
					h.onStdout(out.Stdout)
				}
			case *process.ProcessEvent_DataEvent_Stderr:
				if h.onStderr != nil {
					h.onStderr(out.Stderr)
				}
			}

		case *process.ProcessEvent_End:
			h.exitCode = ev.End.ExitCode
			if ev.End.Error != nil {
				h.exitError = *ev.End.Error
			}
			return

		case *process.ProcessEvent_Keepalive:
			continue
		}
	}

	if err := h.stream.Err(); err != nil {
		h.err = err
	}
}

func (h *StreamExecHandle) PID() uint32 {
	return h.pid
}

func (h *StreamExecHandle) Wait() (exitCode int32, exitError string, err error) {
	<-h.done
	return h.exitCode, h.exitError, h.err
}

func (h *StreamExecHandle) Close() error {
	return h.stream.Close()
}

func (c *StreamingClient) ListProcesses(ctx context.Context) ([]*process.ProcessInfo, error) {
	resp, err := c.processClient.List(ctx, connect.NewRequest(&process.ListRequest{}))
	if err != nil {
		return nil, err
	}
	return resp.Msg.Processes, nil
}

func (c *StreamingClient) SendSignal(ctx context.Context, pid uint32, signal process.Signal) error {
	_, err := c.processClient.SendSignal(ctx, connect.NewRequest(&process.SendSignalRequest{
		Process: &process.ProcessSelector{
			Selector: &process.ProcessSelector_Pid{Pid: pid},
		},
		Signal: signal,
	}))
	return err
}

func (c *StreamingClient) Kill(ctx context.Context, pid uint32) error {
	return c.SendSignal(ctx, pid, process.Signal_SIGNAL_SIGKILL)
}

func (c *StreamingClient) SendStdin(ctx context.Context, pid uint32, data []byte) error {
	_, err := c.processClient.SendInput(ctx, connect.NewRequest(&process.SendInputRequest{
		Process: &process.ProcessSelector{
			Selector: &process.ProcessSelector_Pid{Pid: pid},
		},
		Input: &process.ProcessInput{
			Input: &process.ProcessInput_Stdin{Stdin: data},
		},
	}))
	return err
}
