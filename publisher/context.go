package publisher

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"agent/api/v1/model"
)

var (
	AgentUUIDContextKey    = "agentUUID"
	PlatformAddrContextKey = "platformAddr"
	StateContextKey        = "agentState"
)

func stateFromContext(ctx context.Context) (*AgentState, error) {
	key := StateContextKey
	v, ok := ctx.Value(key).(*AgentState)
	if !ok {
		return nil, fmt.Errorf("cannot get %s from ctx %v", AgentUUIDContextKey, ctx)
	}

	return v, nil
}

func stringFromContext(ctx context.Context, key string) (string, error) {
	v, ok := ctx.Value(key).(string)
	if !ok {
		return "", fmt.Errorf("cannot get %s from ctx %v", AgentUUIDContextKey, ctx)
	}

	return v, nil
}

func newHTTPRequestFromContext(ctx context.Context, msg model.MetrikaMessage) (*http.Request, error) {
	body, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	url, err := stringFromContext(ctx, PlatformAddrContextKey)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}

	return req, nil
}
