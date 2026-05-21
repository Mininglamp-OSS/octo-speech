package dsnutil

import (
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

// EnsureDSNTimeParams guarantees that parseTime=true and loc=Local are present
// in a MySQL DSN so the driver returns time.Time in the local timezone.
// loc=Local relies on TZ=Asia/Shanghai set in Dockerfile.
func EnsureDSNTimeParams(dsn string) string {
	if dsn == "" {
		return ""
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return dsn
	}
	cfg.ParseTime = true
	if !dsnHasParam(dsn, "loc") {
		cfg.Loc = time.Local
	}
	for k := range cfg.Params {
		if strings.EqualFold(k, "parsetime") {
			delete(cfg.Params, k)
		}
	}
	return cfg.FormatDSN()
}

func dsnHasParam(dsn, key string) bool {
	i := strings.IndexByte(dsn, '?')
	if i < 0 {
		return false
	}
	for _, p := range strings.Split(dsn[i+1:], "&") {
		if k, _, _ := strings.Cut(p, "="); k == key {
			return true
		}
	}
	return false
}
