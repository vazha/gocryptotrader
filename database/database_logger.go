package database

import log "github.com/vazha/gocryptotrader/logger"

// Logger implements io.Writer interface to redirect SQLBoiler debug output to GCT logger
type Logger struct{}

// Write takes input and sends to GCT logger
func (l Logger) Write(p []byte) (n int, err error) {
	log.Debugf(log.DatabaseMgr, "SQL: %s", p)
	return 0, nil
}
