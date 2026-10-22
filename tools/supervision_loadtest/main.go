package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"
)

type result struct {
	status int
	body   []byte
}

func main() {
	base := flag.String(
		"base-url",
		"http://localhost:3000/api/v1",
		"API base URL",
	)
	supervisor := flag.Int(
		"supervisor-id",
		1,
		"target supervisor",
	)
	firstStudent := flag.Int(
		"student-start",
		1,
		"first student ID; uses n consecutive students",
	)
	n := flag.Int(
		"n",
		30,
		"number of simultaneous assignments",
	)
	flag.Parse()

	token := os.Getenv("ADMIN_TOKEN")
	if token == "" {
		fmt.Fprintln(os.Stderr, "ADMIN_TOKEN is required")
		os.Exit(2)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	start := make(chan struct{})
	results := make(chan result, *n)

	var wg sync.WaitGroup

	for i := 0; i < *n; i++ {
		wg.Add(1)

		go func(studentID int) {
			defer wg.Done()
			<-start

			body, _ := json.Marshal(
				map[string]int{"student_id": studentID},
			)

			req, err := http.NewRequest(
				http.MethodPost,
				fmt.Sprintf(
					"%s/supervisors/%d/assign",
					*base,
					*supervisor,
				),
				bytes.NewReader(body),
			)
			if err != nil {
				results <- result{
					status: 0,
					body:   []byte(err.Error()),
				}
				return
			}

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)

			resp, err := client.Do(req)
			if err != nil {
				results <- result{
					status: 0,
					body:   []byte(err.Error()),
				}
				return
			}
			defer resp.Body.Close()

			var responseBody bytes.Buffer
			_, _ = responseBody.ReadFrom(resp.Body)

			results <- result{
				status: resp.StatusCode,
				body:   responseBody.Bytes(),
			}
		}(*firstStudent + i)
	}

	startTime := time.Now()
	close(start)
	wg.Wait()
	close(results)

	counts := map[int]int{}
	for r := range results {
		counts[r.status]++
	}

	fmt.Printf(
		"assignments=%d supervisor_id=%d duration=%s\n",
		*n,
		*supervisor,
		time.Since(startTime).Round(time.Millisecond),
	)

	for code, count := range counts {
		fmt.Printf("HTTP %d: %d\n", code, count)
	}

	if counts[http.StatusCreated] != 3 ||
		counts[http.StatusConflict] != *n-3 {
		fmt.Fprintf(
			os.Stderr,
			"unexpected result: expected 3 Created and %d Conflict; CHECK/constraint errors must fail this probe\n",
			*n-3,
		)
		os.Exit(1)
	}
}