# howManyHours

A fast, concurrent CLI tool for calculating the total duration of audio files in a directory.

## Features

- **Multi-format support**: MP3, WAV, OGG, FLAC, and M4A files
- **Concurrent processing**: Utilizes all CPU cores for fast analysis
- **Progress tracking**: Real-time progress bar with file count
- **Recursive scanning**: Automatically scans subdirectories
- **Detailed statistics**: Total duration, file count, and average duration per file

## Installation

### Prerequisites

- Go 1.24.3 or higher

### Build from source

```bash
git clone <repository-url>
cd howManyHours
go build -o howManyHours
```

## Usage

```bash
./howManyHours <folder_path>
```

### Example

```bash
./howManyHours ~/Music/Podcasts
```

### Output

```
Scanning directory: ~/Music/Podcasts
Found 42 audio files. Processing with 8 workers...

Processing files... [===========================>] 42/42 files

=== Results ===
Total files found: 42
Successfully processed: 42
Errors: 0
Total audio duration: 15.67 hours
Mean audio duration per file: 0.3731 hours (22.39 minutes)
```

## Supported Formats

- **MP3** (.mp3) - Full support
- **WAV** (.wav) - Full support
- **OGG** (.ogg) - Detected but not yet implemented
- **FLAC** (.flac) - Detected but not yet implemented
- **M4A** (.m4a) - Detected but not yet implemented

## How It Works

The tool uses a worker pool pattern to process multiple audio files concurrently:

1. Recursively scans the specified directory for audio files
2. Distributes files across worker goroutines (one per CPU core)
3. Calculates duration for each file based on its format
4. Aggregates results and displays statistics

## Dependencies

- [go-audio/wav](https://github.com/go-audio/wav) - WAV file decoding
- [tcolgate/mp3](https://github.com/tcolgate/mp3) - MP3 file decoding
- [schollz/progressbar](https://github.com/schollz/progressbar) - Terminal progress bar

## License

MIT
