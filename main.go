package main

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/gdamore/tcell"
)

type State struct {
	runes            []rune
	cursorPosition   int
	windowWidth      int
	keysPressed      int
	wrongKeysPressed int
	timeStarted      int64
}

var (
	state = State{windowWidth: 0}
)

var text string
var text_runes []rune
var cursor_position int

const ADVANCE_SUCCESS = 1
const ADVANCE_FAILURE = 0

func readTextFromFile(file_path string) ([]rune, error) {
	// get text from file
	// this is not a streaming file implementation. Will fail for big files if it falls out of memory.
	data, error := os.ReadFile(file_path)
	if error != nil {
		panic(error)
	}

	// pick a random line
	var result_runes []rune
	raw_random := rand.Int()
	line_count := 0
	for _, rune := range data {
		if rune == '\n' {
			line_count += 1
		}
	}

	if line_count == 0 {
		return make([]rune, 0), errors.New("File is empty.")
	}

	raw_random = raw_random % line_count
	current_line := 0
	start_loc := -1

	for index, rune := range data {
		if start_loc == -1 {
			start_loc = index
		}
		if rune == '\n' && current_line == raw_random {
			intermediate_string := string(data[start_loc:index])
			intermediate_string = strings.TrimSpace(intermediate_string)
			for _, result_rune := range intermediate_string {
				result_runes = append(result_runes, result_rune)
			}
			return result_runes, nil
		} else if rune == '\n' {
			start_loc = -1
			current_line++
		}
	}

	return make([]rune, 0), errors.New("File is empty.")
}

/* Advances the cursor if the rune entered matches the rune at the cursor's position and returns ADVANCE_SUCCESS. Else, returns ADVANCE_FAILURE. */
func advanceCursor(char_typed rune) int {
	if state.cursorPosition < len(state.runes) {
		if char_typed == state.runes[state.cursorPosition] {
			state.cursorPosition += 1
			return ADVANCE_SUCCESS
		}
	}
	return ADVANCE_FAILURE
}

type Stats struct {
	accuracy float64
	wpm      float64
}

/* Returns a collection of statistics as a Stats struct, computed from the current state.  */
// would be a good idea to make this a pure function, but for now it's ok to keep it dependent to `state`.
func getStats() (result Stats) {
	result.accuracy = 0.0
	result.wpm = 0.0

	if state.keysPressed > 0 {

		result.accuracy = 100 * (1 - float64(state.wrongKeysPressed)/float64(state.keysPressed))

		time_diff := float64(time.Now().Unix() - state.timeStarted)
		if time_diff > 0 {
			result.wpm = 60 * (float64(state.keysPressed) / 5) / time_diff
		}
	}

	return
}

func (stats *Stats) writeToFile(pathname string) {
	os.WriteFile(
		pathname,
		[]byte(fmt.Sprintf("%s,%0.2fACC,%dWPM", string(state.runes), stats.accuracy, int(stats.wpm))),
		os.ModeAppend)

}

/* Renders the UI to the screen. */
func render(screen tcell.Screen) {
	limit := math.Min(float64(state.windowWidth), float64(len(state.runes)-state.cursorPosition))
	stats := getStats()

	accuracy_runes := []rune(fmt.Sprintf("Accuracy: %0.2f", stats.accuracy))
	wpm_runes := []rune(fmt.Sprintf("WPM: %d", int(stats.wpm)))

	sprint_completed_message := []rune("Sprint complete. Press any key to exit!")

	if state.cursorPosition < len(state.runes) {
		screen.SetContent(0, 0,
			state.runes[state.cursorPosition],
			state.runes[state.cursorPosition+1:state.cursorPosition+int(limit)],
			tcell.StyleDefault)
	} else {
		screen.SetContent(0, 0,
			sprint_completed_message[0],
			sprint_completed_message[1:],
			tcell.StyleDefault)
	}

	screen.SetContent(0, 2,
		accuracy_runes[0],
		accuracy_runes[1:],
		tcell.StyleDefault)

	screen.SetContent(0, 3,
		wpm_runes[0],
		wpm_runes[1:],
		tcell.StyleDefault)

	screen.Sync()
}

/* Fetches events from screen and passes it to eventChan. Must be run as a goroutine. */
func eventGenerator(eventChan chan tcell.Event, screen tcell.Screen) {
	for {
		ev := screen.PollEvent()
		eventChan <- ev
	}
}

func main() {
	// setup
	line, error := readTextFromFile("passages.txt")
	if error != nil {
		panic("file is empty.")
	} else {
		state.runes = line
	}

	state.cursorPosition = 0
	// end setup

	// initialize screen
	screen, err := tcell.NewScreen()
	if err != nil {
		panic("problems initializing tcell")
	}

	defer screen.Fini()

	err = screen.Init()
	if err != nil {
		panic("problems initializing tcell")
	}

	screen.Clear()

	state.timeStarted = time.Now().Unix()

	eventChan := make(chan tcell.Event)
	go eventGenerator(eventChan, screen)

	var entered_rune rune
	for state.cursorPosition < len(state.runes) {
		// get text
		select {
		case ev := <-eventChan:
			switch ev := ev.(type) {
			case *tcell.EventKey:
				if ev.Key() == tcell.KeyCtrlC {
					return
				} else {
					entered_rune = ev.Rune()
					if advanceCursor(entered_rune) == ADVANCE_SUCCESS {
						state.keysPressed++
					} else {
						state.wrongKeysPressed++
						state.keysPressed++
					}
				}
			case *tcell.EventResize:
				w, _ := ev.Size()
				state.windowWidth = w
			}
		default:
		}

		if state.windowWidth > 0 {
			render(screen)
		}
	}

	final_stats := getStats()
	final_stats.writeToFile("results.txt")

	// get a final keypress before quitting. A resize might trigger this too, so check the type of event.

	waitForKey := true
	for waitForKey {
		ev := <-eventChan
		switch ev.(type) {
		case *tcell.EventKey:
			waitForKey = false
			break
		}
	}
}
