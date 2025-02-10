package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"sync"
	"time"
)

var version = "1.0.0"

// Global debug flag.
var debug bool

// Global password used for authenticating communications on the server.
var authPassword string

// debugLog prints a log message only if debug is enabled.
func debugLog(format string, v ...interface{}) {
	if debug {
		log.Printf("[DEBUG] "+format, v...)
	}
}

// Job represents a job to be executed.
type Job struct {
	ID      int    `json:"id"`
	Command string `json:"command"`
	Timeout int    `json:"timeout"` // in seconds
}

//
// Server implementation
//

// jobQueue holds jobs in memory.
type jobQueue struct {
	sync.Mutex
	queue  []*Job
	nextID int // auto increment ID for jobs
}

func newJobQueue() *jobQueue {
	return &jobQueue{
		queue:  make([]*Job, 0),
		nextID: 1,
	}
}

// addJob appends a new job to the queue.
func (q *jobQueue) addJob(j *Job) {
	q.Lock()
	defer q.Unlock()
	j.ID = q.nextID
	q.nextID++
	q.queue = append(q.queue, j)
	debugLog("Job added to queue: %+v", j)
}

// getJobs pops up to n jobs from the queue.
func (q *jobQueue) getJobs(n int) []*Job {
	q.Lock()
	defer q.Unlock()
	if n > len(q.queue) {
		n = len(q.queue)
	}
	jobs := q.queue[:n]
	q.queue = q.queue[n:]
	debugLog("Fetched %d job(s) from queue; %d remaining", n, len(q.queue))
	return jobs
}

var (
	// globalQueue holds jobs in memory.
	globalQueue = newJobQueue()
)

// checkAuth verifies if the request has the correct password (if one is set).
func checkAuth(r *http.Request) bool {
	if authPassword == "" {
		return true
	}
	pass := r.Header.Get("X-Job-Password")
	return pass == authPassword
}

func jobsHandler(w http.ResponseWriter, r *http.Request) {
	if !checkAuth(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		debugLog("Unauthorized request on /jobs")
		return
	}

	switch r.Method {
	case "POST":
		var job Job
		if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			debugLog("Invalid JSON payload: %v", err)
			return
		}
		if job.Command == "" || job.Timeout <= 0 {
			http.Error(w, "Missing command or invalid timeout", http.StatusBadRequest)
			debugLog("Missing command or invalid timeout in job: %+v", job)
			return
		}
		globalQueue.addJob(&job)
		log.Printf("Job added: %+v\n", job)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(job); err != nil {
			log.Printf("Error encoding response: %v", err)
		}
	case "GET":
		http.Error(w, "GET not allowed on /jobs", http.StatusMethodNotAllowed)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func pollHandler(w http.ResponseWriter, r *http.Request) {
	if !checkAuth(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		debugLog("Unauthorized request on /poll")
		return
	}

	// Read free slots requested by the worker.
	slotsStr := r.URL.Query().Get("slots")
	if slotsStr == "" {
		http.Error(w, "Missing 'slots' parameter", http.StatusBadRequest)
		debugLog("Poll request missing 'slots' parameter")
		return
	}
	slots, err := strconv.Atoi(slotsStr)
	if err != nil || slots < 1 {
		http.Error(w, "Invalid 'slots' parameter", http.StatusBadRequest)
		debugLog("Invalid 'slots' parameter: %v", slotsStr)
		return
	}
	debugLog("Polling for %d job(s)", slots)
	jobs := globalQueue.getJobs(slots)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(jobs); err != nil {
		debugLog("Error encoding jobs response: %v", err)
	}
}

func runServer() {
	// Server flags.
	port := flag.Int("port", 8080, "port for the server")
	pass := flag.String("password", "", "password for authenticating requests")
	debugFlag := flag.Bool("debug", false, "enable debug logging")
	flag.Parse()
	debug = *debugFlag
	authPassword = *pass

	if debug {
		log.Printf("Debug mode enabled on server")
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs", jobsHandler)
	mux.HandleFunc("/poll", pollHandler)

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Server starting on %s...\n", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

//
// Worker implementation
//

// jobRunner executes a job with the given command and timeout.
func jobRunner(job *Job) {
	log.Printf("Starting job %d: %s (timeout=%ds)\n", job.ID, job.Command, job.Timeout)
	debugLog("Job %d: setting up context with %ds timeout", job.ID, job.Timeout)

	// Create a context with timeout.
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(job.Timeout)*time.Second)
	defer cancel()

	// Prepare the command. It will be executed using "sh -c <command>".
	cmd := exec.CommandContext(ctx, "nice", "-n", "19", "sh", "-c", job.Command)
	debugLog("Job %d: command prepared: %v", job.ID, cmd.Args)

	// Redirect output.
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			log.Printf("Job %d timed out\n", job.ID)
			debugLog("Job %d context deadline exceeded", job.ID)
		} else {
			log.Printf("Job %d finished with error: %v\n", job.ID, err)
			debugLog("Job %d error details: %v", job.ID, err)
		}
	} else {
		log.Printf("Job %d finished successfully\n", job.ID)
		debugLog("Job %d finished without error", job.ID)
	}
}

func runWorker() {
	// Worker flags.
	serverURL := flag.String("server", "http://localhost:8080", "URL of the job server")
	slots := flag.Int("slots", runtime.NumCPU(), "number of concurrent job slots")
	pollInt := flag.Int("poll", 1, "poll interval in seconds")
	pass := flag.String("password", "", "password for authenticating with the server")
	debugFlag := flag.Bool("debug", false, "enable debug logging")
	flag.Parse()
	debug = *debugFlag

	log.Printf("Worker starting with %d slots (server: %s)\n", *slots, *serverURL)
	debugLog("Worker flags: slots=%d, poll interval=%ds", *slots, *pollInt)

	// Semaphore channel to limit concurrency.
	sem := make(chan struct{}, *slots)

	// getFreeSlots returns the number of free job slots.
	getFreeSlots := func() int {
		free := *slots - len(sem)
		debugLog("Currently, %d free slot(s) available", free)
		return free
	}

	client := &http.Client{}
	for {
		freeSlots := getFreeSlots()
		if freeSlots > 0 {
			pollURL := fmt.Sprintf("%s/poll?slots=%d", *serverURL, freeSlots)
			debugLog("Polling URL: %s", pollURL)

			// Create a new request so we can add a header.
			req, err := http.NewRequest("GET", pollURL, nil)
			if err != nil {
				log.Printf("Error creating poll request: %v", err)
				continue
			}
			if *pass != "" {
				req.Header.Set("X-Job-Password", *pass)
			}

			resp, err := client.Do(req)
			if err != nil {
				log.Printf("Error polling server: %v", err)
				debugLog("HTTP GET error: %v", err)
			} else {
				var jobs []*Job
				if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
					log.Printf("Error decoding jobs: %v", err)
					debugLog("Decoding jobs error: %v", err)
				} else {
					debugLog("Received %d job(s) from server", len(jobs))
					for _, job := range jobs {
						sem <- struct{}{}
						go func(j *Job) {
							defer func() {
								<-sem
								debugLog("Job %d: slot freed", j.ID)
							}()
							debugLog("Job %d: starting execution", j.ID)
							jobRunner(j)
						}(job)
					}
				}
				resp.Body.Close()
			}
		}
		time.Sleep(time.Duration(*pollInt) * time.Second)
	}
}

//
// Add job implementation
//

// customReader is an alternative implementation of io.ReadCloser.
type customReader struct {
	data []byte
	pos  int
}

func newCustomReader(b []byte) io.ReadCloser {
	return &customReader{
		data: b,
		pos:  0,
	}
}

func (r *customReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func (r *customReader) Close() error {
	return nil
}

func runAddJob() {
	// Flags for the add subcommand.
	serverURL := flag.String("server", "http://localhost:8080", "URL of the job server")
	cmdStr := flag.String("cmd", "", "shell command to execute")
	timeout := flag.Int("timeout", 10, "timeout in seconds")
	pass := flag.String("password", "", "password for authenticating with the server")
	debugFlag := flag.Bool("debug", false, "enable debug logging")
	flag.Parse()
	debug = *debugFlag

	if debug {
		log.Printf("Debug mode enabled on add command")
	}

	if *cmdStr == "" {
		fmt.Println("Usage: add -cmd <command> [-timeout <seconds>]")
		os.Exit(1)
	}

	job := Job{
		Command: *cmdStr,
		Timeout: *timeout,
	}
	debugLog("Adding job: %+v", job)

	jobJSON, err := json.Marshal(job)
	if err != nil {
		log.Fatalf("Error encoding job: %v", err)
	}

	url := fmt.Sprintf("%s/jobs", *serverURL)
	debugLog("Posting job to URL: %s", url)

	// Build the request with our customReader as the body.
	req, err := http.NewRequest("POST", url, newCustomReader(jobJSON))
	if err != nil {
		log.Fatalf("Error creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if *pass != "" {
		req.Header.Set("X-Job-Password", *pass)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Error posting job: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Server responded with status: %s", resp.Status)
	}

	var returnedJob Job
	if err := json.NewDecoder(resp.Body).Decode(&returnedJob); err != nil {
		log.Fatalf("Error decoding server response: %v", err)
	}

	log.Printf("Job added successfully: %+v", returnedJob)
	debugLog("Job successfully added with ID: %d", returnedJob.ID)
}

//
// Main entry point
//

func main() {
	fmt.Println("Bluengo Simple Worker Version", version)

	if len(os.Args) < 2 {
		fmt.Println("Usage: <program> [server|worker|add] [flags...]")
		os.Exit(1)
	}

	subcommand := os.Args[1]
	// Remove the subcommand from the arguments list for flag package parsing.
	os.Args = append(os.Args[:1], os.Args[2:]...)

	switch subcommand {
	case "server":
		runServer()
	case "worker":
		runWorker()
	case "add":
		runAddJob()
	default:
		fmt.Printf("Unknown subcommand: %s\n", subcommand)
		fmt.Println("Usage: <program> [server|worker|add] [flags...]")
		os.Exit(1)
	}
}
