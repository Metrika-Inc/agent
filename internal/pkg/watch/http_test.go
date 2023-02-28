package watch

import (
	"context"
	"testing"
	"time"

	"agent/internal/pkg/global"

	"github.com/stretchr/testify/require"
)

func TestHTTPWatch_URLUpdates(t *testing.T) {
	w := NewHTTPWatch(HTTPWatchConf{
		URL:         "prevurl",
		URLIndex:    1,
		URLUpdateCh: make(chan global.ConfigUpdate, 1),
		Interval:    time.Hour,
	})
	w.StartUnsafe()

	updCh := make(chan global.ConfigUpdate)
	cupdStream := global.NewConfigUpdateStream(global.ConfigUpdateStreamConf{UpdatesCh: updCh})
	err := cupdStream.Subscribe(global.PEFEndpointsKey, w.URLUpdateCh)
	require.Nil(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cupdStream.Run(ctx)

	updCh <- global.ConfigUpdate{Key: global.PEFEndpointsKey, Val: []global.PEFEndpoint{{}, {URL: "newurl"}}}
	time.Sleep(10 * time.Millisecond)
	w.Stop()
	w.wg.Wait()

	require.Equal(t, "newurl", w.URL)
}
