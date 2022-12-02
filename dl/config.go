package dl

import (
	"time"
)

type Config struct {
	RetryCount int
	MaxThread  int
	M3u8Url    string
	M3u8Path   string
	SaveName   string
	WorkDr     string
	BaseUrl    string
	Proxy      string
	Timeout    *time.Duration
}
