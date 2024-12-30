package main

import "testing"


func TestRateLimiter(t *testing.T){
    rl := NewDefaultRateLimiter()
    ip := "127.0.0.1"
    phone := "+31612345678"
    timeout := rl.GetTimeoutSecsFor(ip, phone)

    if timeout != 0 {
        t.Fatalf("timeout should be 0 but was %v", timeout)
    }

    timeout = rl.GetTimeoutSecsFor(ip, phone)

    if timeout != 0 {
        t.Fatalf("timeout should be 0 but was %v", timeout)
    }
}
