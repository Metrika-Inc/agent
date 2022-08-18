package transport

import (
	"context"
	"crypto/tls"
	"fmt"
	"sync"
	"time"

	"agent/api/v1/model"
	"agent/internal/pkg/buf"
	"agent/internal/pkg/global"
	"agent/pkg/timesync"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

var (
	// AgentUUIDHeaderName GRPC metadata key name for agent hostname
	AgentUUIDHeaderName = "x-agent-uuid"

	// AgentAPIKeyHeaderName GRPC metadata key name for platform API key
	AgentAPIKeyHeaderName = "x-api-key"
)

const (
	defaultConnectTimeout  = 3 * time.Second
	defaultTransmitTimeout = 5 * time.Second
)

type PlatformGRPCConf struct {
	// URL where the publisher will be pushing metrics to.
	URL string

	// UUID the agent's unique identifier
	UUID string

	// APIKey authentication key for the Metrika platform
	APIKey string

	// ConnectTimeout timeout period for platform connection to be established.
	ConnectTimeout time.Duration

	// TransmitTimeout timeout period for requests to the platform
	TransmitTimeout time.Duration

	// AgentService overrides the agent client (only used by tests)
	AgentService model.AgentClient
}

type PlatformGRPC struct {
	PlatformGRPCConf

	grpcConn *grpc.ClientConn
	metadata metadata.MD
	lock     *sync.RWMutex
}

func NewPlatformGRPC(conf PlatformGRPCConf) (*PlatformGRPC, error) {
	// Anything put here will be transmitted as request headers.
	md := metadata.Pairs(AgentUUIDHeaderName, conf.UUID, AgentAPIKeyHeaderName, conf.APIKey)

	if conf.UUID == "" || conf.APIKey == "" {
		return nil, fmt.Errorf("invalid platform configuration (uuid or api key missing): %+v", conf)
	}

	if conf.ConnectTimeout == 0 {
		conf.ConnectTimeout = defaultConnectTimeout
	}

	if conf.TransmitTimeout == 0 {
		conf.TransmitTimeout = defaultTransmitTimeout
	}

	return &PlatformGRPC{PlatformGRPCConf: conf, metadata: md, lock: &sync.RWMutex{}}, nil
}

func (t *PlatformGRPC) Publish(data []*model.Message) (int64, error) {
	t.lock.Lock()
	defer t.lock.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 2*t.TransmitTimeout)
	defer cancel()

	metrikaMsg := model.PlatformMessage{
		AgentUUID: t.UUID,
		Data:      data,
	}
	if t.AgentService == nil {
		if err := t.connect(); err != nil {
			return 0, err
		}
	}

	ctx = metadata.NewOutgoingContext(ctx, t.metadata)

	// Transmit to platform. Failure here signifies transient error.
	resp, err := t.AgentService.Transmit(ctx, &metrikaMsg)
	if err != nil {
		zap.S().Errorw("failed to transmit to the platform", zap.Error(err), "addr", t.URL)

		// mark service for repair
		t.AgentService = nil

		return 0, err
	}

	return resp.Timestamp, nil
}

func (t *PlatformGRPC) connect() error {
	var err error

	ctx, cancel := context.WithTimeout(context.Background(), t.ConnectTimeout)
	defer cancel()

	tlsConfig := &tls.Config{InsecureSkipVerify: false}
	t.grpcConn, err = grpc.DialContext(ctx, t.URL,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)), grpc.WithBlock())
	if err != nil {
		return fmt.Errorf("grpc connect error: %v (%s)", err, t.URL)
	}

	t.AgentService = model.NewAgentClient(t.grpcConn)

	return nil
}

func (t *PlatformGRPC) PublishFunc(b buf.ItemBatch) error {
	batch := make([]*model.Message, 0, len(b)+1)
	for _, item := range b {
		m, ok := item.Data.(*model.Message)
		if !ok {
			zap.S().Warnf("unrecognised type %T", item.Data)

			// ignore
			continue
		}
		batch = append(batch, m)
	}

	errCh := make(chan error)
	go func() {
		timestamp, err := t.Publish(batch)
		if err != nil {
			PlatformPublishErrors.Inc()
			global.AgentRuntimeState.SetPublishState(global.PlatformStateDown)

			errCh <- err
			return
		}
		if timestamp != 0 {
			timesync.Refresh(timestamp)
		}
		MetricsPublishedCnt.Add(float64(len(batch)))
		global.AgentRuntimeState.SetPublishState(global.PlatformStateUp)

		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err // might be nil
	case <-time.After(t.TransmitTimeout):
		return fmt.Errorf("publish goroutine timeout (%v)", t.TransmitTimeout)
	}
}
