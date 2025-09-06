package strava

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	windowShortMinutes = 15
)

// RateLimit is the struct used for the `RateLimiting` global that is
// updated after every request.
type RateLimit struct {
	lock          sync.RWMutex
	RequestTime   time.Time
	WindowShort   string
	WindowLong    string
	LimitShort    int
	LimitLong     int
	UsageShort    int
	UsageLong     int
	UsageReserved int
}

// Exceeded should be called as `strava.RateLimiting.Exceeded() to determine if the most recent
// request exceeded the rate limit
// If the rate limit is not exceeded, 1 unit is reserved for the call you are going to execute.
// In that case you should call the Reserved-function by defer.
func (rl *RateLimit) Exceeded() bool {
	rl.lock.RLock()
	defer rl.lock.RUnlock()

	exceeded := false

	now := time.Now()
	minute := now.Minute()
	currentWindowShort := fmt.Sprintf("%v-%v", now.Hour(), minute-minute%windowShortMinutes)
	currentWindowLong := now.Format("2006-01-02")

	if currentWindowShort != rl.WindowShort {
		rl.WindowShort = currentWindowShort
		rl.UsageShort = 0
	}

	if currentWindowLong != rl.WindowLong {
		rl.WindowLong = currentWindowLong
		rl.UsageLong = 0
	}

	if !rl.RequestTime.IsZero() {
		if rl.UsageShort+rl.UsageReserved >= rl.LimitShort {
			exceeded = true
		} else if rl.UsageLong+rl.UsageReserved >= rl.LimitLong {
			exceeded = true
		}
	}

	if !exceeded {
		rl.UsageReserved++
	}

	// fmt.Printf("Limit for window %s: %v+%v/%v, %v+%v/%v\n", rl.WindowShort, rl.UsageShort, rl.UsageReserved, rl.LimitShort, rl.UsageLong, rl.UsageReserved, rl.LimitLong)
	return exceeded
}

// FractionReached returns the current faction of rate used. The greater of the
// short and long term limits. Should be called as `strava.RateLimiting.FractionReached()`
func (rl *RateLimit) FractionReached() float32 {
	rl.lock.RLock()
	defer rl.lock.RUnlock()

	var shortLimitFraction = float32(rl.UsageShort) / float32(rl.LimitShort)
	var longLimitFraction = float32(rl.UsageLong) / float32(rl.LimitLong)

	if shortLimitFraction > longLimitFraction {
		return shortLimitFraction
	} else {
		return longLimitFraction
	}
}

// ignoring error, instead will reset struct to initial values, so rate limiting is ignored
func (rl *RateLimit) updateRateLimits(resp *http.Response) {
	rl.lock.Lock()
	defer rl.lock.Unlock()

	var err error

	if resp.Header.Get("X-ReadRatelimit-Limit") == "" || resp.Header.Get("X-ReadRatelimit-Usage") == "" {
		rl.clear()
		return
	}

	s := strings.Split(resp.Header.Get("X-ReadRatelimit-Limit"), ",")
	if rl.LimitShort, err = strconv.Atoi(s[0]); err != nil {
		rl.clear()
		return
	}
	if rl.LimitLong, err = strconv.Atoi(s[1]); err != nil {
		rl.clear()
		return
	}

	s = strings.Split(resp.Header.Get("X-ReadRatelimit-Usage"), ",")
	if rl.UsageShort, err = strconv.Atoi(s[0]); err != nil {
		rl.clear()
		return
	}

	if rl.UsageLong, err = strconv.Atoi(s[1]); err != nil {
		rl.clear()
		return
	}

	rl.RequestTime = time.Now()
	return
}

func (rl *RateLimit) clear() {
	rl.RequestTime = time.Time{}
	rl.WindowShort = ""
	rl.WindowLong = ""
	rl.LimitShort = 0
	rl.LimitLong = 0
	rl.UsageShort = 0
	rl.UsageLong = 0
}

func (rl *RateLimit) Reserved() {
	rl.lock.RLock()
	defer rl.lock.RUnlock()

	rl.UsageReserved--
}
