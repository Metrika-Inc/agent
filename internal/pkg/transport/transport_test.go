package transport

import (
	"context"
	"log"
	"net"
	"testing"
	"time"

	"agent/api/v1/model"
	"agent/internal/pkg/discover"
	"agent/internal/pkg/global"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

var (
	lis        *bufconn.Listener
	mockServer = &server{}
)

// server is used to implement metrika.AgentServer
type server struct {
	model.UnimplementedAgentServer

	gotPlatformMessage *model.PlatformMessage
}

// Transmit implements metrika.AgentServer
func (s *server) Transmit(ctx context.Context, in *model.PlatformMessage) (*model.PlatformResponse, error) {
	s.gotPlatformMessage = in
	return &model.PlatformResponse{Timestamp: time.Now().UnixMilli()}, nil
}

// https://stackoverflow.com/questions/42102496/testing-a-grpc-service
func init() {
	lis = bufconn.Listen(bufSize)
	s := grpc.NewServer()
	model.RegisterAgentServer(s, mockServer)
	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Server exited with error: %v", err)
		}
	}()
}

func bufDialer(context.Context, string) (net.Conn, error) {
	return lis.Dial()
}

func TestPlatformGRPC_Publish(t *testing.T) {
	global.BlockchainNode = &discover.MockBlockchain{}

	conf := PlatformGRPCConf{
		UUID:           "agent-uuid",
		APIKey:         "agent-apikey",
		URL:            "bufnet",
		Dialer:         bufDialer,
		ConnectTimeout: 10 * time.Second,
	}
	transp, err := NewPlatformGRPC(conf)
	require.Nil(t, err)

	data := []*model.Message{
		{
			Name: "agent.node.up",
			Value: &model.Message_Event{
				Event: &model.Event{
					Name: "agent.node.up",
				},
			},
		},
	}

	_, err = transp.Publish(data)
	require.Nil(t, err)

	require.NotNil(t, mockServer.gotPlatformMessage)
	require.NotEmpty(t, mockServer.gotPlatformMessage.Network)
	require.NotEmpty(t, mockServer.gotPlatformMessage.Protocol)

	require.Equal(t, global.BlockchainNode.Network(), mockServer.gotPlatformMessage.Network)
	require.Equal(t, global.BlockchainNode.Protocol(), mockServer.gotPlatformMessage.Protocol)
}
