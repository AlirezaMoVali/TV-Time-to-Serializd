package service

import "errors"

// ErrExportNotCompleted is returned when a download is requested before export finishes.
var ErrExportNotCompleted = errors.New("export not completed")
