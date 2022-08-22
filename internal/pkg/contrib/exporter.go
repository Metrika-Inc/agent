package contrib

import (
	"agent/internal/pkg/global"

	"go.uber.org/zap"
)

var enabledExporters = map[string]func() (global.Exporter, error){
	"file_stream_exporter": newFileStream,
}

func GetExporters() ([]global.Exporter, error) {
	exporters := []global.Exporter{}
	for name, exporterSetup := range enabledExporters {
		exporter, err := exporterSetup()
		if err != nil {
			zap.S().Errorw("failed initializing exporter", "exporter_name", name)
			return nil, err
		}
		exporters = append(exporters, exporter)
	}

	return exporters, nil
}
