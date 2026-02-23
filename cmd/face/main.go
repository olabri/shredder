package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"image/color"
	"log"
	"math"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	windowWidth       = 960
	windowHeight      = 540
	laneTop           = 120
	laneHeight        = 50
	hitLineX          = 160
	noteRadius        = 14
	scrollSpeedPxMs   = 0.30
	pitchToleranceC  = 25.0
	timingSlackMs     = 80
	confidenceMin     = 0.80
	defaultSocketPath = "/tmp/guitar_ear.sock"
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

type Game struct {
	song        Song
	noteStates  []NoteState
	pitchChan   chan PitchMessage
	lastPitch   PitchMessage
	startTime   time.Time
	lastUpdate  time.Time
	playing     bool
	mutex       sync.Mutex
	elapsedMs   int64
	socketPath  string
	spaceDown   bool
}

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("usage: %s <song.json>", os.Args[0])
	}

	song, err := loadSong(os.Args[1])
	if err != nil {
		log.Fatalf("load song: %v", err)
	}

	g := &Game{
		song:       song,
		noteStates: make([]NoteState, len(song.Notes)),
		pitchChan:  make(chan PitchMessage, 64),
		playing:    true,
		startTime:  time.Now(),
		lastUpdate: time.Now(),
		socketPath: defaultSocketPath,
	}

	if err := g.startSocketServer(); err != nil {
		log.Fatalf("socket server: %v", err)
	}

	ebiten.SetWindowSize(windowWidth, windowHeight)
	ebiten.SetWindowTitle("Shredder - Real-Time Guitar Trainer")
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
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

func (g *Game) startSocketServer() error {
	_ = os.Remove(g.socketPath)
	if err := os.MkdirAll(filepath.Dir(g.socketPath), 0o755); err != nil {
		return err
	}

	listener, err := net.Listen("unix", g.socketPath)
	if err != nil {
		return err
	}

	go func() {
		defer listener.Close()
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Printf("socket accept: %v", err)
				return
			}

			go g.handleConn(conn)
		}
	}()

	return nil
}

func (g *Game) handleConn(conn net.Conn) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		var msg PitchMessage
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			continue
		}

		select {
		case g.pitchChan <- msg:
		default:
			// Drop if the channel is full; newer samples are more valuable.
		}
	}
}

func (g *Game) Update() error {
	if ebiten.IsKeyPressed(ebiten.KeySpace) {
		if !g.spaceDown {
			g.togglePlay()
			g.spaceDown = true
		}
	} else {
		g.spaceDown = false
	}

	now := time.Now()
	deltaMs := now.Sub(g.lastUpdate).Milliseconds()
	if deltaMs < 0 {
		deltaMs = 0
	}
	g.lastUpdate = now

	for {
		select {
		case msg := <-g.pitchChan:
			g.lastPitch = msg
		default:
			goto done
		}
	}

done:
	if !g.playing {
		return nil
	}

	g.elapsedMs = now.Sub(g.startTime).Milliseconds() + g.song.SyncOffsetMs
	g.judgeNotes(deltaMs)

	return nil
}

func (g *Game) togglePlay() {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	if g.playing {
		g.playing = false
		return
	}

	g.playing = true
	g.startTime = time.Now().Add(-time.Duration(g.elapsedMs) * time.Millisecond)
	g.lastUpdate = time.Now()
}

func (g *Game) judgeNotes(deltaMs int64) {
	for i, note := range g.song.Notes {
		state := &g.noteStates[i]
		if state.Hit || state.Miss {
			continue
		}

		if note.DurationMs > 0 {
			g.judgeSustain(note, state, deltaMs)
			continue
		}

		if g.elapsedMs > note.TimeMs+timingSlackMs {
			state.Miss = true
			continue
		}

		if absInt64(g.elapsedMs-note.TimeMs) <= timingSlackMs {
			if g.pitchMatches(note) {
				state.Hit = true
			}
		}
	}
}

func (g *Game) judgeSustain(note Note, state *NoteState, deltaMs int64) {
	start := note.TimeMs
	end := note.TimeMs + note.DurationMs

	if g.elapsedMs < start-timingSlackMs {
		return
	}

	if g.elapsedMs >= start && g.elapsedMs <= end {
		if g.pitchMatches(note) {
			state.SustainGoodMs += deltaMs
		}
		return
	}

	if g.elapsedMs > end+timingSlackMs {
		required := int64(float64(note.DurationMs) * 0.7)
		if state.SustainGoodMs >= required {
			state.Hit = true
		} else {
			state.Miss = true
		}
	}
}

func (g *Game) pitchMatches(note Note) bool {
	if g.lastPitch.Conf < confidenceMin {
		return false
	}
	if g.lastPitch.Freq <= 0 || note.FreqHz <= 0 {
		return false
	}

	cents := 1200.0 * math.Log2(g.lastPitch.Freq/note.FreqHz)
	return math.Abs(cents) <= pitchToleranceC
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{18, 18, 20, 255})

	for i := 0; i < 6; i++ {
		y := float32(laneTop + i*laneHeight)
		vector.StrokeLine(screen, 0, y, windowWidth, y, 2, color.RGBA{80, 80, 90, 255}, true)
	}

	vector.StrokeLine(screen, hitLineX, 0, hitLineX, windowHeight, 3, color.RGBA{240, 200, 60, 255}, true)

	for i, note := range g.song.Notes {
		x := g.noteX(note)
		if x < -50 || x > float32(windowWidth+50) {
			continue
		}

		y := float32(laneTop + (note.String-1)*laneHeight)
		if note.String < 1 || note.String > 6 {
			continue
		}

		state := g.noteStates[i]
		col := color.RGBA{90, 160, 240, 255}
		if state.Hit {
			col = color.RGBA{80, 210, 120, 255}
		} else if state.Miss {
			col = color.RGBA{210, 80, 90, 255}
		}

		vector.DrawFilledCircle(screen, x, y, noteRadius, col, true)
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%d", note.Fret), int(x-4), int(y-6))
	}

	accuracy := g.accuracy()
	status := fmt.Sprintf("%s | Accuracy: %.1f%% | Pitch: %.2f Hz (%.2f)", g.song.Title, accuracy, g.lastPitch.Freq, g.lastPitch.Conf)
	if !g.playing {
		status = "Paused - press Space to resume"
	}
	seb := ebitenutil.DebugPrintAt
	seb(screen, status, 16, 16)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return windowWidth, windowHeight
}

func (g *Game) noteX(note Note) float32 {
	delta := float32(note.TimeMs - g.elapsedMs)
	return delta*scrollSpeedPxMs + hitLineX
}

func (g *Game) accuracy() float64 {
	hits := 0
	misses := 0
	for _, s := range g.noteStates {
		if s.Hit {
			hits++
		} else if s.Miss {
			misses++
		}
	}

	total := hits + misses
	if total == 0 {
		return 0
	}

	return float64(hits) / float64(total) * 100
}

func absInt64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
