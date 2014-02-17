// Copyright 2014 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package commands

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	barChar = "∎"
)

type Report struct {
	AvgTotal   float64
	Fastest    float64
	Slowest    float64
	Average    float64
	RPS        float64
	SuccessRPS float64

	results chan *result
	Total   time.Duration

	StatusCodeDist map[int]int
	Lats           []float64
	Errors         map[string]int
	SizeTotal      int64

	output string
}

func newReport(size int, results chan *result, output string) *Report {
	return &Report{
		StatusCodeDist: make(map[int]int),
		results:        results,
		output:         output,
		Errors:         make(map[string]int),
	}
}

func (r *Report) finalize(total time.Duration) {
	successCnt := 0
	for {
		select {
		case res := <-r.results:
			if res.err != nil {
				r.Errors[res.err.Error()]++
			} else {
				r.Lats = append(r.Lats, res.duration.Seconds())
				r.AvgTotal += res.duration.Seconds()
				r.StatusCodeDist[res.statusCode]++
				if res.contentLength > 0 {
					r.SizeTotal += res.contentLength
				}
				if res.statusCode >= 200 && res.statusCode < 300 {
					successCnt++
				}
			}
		default:
			r.Total = total
			r.RPS = float64(len(r.Lats)) / r.Total.Seconds()
			r.SuccessRPS = float64(successCnt) / r.Total.Seconds()
			r.Average = r.AvgTotal / float64(len(r.Lats))
			r.print()
			return
		}
	}
}

func (r *Report) print() {
	sort.Float64s(r.Lats)

	if r.output == "csv" {
		r.printCSV()
		return
	}

	if len(r.Lats) > 0 {
		r.Fastest = r.Lats[0]
		r.Slowest = r.Lats[len(r.Lats)-1]
		if r.output != "quiet" {
			fmt.Printf("\nSummary:\n")
			fmt.Printf("  Total:\t%4.4f secs.\n", r.Total.Seconds())
			fmt.Printf("  Slowest:\t%4.4f secs.\n", r.Slowest)
			fmt.Printf("  Fastest:\t%4.4f secs.\n", r.Fastest)
			fmt.Printf("  Average:\t%4.4f secs.\n", r.Average)
			fmt.Printf("  Requests/sec:\t%4.4f\n", r.RPS)
			if r.SizeTotal > 0 {
				fmt.Printf("  Total Data Recieved:\t%d bytes.\n", r.SizeTotal)
				fmt.Printf("  Response Size per Request:\t%d bytes.\n", r.SizeTotal/int64(len(r.Lats)))
			}
			r.printStatusCodes()
			r.printHistogram()
			r.printLatencies()
		}
	}

	if len(r.Errors) > 0 {
		r.printErrors()
	}
}

func (r *Report) printCSV() {
	for i, val := range r.Lats {
		fmt.Printf("%v,%4.4f\n", i+1, val)
	}
}

// Prints percentile latencies.
func (r *Report) printLatencies() {
	pctls := []int{10, 25, 50, 75, 90, 95, 99}
	data := make([]float64, len(pctls))
	j := 0
	for i := 0; i < len(r.Lats) && j < len(pctls); i++ {
		current := i * 100 / len(r.Lats)
		if current >= pctls[j] {
			data[j] = r.Lats[i]
			j++
		}
	}
	fmt.Printf("\nLatency distribution:\n")
	for i := 0; i < len(pctls); i++ {
		if data[i] > 0 {
			fmt.Printf("  %v%% in %4.4f secs.\n", pctls[i], data[i])
		}
	}
}

func (r *Report) printHistogram() {
	bc := 10
	buckets := make([]float64, bc+1)
	counts := make([]int, bc+1)
	bs := (r.Slowest - r.Fastest) / float64(bc)
	for i := 0; i < bc; i++ {
		buckets[i] = r.Fastest + bs*float64(i)
	}
	buckets[bc] = r.Slowest
	var bi int
	var max int
	for i := 0; i < len(r.Lats); {
		if r.Lats[i] <= buckets[bi] {
			i++
			counts[bi]++
			if max < counts[bi] {
				max = counts[bi]
			}
		} else if bi < len(buckets)-1 {
			bi++
		}
	}
	fmt.Printf("\nResponse time histogram:\n")
	for i := 0; i < len(buckets); i++ {
		// Normalize bar lengths.
		var barLen int
		if max > 0 {
			barLen = counts[i] * 40 / max
		}
		fmt.Printf("  %4.3f [%v]\t|%v\n", buckets[i], counts[i], strings.Repeat(barChar, barLen))
	}
}

// Prints status code distribution.
func (r *Report) printStatusCodes() {
	fmt.Printf("\nStatus code distribution:\n")
	for code, num := range r.StatusCodeDist {
		fmt.Printf("  [%d]\t%d responses\n", code, num)
	}
}

func (r *Report) printErrors() {
	fmt.Printf("\nError distribution:\n")
	for error, num := range r.Errors {
		fmt.Printf("  [%d]\t%s\n", num, error)
	}
}
