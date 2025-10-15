package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/go-audio/wav"
	"github.com/schollz/progressbar/v3"
	"github.com/tcolgate/mp3"
)

// Worker pool size - adjust based on your CPU cores
var numWorkers = runtime.NumCPU()

type fileJob struct {
	path  string
	index int
}

type result struct {
	index    int
	duration float64
	err      error
}

func getAudioDuration(filePath string) (float64, error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".mp3":
		return getMP3Duration(filePath)
	case ".wav":
		return getWAVDuration(filePath)
	case ".m4a":
		return getM4ADuration(filePath)
	default:
		return 0, fmt.Errorf("unsupported format: %s", ext)
	}
}

func getMP3Duration(filePath string) (float64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	decoder := mp3.NewDecoder(file)
	var duration float64
	var frame mp3.Frame
	var skipped int

	for {
		err := decoder.Decode(&frame, &skipped)
		if err != nil {
			break
		}
		duration += frame.Duration().Seconds()
	}

	return duration, nil
}

func getWAVDuration(filePath string) (float64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	decoder := wav.NewDecoder(file)
	if !decoder.IsValidFile() {
		return 0, fmt.Errorf("invalid WAV file")
	}

	duration, err := decoder.Duration()
	if err != nil {
		return 0, err
	}
	return duration.Seconds(), nil
}

func getM4ADuration(filePath string) (float64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	// Read file info to get size
	info, err := file.Stat()
	if err != nil {
		return 0, err
	}

	// Parse M4A/MP4 atoms to find duration
	// M4A files use MP4 container format with atoms/boxes
	buf := make([]byte, 8)
	var duration float64
	var timeScale uint32

	for {
		_, err := file.Read(buf)
		if err != nil {
			break
		}

		// Read atom size and type
		size := uint32(buf[0])<<24 | uint32(buf[1])<<16 | uint32(buf[2])<<8 | uint32(buf[3])
		atomType := string(buf[4:8])

		if size == 0 {
			break
		}

		// Look for 'mvhd' (movie header) atom which contains duration
		if atomType == "mvhd" {
			headerBuf := make([]byte, size-8)
			_, err := file.Read(headerBuf)
			if err != nil {
				break
			}

			// Parse mvhd atom
			version := headerBuf[0]
			if version == 0 {
				// Version 0: 32-bit values
				timeScale = uint32(headerBuf[12])<<24 | uint32(headerBuf[13])<<16 | uint32(headerBuf[14])<<8 | uint32(headerBuf[15])
				durationUnits := uint32(headerBuf[16])<<24 | uint32(headerBuf[17])<<16 | uint32(headerBuf[18])<<8 | uint32(headerBuf[19])
				if timeScale > 0 {
					duration = float64(durationUnits) / float64(timeScale)
				}
			} else if version == 1 {
				// Version 1: 64-bit values
				timeScale = uint32(headerBuf[20])<<24 | uint32(headerBuf[21])<<16 | uint32(headerBuf[22])<<8 | uint32(headerBuf[23])
				durationUnits := uint64(headerBuf[24])<<56 | uint64(headerBuf[25])<<48 | uint64(headerBuf[26])<<40 | uint64(headerBuf[27])<<32 |
					uint64(headerBuf[28])<<24 | uint64(headerBuf[29])<<16 | uint64(headerBuf[30])<<8 | uint64(headerBuf[31])
				if timeScale > 0 {
					duration = float64(durationUnits) / float64(timeScale)
				}
			}
			break
		}

		// Skip to next atom
		if size > 8 {
			_, err = file.Seek(int64(size-8), 1)
			if err != nil {
				break
			}
		}

		// Safety check to prevent infinite loops
		currentPos, _ := file.Seek(0, 1)
		if currentPos >= info.Size() {
			break
		}
	}

	if duration == 0 {
		return 0, fmt.Errorf("could not parse M4A duration")
	}

	return duration, nil
}

func worker(jobs <-chan fileJob, results chan<- result, wg *sync.WaitGroup, progress *progressbar.ProgressBar) {
	defer wg.Done()
	for job := range jobs {
		duration, err := getAudioDuration(job.path)
		if err != nil {
			results <- result{index: job.index, duration: 0, err: err}
		} else {
			results <- result{index: job.index, duration: duration, err: nil}
		}
		progress.Add(1)
	}
}

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: calculate <folder_path>")
		return
	}

	folderPath := os.Args[1]
	extensions := map[string]bool{
		".mp3":  true,
		".wav":  true,
		".ogg":  true,
		".flac": true,
		".m4a":  true,
	}

	// Resolve symlink if needed
	resolvedPath, err := filepath.EvalSymlinks(folderPath)
	if err != nil {
		fmt.Printf("Error resolving path: %v\n", err)
		return
	}

	fmt.Printf("Scanning directory: %s\n", resolvedPath)

	// Collect audio files
	var audioFiles []string
	err = filepath.Walk(resolvedPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("Warning: skipping %s: %v\n", path, err)
			return nil // Skip files we can't read
		}
		if !info.IsDir() {
			ext := strings.ToLower(filepath.Ext(path))
			if extensions[ext] {
				audioFiles = append(audioFiles, path)
			}
		}
		return nil
	})

	if err != nil {
		fmt.Printf("Error reading directory: %v\n", err)
		return
	}

	if len(audioFiles) == 0 {
		fmt.Println("No audio files found in the folder.")
		return
	}

	fmt.Printf("Found %d audio files. Processing with %d workers...\n\n", len(audioFiles), numWorkers)

	// Create progress bar
	bar := progressbar.NewOptions(len(audioFiles),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowBytes(false),
		progressbar.OptionSetWidth(50),
		progressbar.OptionSetDescription("[cyan]Processing files...[reset]"),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionSetItsString("files"),
	)

	// Create worker pool
	jobs := make(chan fileJob, len(audioFiles))
	results := make(chan result, len(audioFiles))
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go worker(jobs, results, &wg, bar)
	}

	// Send jobs
	for i, file := range audioFiles {
		jobs <- fileJob{path: file, index: i}
	}
	close(jobs)

	// Close results channel when all workers are done
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	durations := make([]float64, len(audioFiles))
	errorCount := 0

	for res := range results {
		if res.err != nil {
			errorCount++
		} else {
			durations[res.index] = res.duration
		}
	}

	bar.Finish()
	fmt.Println()

	// Calculate totals
	var totalSeconds float64
	validFiles := 0
	for _, d := range durations {
		if d > 0 {
			totalSeconds += d
			validFiles++
		}
	}

	totalHours := totalSeconds / 3600.0
	meanHours := 0.0
	if validFiles > 0 {
		meanHours = (totalSeconds / float64(validFiles)) / 3600.0
	}

	fmt.Println("\n=== Results ===")
	fmt.Printf("Total files found: %d\n", len(audioFiles))
	fmt.Printf("Successfully processed: %d\n", validFiles)
	fmt.Printf("Errors: %d\n", errorCount)
	fmt.Printf("Total audio duration: %.2f hours\n", totalHours)
	fmt.Printf("Mean audio duration per file: %.4f hours (%.2f minutes)\n", meanHours, meanHours*60)
}
