package coding

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"station/internal/config"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	DefaultNATSURL         = "nats://localhost:4222"
	DefaultTaskSubject     = "station.coding.task"
	DefaultStreamSubject   = "station.coding.stream"
	DefaultResultSubject   = "station.coding.result"
	DefaultSessionsBucket  = "opencode-sessions"
	DefaultStateBucket     = "opencode-state"
	DefaultArtifactsBucket = "opencode-files"
	DefaultKVTTL           = 24 * time.Hour * 7
)

type NATSCodingClient struct {
	nc       *nats.Conn
	js       jetstream.JetStream
	cfg      config.CodingNATSConfig
	mu       sync.RWMutex
	sessions jetstream.KeyValue
	state    jetstream.KeyValue
}

type NATSClientOption func(*NATSCodingClient)

func NewNATSCodingClient(cfg config.CodingNATSConfig, opts ...NATSClientOption) (*NATSCodingClient, error) {
	c := &NATSCodingClient{cfg: cfg}
	for _, opt := range opts {
		opt(c)
	}

	natsURL := cfg.URL
	if natsURL == "" {
		natsURL = DefaultNATSURL
	}

	var natsOpts []nats.Option
	natsOpts = append(natsOpts, nats.Name("station-coding"))
	natsOpts = append(natsOpts, nats.ReconnectWait(2*time.Second))
	natsOpts = append(natsOpts, nats.MaxReconnects(-1))

	if cfg.CredsFile != "" {
		natsOpts = append(natsOpts, nats.UserCredentials(cfg.CredsFile))
	}

	nc, err := nats.Connect(natsURL, natsOpts...)
	if err != nil {
		return nil, fmt.Errorf("connect to NATS: %w", err)
	}
	c.nc = nc

	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("create JetStream context: %w", err)
	}
	c.js = js

	return c, nil
}

func (c *NATSCodingClient) Close() error {
	if c.nc != nil {
		c.nc.Close()
	}
	return nil
}

func (c *NATSCodingClient) IsConnected() bool {
	return c.nc != nil && c.nc.IsConnected()
}

func (c *NATSCodingClient) taskSubject() string {
	if c.cfg.Subjects.Task != "" {
		return c.cfg.Subjects.Task
	}
	return DefaultTaskSubject
}

func (c *NATSCodingClient) streamSubject(taskID string) string {
	base := c.cfg.Subjects.Stream
	if base == "" {
		base = DefaultStreamSubject
	}
	return fmt.Sprintf("%s.%s", base, taskID)
}

func (c *NATSCodingClient) resultSubject(taskID string) string {
	base := c.cfg.Subjects.Result
	if base == "" {
		base = DefaultResultSubject
	}
	return fmt.Sprintf("%s.%s", base, taskID)
}

func (c *NATSCodingClient) sessionsBucket() string {
	if c.cfg.KV.Sessions != "" {
		return c.cfg.KV.Sessions
	}
	return DefaultSessionsBucket
}

func (c *NATSCodingClient) stateBucket() string {
	if c.cfg.KV.State != "" {
		return c.cfg.KV.State
	}
	return DefaultStateBucket
}

type CodingTask struct {
	TaskID    string        `json:"taskID"`
	Session   TaskSession   `json:"session"`
	Workspace TaskWorkspace `json:"workspace"`
	Prompt    string        `json:"prompt"`
	Agent     string        `json:"agent,omitempty"`
	Model     *TaskModel    `json:"model,omitempty"`
	Timeout   int           `json:"timeout,omitempty"`
	Callback  TaskCallback  `json:"callback"`
}

type TaskSession struct {
	Name     string `json:"name"`
	Continue bool   `json:"continue"`
}

type TaskWorkspace struct {
	Name string         `json:"name"`
	Git  *TaskGitConfig `json:"git,omitempty"`
}

type TaskGitConfig struct {
	URL         string `json:"url"`
	Branch      string `json:"branch,omitempty"`
	Ref         string `json:"ref,omitempty"`
	Token       string `json:"token,omitempty"`
	TokenFromKV string `json:"tokenFromKV,omitempty"`
	Pull        bool   `json:"pull"`
}

type TaskModel struct {
	ProviderID string `json:"providerID"`
	ModelID    string `json:"modelID"`
}

type TaskCallback struct {
	StreamSubject string `json:"streamSubject"`
	ResultSubject string `json:"resultSubject"`
}

type CodingStreamEvent struct {
	TaskID    string              `json:"taskID"`
	Seq       int                 `json:"seq"`
	Timestamp string              `json:"timestamp"`
	Type      string              `json:"type"`
	Content   string              `json:"content,omitempty"`
	Tool      *StreamEventTool    `json:"tool,omitempty"`
	Git       *StreamEventGit     `json:"git,omitempty"`
	Session   *StreamEventSession `json:"session,omitempty"`
}

type StreamEventTool struct {
	Name     string                 `json:"name"`
	CallID   string                 `json:"callID"`
	Args     map[string]interface{} `json:"args,omitempty"`
	Output   string                 `json:"output,omitempty"`
	Duration int                    `json:"duration,omitempty"`
}

type StreamEventGit struct {
	URL    string `json:"url,omitempty"`
	Branch string `json:"branch,omitempty"`
	Commit string `json:"commit,omitempty"`
}

type StreamEventSession struct {
	Name       string `json:"name"`
	OpencodeID string `json:"opencodeID"`
}

type CodingResult struct {
	TaskID    string          `json:"taskID"`
	Status    string          `json:"status"`
	Result    string          `json:"result,omitempty"`
	Error     string          `json:"error,omitempty"`
	ErrorType string          `json:"errorType,omitempty"`
	Session   ResultSession   `json:"session"`
	Workspace ResultWorkspace `json:"workspace"`
	Metrics   ResultMetrics   `json:"metrics"`
}

type ResultSession struct {
	Name         string `json:"name"`
	OpencodeID   string `json:"opencodeID"`
	MessageCount int    `json:"messageCount"`
}

type ResultWorkspace struct {
	Name string              `json:"name"`
	Path string              `json:"path"`
	Git  *ResultWorkspaceGit `json:"git,omitempty"`
}

type ResultWorkspaceGit struct {
	Branch string `json:"branch"`
	Commit string `json:"commit"`
	Dirty  bool   `json:"dirty"`
}

type ResultMetrics struct {
	Duration         int `json:"duration"`
	PromptTokens     int `json:"promptTokens,omitempty"`
	CompletionTokens int `json:"completionTokens,omitempty"`
	ToolCalls        int `json:"toolCalls"`
	StreamEvents     int `json:"streamEvents"`
}

type SessionState struct {
	SessionName   string           `json:"sessionName"`
	OpencodeID    string           `json:"opencodeID"`
	WorkspaceName string           `json:"workspaceName"`
	WorkspacePath string           `json:"workspacePath"`
	Created       string           `json:"created"`
	LastUsed      string           `json:"lastUsed"`
	MessageCount  int              `json:"messageCount"`
	Git           *SessionStateGit `json:"git,omitempty"`
}

type SessionStateGit struct {
	URL        string `json:"url"`
	Branch     string `json:"branch"`
	LastCommit string `json:"lastCommit"`
}

func (c *NATSCodingClient) PublishTask(ctx context.Context, task *CodingTask) error {
	data, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("marshal task: %w", err)
	}

	if err := c.nc.Publish(c.taskSubject(), data); err != nil {
		return fmt.Errorf("publish task: %w", err)
	}

	return nil
}

type TaskExecution struct {
	client    *NATSCodingClient
	task      *CodingTask
	streamSub *nats.Subscription
	resultSub *nats.Subscription
	events    chan *CodingStreamEvent
	result    chan *CodingResult
	done      chan struct{}
	mu        sync.Mutex
	closed    bool
}

func (c *NATSCodingClient) ExecuteTask(ctx context.Context, task *CodingTask) (*TaskExecution, error) {
	task.Callback.StreamSubject = c.streamSubject(task.TaskID)
	task.Callback.ResultSubject = c.resultSubject(task.TaskID)

	exec := &TaskExecution{
		client: c,
		task:   task,
		events: make(chan *CodingStreamEvent, 100),
		result: make(chan *CodingResult, 1),
		done:   make(chan struct{}),
	}

	streamSub, err := c.nc.Subscribe(task.Callback.StreamSubject, func(msg *nats.Msg) {
		var event CodingStreamEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			return
		}
		select {
		case exec.events <- &event:
		default:
		}
	})
	if err != nil {
		return nil, fmt.Errorf("subscribe to stream: %w", err)
	}
	exec.streamSub = streamSub

	resultSub, err := c.nc.Subscribe(task.Callback.ResultSubject, func(msg *nats.Msg) {
		var result CodingResult
		if err := json.Unmarshal(msg.Data, &result); err != nil {
			return
		}
		select {
		case exec.result <- &result:
		default:
		}
		exec.close()
	})
	if err != nil {
		streamSub.Unsubscribe()
		return nil, fmt.Errorf("subscribe to result: %w", err)
	}
	exec.resultSub = resultSub

	if err := c.PublishTask(ctx, task); err != nil {
		streamSub.Unsubscribe()
		resultSub.Unsubscribe()
		return nil, err
	}

	return exec, nil
}

func (e *TaskExecution) Events() <-chan *CodingStreamEvent {
	return e.events
}

func (e *TaskExecution) Result() <-chan *CodingResult {
	return e.result
}

func (e *TaskExecution) Done() <-chan struct{} {
	return e.done
}

func (e *TaskExecution) close() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return
	}
	e.closed = true

	if e.streamSub != nil {
		e.streamSub.Unsubscribe()
	}
	if e.resultSub != nil {
		e.resultSub.Unsubscribe()
	}
	close(e.done)
}

func (e *TaskExecution) Cancel() {
	e.close()
}

func (e *TaskExecution) Wait(ctx context.Context) (*CodingResult, error) {
	select {
	case result := <-e.result:
		return result, nil
	case <-ctx.Done():
		e.Cancel()
		return nil, ctx.Err()
	}
}

func (c *NATSCodingClient) getOrCreateKV(ctx context.Context, bucket string) (jetstream.KeyValue, error) {
	kv, err := c.js.KeyValue(ctx, bucket)
	if err == nil {
		return kv, nil
	}

	kv, err = c.js.CreateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket:      bucket,
		Description: fmt.Sprintf("Station coding %s", bucket),
		TTL:         DefaultKVTTL,
	})
	if err != nil {
		return nil, fmt.Errorf("create KV bucket %s: %w", bucket, err)
	}

	return kv, nil
}

func (c *NATSCodingClient) GetSession(ctx context.Context, name string) (*SessionState, error) {
	c.mu.Lock()
	if c.sessions == nil {
		kv, err := c.getOrCreateKV(ctx, c.sessionsBucket())
		if err != nil {
			c.mu.Unlock()
			return nil, err
		}
		c.sessions = kv
	}
	kv := c.sessions
	c.mu.Unlock()

	entry, err := kv.Get(ctx, name)
	if err != nil {
		if err == jetstream.ErrKeyNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get session %s: %w", name, err)
	}

	var state SessionState
	if err := json.Unmarshal(entry.Value(), &state); err != nil {
		return nil, fmt.Errorf("unmarshal session %s: %w", name, err)
	}

	return &state, nil
}

func (c *NATSCodingClient) SaveSession(ctx context.Context, state *SessionState) error {
	c.mu.Lock()
	if c.sessions == nil {
		kv, err := c.getOrCreateKV(ctx, c.sessionsBucket())
		if err != nil {
			c.mu.Unlock()
			return err
		}
		c.sessions = kv
	}
	kv := c.sessions
	c.mu.Unlock()

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	if _, err := kv.Put(ctx, state.SessionName, data); err != nil {
		return fmt.Errorf("put session %s: %w", state.SessionName, err)
	}

	return nil
}

func (c *NATSCodingClient) DeleteSession(ctx context.Context, name string) error {
	c.mu.Lock()
	if c.sessions == nil {
		kv, err := c.getOrCreateKV(ctx, c.sessionsBucket())
		if err != nil {
			c.mu.Unlock()
			return err
		}
		c.sessions = kv
	}
	kv := c.sessions
	c.mu.Unlock()

	if err := kv.Delete(ctx, name); err != nil && err != jetstream.ErrKeyNotFound {
		return fmt.Errorf("delete session %s: %w", name, err)
	}

	return nil
}

func (c *NATSCodingClient) GetState(ctx context.Context, key string) ([]byte, error) {
	c.mu.Lock()
	if c.state == nil {
		kv, err := c.getOrCreateKV(ctx, c.stateBucket())
		if err != nil {
			c.mu.Unlock()
			return nil, err
		}
		c.state = kv
	}
	kv := c.state
	c.mu.Unlock()

	entry, err := kv.Get(ctx, key)
	if err != nil {
		if err == jetstream.ErrKeyNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get state %s: %w", key, err)
	}

	return entry.Value(), nil
}

func (c *NATSCodingClient) SetState(ctx context.Context, key string, value []byte) error {
	c.mu.Lock()
	if c.state == nil {
		kv, err := c.getOrCreateKV(ctx, c.stateBucket())
		if err != nil {
			c.mu.Unlock()
			return err
		}
		c.state = kv
	}
	kv := c.state
	c.mu.Unlock()

	if _, err := kv.Put(ctx, key, value); err != nil {
		return fmt.Errorf("put state %s: %w", key, err)
	}

	return nil
}

func (c *NATSCodingClient) DeleteState(ctx context.Context, key string) error {
	c.mu.Lock()
	if c.state == nil {
		kv, err := c.getOrCreateKV(ctx, c.stateBucket())
		if err != nil {
			c.mu.Unlock()
			return err
		}
		c.state = kv
	}
	kv := c.state
	c.mu.Unlock()

	if err := kv.Delete(ctx, key); err != nil && err != jetstream.ErrKeyNotFound {
		return fmt.Errorf("delete state %s: %w", key, err)
	}

	return nil
}
