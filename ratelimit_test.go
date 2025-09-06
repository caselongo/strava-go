package strava

import (
	"net/http"
	"testing"
)

func TestRateLimitUpdating(t *testing.T) {
	var resp http.Response

	resp.StatusCode = 200
	resp.Header = http.Header{"Date": []string{"Tue, 10 Oct 2013 20:11:05 GMT"}, "X-Ratelimit-Limit": []string{"600,30000"}, "X-Ratelimit-Usage": []string{"300,10000"}}

	var ratelimit = &RateLimit{}
	ratelimit.updateRateLimits(&resp)

	if ratelimit.RequestTime.IsZero() {
		t.Errorf("rate limiting should set request time")
	}

	if v := ratelimit.FractionReached(); v != 0.5 {
		t.Errorf("fraction of rate limit computed incorrectly, got %v", v)
	}

	resp.Header = http.Header{"Date": []string{"Tue, 10 Oct 2013 20:11:05 GMT"}, "X-Ratelimit-Limit": []string{"600,30000"}, "X-Ratelimit-Usage": []string{"300,27000"}}
	ratelimit.updateRateLimits(&resp)

	if ratelimit.RequestTime.IsZero() {
		t.Errorf("rate limiting should set request time")
	}

	if v := ratelimit.FractionReached(); v != 0.9 {
		t.Errorf("fraction of rate limit computed incorrectly, got %v", v)
	}

	// we'll feed it nonsense
	resp.Header = http.Header{"Date": []string{"Tue, 10 Oct 2013 20:11:05 GMT"}, "X-Ratelimit-Limit": []string{"xxx"}, "X-Ratelimit-Usage": []string{"zzz"}}
	ratelimit.updateRateLimits(&resp)

	if !ratelimit.RequestTime.IsZero() {
		t.Errorf("nonsense in rate limiting fields should set next reset to zero")
	}
}

func TestRateLimitExceeded(t *testing.T) {
	var ratelimit = &RateLimit{}
	ratelimit.LimitLong = 1
	ratelimit.UsageLong = 0

	ratelimit.LimitShort = 100
	ratelimit.UsageShort = 200

	if ratelimit.Exceeded() != true {
		t.Errorf("should have exceeded rate limit")
	}

	ratelimit.LimitShort = 200
	ratelimit.UsageShort = 100

	if ratelimit.Exceeded() == true {
		t.Errorf("should not have exceeded rate limit")
	}

	ratelimit.LimitShort = 1
	ratelimit.UsageShort = 0
	ratelimit.LimitLong = 100
	ratelimit.UsageLong = 200

	if ratelimit.Exceeded() != true {
		t.Errorf("should have exceeded rate limit")
	}

	ratelimit.LimitLong = 200
	ratelimit.UsageLong = 100

	if ratelimit.Exceeded() == true {
		t.Errorf("should not have exceeded rate limit")
	}
}
