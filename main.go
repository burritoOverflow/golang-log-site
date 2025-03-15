package main

import (
	"bufio"
	"embed"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

//go:embed templates/*.html
var templatesFS embed.FS
var indexTemplate = template.Must(template.ParseFS(templatesFS, "templates/index.html"))

// LogWatcher watches a log file and notifies when changes to that file occur
type LogWatcher struct {
	filename     string
	lastPosition int64 // Last position read in the file
	mutex        sync.Mutex
	clients      map[chan string]bool
	clientsMutex sync.Mutex
}

func main() {
	logFilePath := flag.String("file", "", "Path to the log file to monitor")
	logDirPath := flag.String("dir", "", "Directory containing log files (will use most recent)")
	port := flag.Int("port", 8080, "Port to serve on")
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmsgprefix | log.Lmicroseconds)

	if *logFilePath != "" && *logDirPath != "" {
		log.Fatal("Only one of -file or -dir can be specified")
	}

	var targetFile string
	if *logFilePath != "" {
		targetFile = *logFilePath
	} else {
		var err error
		targetFile, err = findMostRecentLogFile(*logDirPath)
		if err != nil {
			log.Fatalf("Error finding log file: %v", err)
		}
		log.Printf("Using most recent log file: %s", targetFile)
	}

	// Verify the file exists
	if _, err := os.Stat(targetFile); os.IsNotExist(err) {
		log.Fatalf("Log file does not exist: %s", targetFile)
	}

	watcher := NewLogWatcher(targetFile)
	go watcher.Watch()

	setHandlers(targetFile, watcher)

	serverAddr := fmt.Sprintf(":%d", *port)
	log.Printf("Starting server at http://localhost%s", serverAddr)
	log.Printf("Monitoring log file: %s", targetFile)
	if err := http.ListenAndServe(serverAddr, nil); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func setHandlers(targetLogFile string, watcher *LogWatcher) {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		serveHomePage(w, r, targetLogFile)
	})
	http.HandleFunc("/logs", func(w http.ResponseWriter, r *http.Request) {
		watcher.ServeHTTP(w, r)
	})
	http.HandleFunc("/content", func(w http.ResponseWriter, r *http.Request) {
		serveInitialContent(w, r, targetLogFile)
	})
	// keep in mind this is relative to the binary location--so if you run from a different directory, it will not work
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
}

// NewLogWatcher creates a new LogWatcher for the given file
func NewLogWatcher(filename string) *LogWatcher {
	return &LogWatcher{
		filename:     filename,
		lastPosition: 0,
		clients:      make(map[chan string]bool),
	}
}

// monitor the log file for changes
func (w *LogWatcher) Watch() {
	for {
		w.checkForChanges()
		time.Sleep(300 * time.Millisecond)
	}
}

// checkForChanges checks if the log file has new content
func (w *LogWatcher) checkForChanges() {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	file, err := os.Open(w.filename)
	if err != nil {
		log.Printf("Error opening file: %v", err)

		// If the file doesn't exist, try to find the most recent log file in the same directory
		if os.IsNotExist(err) {
			dirPath := filepath.Dir(w.filename)
			log.Printf("Log file no longer exists. Checking directory %s for most recent log file", dirPath)

			newFile, err := findMostRecentLogFile(dirPath)
			if err != nil {
				log.Printf("Failed to find new log file: %v", err)
				return
			}

			if newFile != w.filename {
				log.Printf("Switching to new log file: %s", newFile)
				w.filename = newFile
				w.lastPosition = 0 // Start reading from the beginning of the new file

				// Try to open the new file
				file, err = os.Open(w.filename)
				if err != nil {
					log.Printf("Error opening new file %s: %v", file.Name(), err)
					return
				}
			} else {
				return // No new file found
			}
		} else {
			return // other error occurred
		}
	}
	defer file.Close()

	// Get file size
	info, err := file.Stat()
	if err != nil {
		log.Printf("Error getting file stats: %v", err)
		return
	}

	// If file size is smaller than last position (file was truncated)
	// or it's a new file, read from beginning
	size := info.Size()
	if size < w.lastPosition {
		w.lastPosition = 0
	}

	// If there's new content
	if size > w.lastPosition {
		// Seek to where we left off
		if _, err := file.Seek(w.lastPosition, io.SeekStart); err != nil {
			log.Printf("Error seeking in file: %v", err)
			return
		}

		// Read new lines
		scanner := bufio.NewScanner(file)
		var newLines []string
		for scanner.Scan() {
			newLines = append(newLines, scanner.Text())
		}

		if err := scanner.Err(); err != nil {
			log.Printf("Error scanning file: %v", err)
			return
		}

		// Update position
		w.lastPosition = size

		// Notify all clients
		if len(newLines) > 0 {
			w.notifyClients(newLines)
		}
	}
}

// notifyClients sends new lines to all connected clients
func (w *LogWatcher) notifyClients(lines []string) {
	w.clientsMutex.Lock()
	defer w.clientsMutex.Unlock()

	for client := range w.clients {
		for _, line := range lines {
			select {
			case client <- line:
				// Line sent successfully
			default:
				// Channel is full or closed
				delete(w.clients, client)
				close(client)
			}
		}
	}
}

func (w *LogWatcher) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	// Set headers for SSE
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Connection", "keep-alive")
	writer.Header().Set("Access-Control-Allow-Origin", "*")

	messageChan := make(chan string, 100)

	w.clientsMutex.Lock()
	w.clients[messageChan] = true
	w.clientsMutex.Unlock()

	notify := request.Context().Done()
	go func() {
		<-notify
		w.clientsMutex.Lock()
		delete(w.clients, messageChan)
		close(messageChan)
		w.clientsMutex.Unlock()
	}()

	// Send messages to client
	flusher, ok := writer.(http.Flusher)
	if !ok {
		http.Error(writer, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	for {
		select {
		case <-notify:
			return
		case msg, ok := <-messageChan:
			if !ok {
				return
			}
			// Format message as SSE
			fmt.Fprintf(writer, "data: %s\n\n", msg)
			flusher.Flush()
		}
	}
}

func serveHomePage(w http.ResponseWriter, _ *http.Request, filename string) {
	currentTime := time.Now().Format("January 2, 2006 at 3:04:05 PM MST")
	data := struct {
		Filename    string
		CurrentTime string
	}{
		Filename:    filepath.Base(filename),
		CurrentTime: currentTime,
	}

	if err := indexTemplate.Execute(w, data); err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

func serveInitialContent(w http.ResponseWriter, _ *http.Request, filename string) {
	file, err := os.Open(filename)
	if err != nil {
		http.Error(w, "Could not open log file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	w.Header().Set("Content-Type", "text/plain")
	io.Copy(w, file)
}

// findMostRecentLogFile finds the most recently modified file in the given directory
func findMostRecentLogFile(dirPath string) (string, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return "", err
	}

	if len(entries) == 0 {
		return "", fmt.Errorf("no files found in directory: %s", dirPath)
	}

	// Filter out directories and sort by modification time
	type fileInfo struct {
		path    string
		modTime time.Time
	}
	var files []fileInfo

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Skip hidden files
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		// only look for explictly labeled log files
		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".log") {
			continue
		}

		fullPath := filepath.Join(dirPath, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, fileInfo{
			path:    fullPath,
			modTime: info.ModTime(),
		})
	}

	if len(files) == 0 {
		return "", fmt.Errorf("no log files found in directory: %s", dirPath)
	}

	// Sort files by modification time (newest first)
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.After(files[j].modTime)
	})

	log.Printf("Found %d log files in directory: %s", len(files), dirPath)
	log.Printf("Most recent log file: %s", files[0].path)

	return files[0].path, nil
}
