package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bugzi/gs/internal/config"
)

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"` // role:"tool" 回填用
}

// Tool 工具的内部扁平定义（name/description/JSON Schema）；序列化由 MarshalJSON 包上 OpenAI 外壳
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// MarshalJSON 按 OpenAI 规范输出 {"type":"function","function":{...}}；
// 用 plain 逐字复制字段以避免递归调用自身
func (t Tool) MarshalJSON() ([]byte, error) {
	type plain struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Parameters  json.RawMessage `json:"parameters"`
	}
	return json.Marshal(struct {
		Type     string `json:"type"`
		Function plain  `json:"function"`
	}{"function", plain(t)})
}

type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// MarshalJSON 按 OpenAI 规范输出 {"id":...,"type":"function","function":{name,arguments}}，plain 防递归，同 Tool
func (tc ToolCall) MarshalJSON() ([]byte, error) {
	type fn struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	}
	return json.Marshal(struct {
		ID       string `json:"id"`
		Type     string `json:"type"`
		Function fn     `json:"function"`
	}{tc.ID, "function", fn{tc.Name, tc.Arguments}})
}

type Client struct {
	BaseURL     string
	APIKey      string
	Model       string
	Temperature float64
	HTTP        *http.Client
	// IdleTimeout 流式空闲超时：连续这么久没收到任何数据（含响应头）就中断；0 用默认 60s
	IdleTimeout time.Duration
}

// NewClient base_url 与 model 必填；api_key 可空（部分本地端点不需要）
func NewClient(cfg *config.Config) (*Client, error) {
	if cfg.AI.BaseURL == "" || cfg.AI.Model == "" {
		return nil, fmt.Errorf("AI 未配置：在 ~/.gs/config.toml 写入 [ai] 段的 base_url 与 model（api_key 可用环境变量 GS_API_KEY）")
	}
	return &Client{
		BaseURL:     strings.TrimRight(cfg.AI.BaseURL, "/"),
		APIKey:      cfg.AI.APIKey,
		Model:       cfg.AI.Model,
		Temperature: cfg.AI.Temperature,
		// 不设 Client.Timeout：流式生成总时长不定，超时全部交给请求 context（空闲 + 整体兜底）
		HTTP: &http.Client{},
	}, nil
}

type chatReq struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
	Stream      bool      `json:"stream"`
	Tools       []Tool    `json:"tools,omitempty"`
}

// chatChunk 兼容两种载荷：非 200 的 JSON 错误体（error 字段）与流式增量（choices[].delta）
type chatChunk struct {
	Choices []struct {
		Delta struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
	} `json:"choices"`

	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// errIdleTimeout 看门狗取消请求时携带的哨兵，文案即用户可见的报错信息
var errIdleTimeout = errors.New("AI 响应超时")

// Chat 无工具便捷形式，等价于 ChatWithTools(msgs, nil, nil)
func (c *Client) Chat(ctx context.Context, msgs []Message) (string, error) {
	m, err := c.ChatWithTools(ctx, msgs, nil, nil)
	if err != nil {
		return "", err
	}
	return m.Content, nil
}

// toolCallAcc 流式增量中单个 tool_call 的拼装缓冲（按 index 对齐）
type toolCallAcc struct{ id, name, args strings.Builder }

// ChatWithTools 调用 OpenAI 兼容 chat/completions，走流式并支持 tools。
// content 增量实时回调 onDelta（可为 nil）；tool_calls 增量按 index 静默拼装。
// 流式下响应头立即返回，超时语义为「空闲多久没新数据」+「整体兜底上限」。
// 返回的 assistant 消息可能只有 content、只有 tool_calls，或两者兼有。
func (c *Client) ChatWithTools(ctx context.Context, msgs []Message, tools []Tool, onDelta func(string)) (Message, error) {
	body, err := json.Marshal(chatReq{Model: c.Model, Messages: msgs, Temperature: c.Temperature, Stream: true, Tools: tools})
	if err != nil {
		return Message{}, err
	}
	idle := c.IdleTimeout
	if idle <= 0 {
		idle = 60 * time.Second
	}
	ctx, cancelOverall := context.WithTimeout(ctx, 10*time.Minute)
	defer cancelOverall()
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	// 空闲看门狗：连接挂起（连响应头都不回）时也能在 idle 内报错，而非等整体兜底
	var last atomic.Int64
	last.Store(time.Now().UnixNano())
	done := make(chan struct{})
	defer close(done)
	go func() {
		tick := time.NewTicker(idle / 4)
		defer tick.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-tick.C:
				if time.Since(time.Unix(0, last.Load())) > idle {
					cancel(fmt.Errorf("%w：%s 内未收到新数据", errIdleTimeout, idle))
					return
				}
			}
		}
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Message{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return Message{}, mapCause(ctx, err, "AI 请求失败")
	}
	defer resp.Body.Close()
	last.Store(time.Now().UnixNano()) // 响应头已到
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		var cr chatChunk
		msg := resp.Status
		if json.Unmarshal(raw, &cr) == nil && cr.Error != nil && cr.Error.Message != "" {
			msg = cr.Error.Message
		}
		return Message{}, fmt.Errorf("AI 请求失败(HTTP %d): %s", resp.StatusCode, msg)
	}

	var content strings.Builder
	var accs []*toolCallAcc
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		last.Store(time.Now().UnixNano())
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") || !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			break
		}
		var chunk chatChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			return Message{}, fmt.Errorf("AI 流式响应解析失败: %w", err)
		}
		if chunk.Error != nil && chunk.Error.Message != "" {
			return Message{}, fmt.Errorf("AI 请求失败: %s", chunk.Error.Message)
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta
		if delta.Content != "" {
			content.WriteString(delta.Content)
			if onDelta != nil {
				onDelta(delta.Content)
			}
		}
		for _, tc := range delta.ToolCalls {
			for len(accs) <= tc.Index {
				accs = append(accs, &toolCallAcc{})
			}
			acc := accs[tc.Index]
			if tc.ID != "" {
				acc.id.WriteString(tc.ID)
			}
			if tc.Function.Name != "" {
				acc.name.WriteString(tc.Function.Name)
			}
			acc.args.WriteString(tc.Function.Arguments)
		}
	}
	if err := scanner.Err(); err != nil {
		return Message{}, mapCause(ctx, err, "AI 流式响应读取失败")
	}

	msg := Message{Role: "assistant", Content: content.String()}
	for i, acc := range accs {
		if acc.name.Len() == 0 {
			continue
		}
		id := acc.id.String()
		if id == "" {
			id = fmt.Sprintf("call_%d", i) // 个别端点后续分片不重复 id
		}
		msg.ToolCalls = append(msg.ToolCalls, ToolCall{ID: id, Name: acc.name.String(), Arguments: acc.args.String()})
	}
	if msg.Content == "" && len(msg.ToolCalls) == 0 {
		return Message{}, errors.New("AI 响应缺少内容")
	}
	return msg, nil
}

// mapCause 看门狗触发时返回其友好原因；其余情况保留底层错误细节
func mapCause(ctx context.Context, err error, prefix string) error {
	cause := context.Cause(ctx)
	if errors.Is(cause, errIdleTimeout) {
		return cause
	}
	if errors.Is(cause, context.DeadlineExceeded) {
		return fmt.Errorf("AI 响应超时（整体上限 10 分钟）: %w", err)
	}
	return fmt.Errorf("%s: %w", prefix, err)
}
