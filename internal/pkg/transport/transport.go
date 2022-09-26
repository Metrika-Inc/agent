// Copyright 2022 Metrika Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package transport

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"sync"
	"time"

	"agent/api/v1/model"
	"agent/internal/pkg/buf"
	"agent/internal/pkg/global"
	"agent/pkg/timesync"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
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

// PlatformGRPCConf PlatformGRPC configuration struct.
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

	// Dialer
	Dialer func(context.Context, string) (net.Conn, error)
}

// PlatformGRPC implements a GRPC client for publishing data to the
// Metrika platform.
type PlatformGRPC struct {
	PlatformGRPCConf

	grpcConn *grpc.ClientConn
	metadata metadata.MD
	lock     *sync.RWMutex
}

// NewPlatformGRPC platform transport constructor.
func NewPlatformGRPC(conf PlatformGRPCConf) (*PlatformGRPC, error) {
	// Anything put here will be transmitted as request headers.
	md := metadata.Pairs(AgentUUIDHeaderName, conf.UUID, AgentAPIKeyHeaderName, conf.APIKey)

	if conf.UUID == "" || conf.APIKey == "" || conf.URL == "" {
		return nil, fmt.Errorf("invalid platform configuration (check uuid, api key or url): %+v", conf)
	}

	if conf.ConnectTimeout == 0 {
		conf.ConnectTimeout = defaultConnectTimeout
	}

	if conf.TransmitTimeout == 0 {
		conf.TransmitTimeout = defaultTransmitTimeout
	}

	return &PlatformGRPC{PlatformGRPCConf: conf, metadata: md, lock: &sync.RWMutex{}}, nil
}

// Publish publishes a slice of messages to the platform by invoking
// metrika.agent/Transmit.
func (t *PlatformGRPC) Publish(data []*model.Message) (int64, error) {
	t.lock.Lock()
	defer t.lock.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), t.TransmitTimeout)
	defer cancel()

	metrikaMsg := model.PlatformMessage{
		AgentUUID: t.UUID,
		Data:      data,
		Protocol:  global.BlockchainNode.Protocol(),
		Network:   global.BlockchainNode.Network(),
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
		t.grpcConn.Close()

		return 0, err
	}

	return resp.Timestamp, nil
}

func (t *PlatformGRPC) connect() error {
	var err error

	ctx, cancel := context.WithTimeout(context.Background(), t.ConnectTimeout)
	defer cancel()

	tlsConfig := &tls.Config{InsecureSkipVerify: false}

	if t.Dialer != nil {
		ctxDialer := grpc.WithContextDialer(t.Dialer)
		t.grpcConn, err = grpc.DialContext(ctx, t.URL, grpc.WithTransportCredentials(insecure.NewCredentials()), ctxDialer)
	} else {
		t.grpcConn, err = grpc.DialContext(ctx, t.URL, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)), grpc.WithBlock())
	}
	if err != nil {
		return fmt.Errorf("grpc connect error: %v (%s)", err, t.URL)
	}

	t.AgentService = model.NewAgentClient(t.grpcConn)

	return nil
}

// PublishFunc is a callback function used by the agent buffer for
// data publish.
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

	errCh := make(chan error, 1)
	go func() {
		timestamp, err := t.Publish(batch)
		if err != nil {
			platformPublishErrors.Inc()
			global.AgentRuntimeState.SetPublishState(global.PlatformStateDown)

			errCh <- err
			return
		}
		if timestamp != 0 {
			timesync.Refresh(timestamp)
		}
		metricsPublishedCnt.Add(float64(len(batch)))
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
