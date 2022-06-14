package contrib

import "agent/internal/pkg/global"

var enabledStreams = []func() (global.Stream, error){
	// newFileStream,
}

func GetStreams() ([]global.Stream, error) {
	streams := []global.Stream{}
	for _, streamFunc := range enabledStreams {
		stream, err := streamFunc()
		if err != nil {
			return nil, err
		}
		streams = append(streams, stream)
	}

	return streams, nil
}
