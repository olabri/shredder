package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"math"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultSocketPath = "/tmp/guitar_ear.sock"
	pitchToleranceC   = 25.0
	timingSlackMs     = 80
	confidenceMin     = 0.80
	scrollSpeedColsMs = 0.03
	hitLineCol        = 10
	laneCount         = 6
	frameDelay        = 33 * time.Millisecond
)

type Note struct {
	TimeMs     int64   `json:"time_ms"`
	String     int     `json:"string"`
	Fret       int     `json:"fret"`
	FreqHz     float64 `json:"freq_hz"`
	DurationMs int64   `json:"duration_ms"`
}

type Song struct {
	Title        string `json:"title"`
	BPM          float64 `json:"bpm"`
	SyncOffsetMs int64  `json:"sync_offset_ms"`
	Notes        []Note `json:"notes"`
}

type PitchMessage struct {
	Freq float64 `json:"freq"`
	Conf float64 `json:"conf"`
	TsMs int64   `json:"ts_ms"`
}

type NoteState struct {
	Hit           bool
	Miss          bool
	SustainGoodMs int64
}

func main() {
	songPath := flag.String("song", "", "path to song JSON")
	socketPath := flag.String("socket", defaultSocketPath, "unix socket path")
	flag.Parse()

	if *songPath == "" {
		log.Fatal("-song is required")
	}

	song, err := loadSong(*songPath)
	if err != nil {
		log.Fatalf("load song: %v", err)
	}

	pitchChan := make(chan PitchMessage, 64)
	if err := startSocketServer(*socketPath, pitchChan); err != nil {
		log.Fatalf("socket: %v", err)
	}

	states := make([]NoteState, len(song.Notes))
	start := time.Now()
	lastUpdate := time.Now()
	var lastPitch PitchMessage

	clearScreen()
	for {
		now := time.Now()
		deltaMs := now.Sub(lastUpdate).Milliseconds()
		if deltaMs < 0 {
			deltaMs = 0
		}
		lastUpdate = now

		for {
			select {
			case msg := <-pitchChan:
				lastPitch = msg
			default:
				goto done
			}
		}
	done:
		elapsedMs := now.Sub(start).Milliseconds() + song.SyncOffsetMs
		judgeNotes(elapsedMs, deltaMs, song.Notes, states, lastPitch)

		render(elapsedMs, song, states, lastPitch)
		time.Sleep(frameDelay)
	}
}

func loadSong(path string) (Song, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Song{}, err
	}

	var song Song
	if err := json.Unmarshal(data, &song); err != nil {
		return Song{}, err
	}

	if len(song.Notes) == 0 {
		return Song{}, errors.New("song has no notes")
	}

	return song, nil
}

func startSocketServer(path string, pitchChan chan<- PitchMessage) error {
	_ = os.Remove(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	listener, err := net.Listen("unix", path)
	if err != nil {
		return err
	}

	go func() {
		defer listener.Close()
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Printf("accept: %v", err)
				return
			}
			go handleConn(conn, pitchChan)
		}
	}()

	return nil
}

func handleConn(conn net.Conn, pitchChan chan<- PitchMessage) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		var msg PitchMessage
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			continue
		}
		select {
		case pitchChan <- msg:
		default:
			// drop
		}
	}
}

func judgeNotes(elapsedMs, deltaMs int64, notes []Note, states []NoteState, lastPitch PitchMessage) {
	for i, note := range notes {
		state := &states[i]
		if state.Hit || state.Miss {
			continue
		}

		if note.DurationMs > 0 {
			judgeSustain(elapsedMs, deltaMs, note, state, lastPitch)
			continue
		}

		if elapsedMs > note.TimeMs+timingSlackMs {
			state.Miss = true
			continue
		}

		if absInt64(elapsedMs-note.TimeMs) <= timingSlackMs {
			if pitchMatches(note, lastPitch) {
				state.Hit = true
			}
		}
	}
}

func judgeSustain(elapsedMs, deltaMs int64, note Note, state *NoteState, lastPitch PitchMessage) {
	start := note.TimeMs
	end := note.TimeMs + note.DurationMs

	if elapsedMs < start-timingSlackMs {
		return
	}

	if elapsedMs >= start && elapsedMs <= end {
		if pitchMatches(note, lastPitch) {
			state.SustainGoodMs += deltaMs
		}
		return
	}

	if elapsedMs > end+timingSlackMs {
		required := int64(float64(note.DurationMs) * 0.7)
		if state.SustainGoodMs >= required {
			state.Hit = true
		} else {
			state.Miss = true
		}
	}
}

func pitchMatches(note Note, lastPitch PitchMessage) bool {
	if lastPitch.Conf < confidenceMin {
		return false
	}
	if lastPitch.Freq <= 0 || note.FreqHz <= 0 {
		return false
	}

	cents := 1200.0 * math.Log2(lastPitch.Freq/note.FreqHz)
	return math.Abs(cents) <= pitchToleranceC
}

func render(elapsedMs int64, song Song, states []NoteState, lastPitch PitchMessage) {
	width := 80
	header := fmt.Sprintf("%s | time=%dms | pitch=%.2fHz conf=%.2f", song.Title, elapsedMs, lastPitch.Freq, lastPitch.Conf)
	if len(header) > width {
		header = header[:width]
	}

	lines := make([]string, 0, laneCount+4)
	lines = append(lines, pad(header, width))
	lines = append(lines, strings.Repeat("-", width))

	for lane := 1; lane <= laneCount; lane++ {
		row := make([]rune, width)
		for i := range row {
			row[i] = ' '
		}
		if hitLineCol >= 0 && hitLineCol < width {
			row[hitLineCol] = '|'
		}

		for i, note := range song.Notes {
			if note.String != lane {
				continue
			}
			col := hitLineCol + int(float64(note.TimeMs-elapsedMs)*scrollSpeedColsMs)
			if col < 0 || col >= width {
				continue
			}
			ch := 'o'
			if states[i].Hit {
				ch = 'x'
			} else if states[i].Miss {
				ch = '!'
			}
			row[col] = ch
		}

		lines = append(lines, string(row))
	}

	lines = append(lines, strings.Repeat("-", width))
	lines = append(lines, pad(fmt.Sprintf("accuracy: %.1f%%", accuracy(states)), width))

	moveHome()
	fmt.Println(strings.Join(lines, "\n"))
}

func accuracy(states []NoteState) float64 {
	hits := 0
	misses := 0
	for _, s := range states {
		if s.Hit {
			hits++
		} else if s.Miss {
			misses++
		}
	}
	if hits+misses == 0 {
		return 0
	}
	return float64(hits) / float64(hits+misses) * 100
}

func clearScreen() {
	fmt.Print("\033[2J")
	moveHome()
}

func moveHome() {
	fmt.Print("\033[H")
}

func pad(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

func absInt64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
