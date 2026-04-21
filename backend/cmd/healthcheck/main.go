package main

import (
    "fmt"
    "net/http"
    "os"
    "time"
)

func main() {
    client := &http.Client{Timeout: 2 * time.Second}
    resp, err := client.Get("http://127.0.0.1:3001/healthz")
    if err != nil {
        fmt.Fprintf(os.Stderr, "healthcheck request failed: %v\n", err)
        os.Exit(1)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        fmt.Fprintf(os.Stderr, "healthcheck returned status %d\n", resp.StatusCode)
        os.Exit(1)
    }
}
