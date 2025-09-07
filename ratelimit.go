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
	lock         sync.RWMutex
	RequestTime  time.Time
	WindowShort  string
	WindowLong   string
	LimitShort   int
	LimitLong    int
	UsageShort   int
	UsageLong    int
	UsageClaimed int
}

// ExceededAndClaim should be called as `strava.RateLimiting.ExceededAndClaim() to determine if the most recent
// request exceeded the rate limit. The function returns the number of seconds to wait for the rate-limit to be released again.
// If the rate limit is not exceeded, directly claim 1 rate-limit unit for the actual APi call you are going to execute.
// When that API call has finished, unclaim the unit by calling the Unclaim function.
// This is necessary if we do parallel calls to the client.
// Otherwise, in the time between the Exceeded function returned false and the actual execution of the API call, the rate-limit could have reached the Exceeded state, so that we get a 429 response, although Exceeded returned false a fraction earlier.
func (rl *RateLimit) ExceededAndClaim() int {
	rl.lock.RLock()
	defer rl.lock.RUnlock()

	nowUtc := time.Now().UTC()
	minute := nowUtc.Minute()
	currentWindowShort := fmt.Sprintf("%v-%v", nowUtc.Hour(), minute-minute%windowShortMinutes)
	currentWindowLong := nowUtc.Format("2006-01-02")

	if currentWindowShort != rl.WindowShort {
		rl.WindowShort = currentWindowShort
		rl.UsageShort = 0
	}

	if currentWindowLong != rl.WindowLong {
		rl.WindowLong = currentWindowLong
		rl.UsageLong = 0
	}

	if !rl.RequestTime.IsZero() {
		if rl.UsageShort+rl.UsageClaimed >= rl.LimitShort {
			windowShortSeconds := windowShortMinutes * 60
			return 5 + windowShortSeconds - (nowUtc.Minute()*60+nowUtc.Second())%windowShortSeconds
		} else if rl.UsageLong+rl.UsageClaimed >= rl.LimitLong {
			return 5 + 24*60*60 - (nowUtc.Hour()*60*60 + nowUtc.Minute()*60 + nowUtc.Second())
		}
	}

	rl.UsageClaimed++

	//fmt.Printf("Limit for window %s: %v+%v/%v, %v+%v/%v\n", rl.WindowShort, rl.UsageShort, rl.UsageClaimed, rl.LimitShort, rl.UsageLong, rl.UsageClaimed, rl.LimitLong)
	return 0
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

func (rl *RateLimit) Unclaim() {
	rl.lock.RLock()
	defer rl.lock.RUnlock()

	rl.UsageClaimed--
}
