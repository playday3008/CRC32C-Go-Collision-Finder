package main

import (
	"fmt"
	"hash/crc32"
	"os"
	"os/signal"
	"runtime"
	"time"
)

// Table of ASCII characters
var default_alphabet = []byte(
	"!\"#$%&'()*+,-./" +
		"0123456789" +
		":;<=>?@" +
		"ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
		"[\\]^_`" +
		"abcdefghijklmnopqrstuvwxyz" +
		"{|}~")

// typedef of result_t struct
type result_t struct {
	hash uint32
	str  []byte
	time time.Duration
}

var results []result_t
var crc32t *crc32.Table
var target_hash uint32
var start_time time.Time
var results_c chan result_t
var jobs_c chan bool

func signal_handler(sig os.Signal) {
	fmt.Println("Caught signal: ", sig)

	fmt.Println("Waiting for jobs to finish...")
	close(jobs_c)
	close(results_c)

	fmt.Println("Total results:", len(results))

	fmt.Println("Saving results to file...")
	// Save results to file
	f, err := os.Create("results.txt")
	if err != nil {
		fmt.Println("Error creating file:", err)
		os.Exit(1)
	}
	defer f.Close()

	for _, result := range results {
		f.WriteString(fmt.Sprintf("0x%08x\t%s\t%v\n", result.hash, result.str, result.time))
	}

	fmt.Println("Done, check results.txt!")

	os.Exit(0)
}

func combo(alphabet []byte, length int, line []byte, result chan<- result_t) {
	for _, char := range alphabet {
		line = append(line, char)
		if length <= 1 {
			hash := crc32.Checksum(line, crc32t)
			if hash == target_hash {
				var temp []byte = make([]byte, len(line)) // Will it do a memory leak?
				copy(temp, line)
				elapsed := time.Since(start_time)
				result <- result_t{hash, temp, elapsed}
			}
			line = line[:len(line)-1]
		} else {
			combo(alphabet, length-1, line, result)
			line = line[:len(line)-1]
		}
	}
}

func worker(alphabet []byte, length int, job chan<- bool, result chan<- result_t) {
	combo(alphabet, length, []byte{}, result)
	job <- true
}

func main() {
	// Ask user for hash in format 0x00000000
	fmt.Print("Enter hash: ")
	fmt.Scanf("0x%x", &target_hash)
	// Flush newline (Old C bug?)
	fmt.Scanf("\n")
	//target_hash = 0x86a072c0

	// Choose CRC32C table
	fmt.Print("Choose CRC32C table (1 - IEEE, 2 - Castagnoli, 3 - Koopman): ")
	var table int
	//table = 2
	fmt.Scanf("%d", &table)
	switch table {
	case 1:
		crc32t = crc32.MakeTable(crc32.IEEE)
	case 2:
		crc32t = crc32.MakeTable(crc32.Castagnoli)
	case 3:
		crc32t = crc32.MakeTable(crc32.Koopman)
	default:
		crc32t = crc32.MakeTable(crc32.IEEE)
	}
	// Flush newline (Old C bug?)
	fmt.Scanf("\n")

	// Set count of simultaneous goroutines
	fmt.Printf("Enter count of simultaneous goroutines (optimal: %d): ", runtime.NumCPU())
	var multi int
	//multi = 1
	fmt.Scanf("%d", &multi)
	if multi <= 0 {
		fmt.Println("Invalid count, setting to: ", runtime.NumCPU())
		multi = runtime.NumCPU()
	} else if multi > runtime.NumCPU()*10 {
		fmt.Println("Too many goroutines, setting to: ", runtime.NumCPU()*10)
		multi = runtime.NumCPU() * 10
	}
	// Flush newline (Old C bug?)
	fmt.Scanf("\n")

	// Ask user for alphabet, if empty use default
	//fmt.Print("Enter alphabet (empty for default): ")
	var alphabet []byte
	//fmt.Scanf("%s", &alphabet)
	if len(alphabet) == 0 {
		alphabet = default_alphabet
	}

	// Report settings
	fmt.Printf("Hash: 0x%08x\n", target_hash)
	fmt.Printf("Alphabet: %s\n", alphabet)
	fmt.Printf("Alphabet length: %d\n", len(alphabet))

	// Start brute force
	fmt.Println("Starting brute force...")

	// Handle Ctrl+C
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, os.Interrupt)
	go func() {
		sig := <-sigchan
		signal_handler(sig)
	}()

	results_c = make(chan result_t)
	jobs_c = make(chan bool)

	start_time = time.Now()

	var length int = 0

	// Fill our CPU cores with goroutines
	for i := 0; i < multi; i++ {
		go worker(alphabet, length, jobs_c, results_c)
		length++
	}

	go func() {
		for {
			<-jobs_c
			go worker(alphabet, length, jobs_c, results_c)
			length++
		}
	}()

	for {
		result := <-results_c
		results = append(results, result)
		fmt.Printf("0x%08x\t%s\t%v\n", result.hash, result.str, result.time)
	}
}
