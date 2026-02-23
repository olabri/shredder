package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
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
	"github.com/hajimehoshi/ebiten/v2/inpututil"
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
	pitchToleranceC   = 25.0
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
	Title        string  `json:"title"`
	BPM          float64 `json:"bpm"`
	SyncOffsetMs int64   `json:"sync_offset_ms"`
	StringCount  int     `json:"string_count"`
	Notes        []Note  `json:"notes"`
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
	lastUpdate  time.Time
	playing     bool
	mutex       sync.Mutex
	elapsed     float64
	elapsedMs   int64
	socketPath  string
	stringCount int
	speed       float64
}

func main() {
	songPath := flag.String("song", "", "path to song JSON")
	stringsFlag := flag.Int("strings", 0, "number of strings (4 or 6)")
	flag.Parse()

	path := *songPath
	if path == "" && flag.NArg() > 0 {
		path = flag.Arg(0)
	}
	if path == "" {
		log.Fatalf("usage: %s -song <song.json> [-strings N]", os.Args[0])
	}

	song, err := loadSong(path)
	if err != nil {
		log.Fatalf("load song: %v", err)
	}

	stringCount := song.StringCount
	if stringCount == 0 {
		stringCount = 6
	}
	if *stringsFlag > 0 {
		stringCount = *stringsFlag
	}

	g := &Game{
		song:        song,
		noteStates:  make([]NoteState, len(song.Notes)),
		pitchChan:   make(chan PitchMessage, 64),
		playing:     true,
		lastUpdate:  time.Now(),
		socketPath:  defaultSocketPath,
		stringCount: stringCount,
		speed:       1.0,
	}
	g.elapsed = float64(song.SyncOffsetMs)
	g.elapsedMs = song.SyncOffsetMs

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
	if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		g.togglePlay()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyS) {
		g.stop()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEqual) || inpututil.IsKeyJustPressed(ebiten.KeyKPAdd) {
		g.adjustSpeed(0.1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyMinus) || inpututil.IsKeyJustPressed(ebiten.KeyKPSubtract) {
		g.adjustSpeed(-0.1)
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
	if g.playing {
		g.elapsed += float64(deltaMs) * g.speed
		g.elapsedMs = int64(g.elapsed) + g.song.SyncOffsetMs
		g.judgeNotes(int64(float64(deltaMs) * g.speed))
	}

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
	g.lastUpdate = time.Now()
}

func (g *Game) stop() {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	g.playing = false
	g.elapsed = 0
	g.elapsedMs = g.song.SyncOffsetMs
	g.lastUpdate = time.Now()
	for i := range g.noteStates {
		g.noteStates[i] = NoteState{}
	}
}

func (g *Game) adjustSpeed(delta float64) {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	g.speed += delta
	if g.speed < 0.5 {
		g.speed = 0.5
	}
	if g.speed > 2.0 {
		g.speed = 2.0
	}
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

	for i := 0; i < g.stringCount; i++ {
		y := float32(laneTop + i*laneHeight)
		vector.StrokeLine(screen, 0, y, windowWidth, y, 2, color.RGBA{80, 80, 90, 255}, true)
		label := stringName(g.stringCount, i+1)
		if label != "" {
			ebitenutil.DebugPrintAt(screen, label, 8, int(y-10))
		}
	}

	vector.StrokeLine(screen, hitLineX, 0, hitLineX, windowHeight, 3, color.RGBA{240, 200, 60, 255}, true)

	for i, note := range g.song.Notes {
		x := g.noteX(note)
		if x < -50 || x > float32(windowWidth+50) {
			continue
		}

		y := float32(laneTop + (note.String-1)*laneHeight)
		if note.String < 1 || note.String > g.stringCount {
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
	status := fmt.Sprintf("%s | Accuracy: %.1f%% | Speed: %.1fx | Pitch: %.2f Hz (%.2f)", g.song.Title, accuracy, g.speed, g.lastPitch.Freq, g.lastPitch.Conf)
	if !g.playing {
		status = "Paused - Space: resume | S: stop | +/-: speed"
	}
	seb := ebitenutil.DebugPrintAt
	seb(screen, status, 16, 16)
}

func stringName(stringCount, lane int) string {
	if lane < 1 || lane > stringCount {
		return ""
	}
	switch stringCount {
	case 6:
		names := []string{"E4", "B3", "G3", "D3", "A2", "E2"}
		return names[lane-1]
	case 4:
		names := []string{"G2", "D2", "A1", "E1"}
		return names[lane-1]
	default:
		return ""
	}
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
