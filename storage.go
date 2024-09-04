package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// Stores State and Statistics structs in an SQLite database for future use

type State struct {
	runes            []rune
	cursorPosition   int
	windowWidth      int
	keysPressed      int
	wrongKeysPressed int
	timeStarted      int64
	passageSource    string
	timeTaken        int64
}

type Statistics struct {
	Accuracy float64
	Wpm      float64
}

var envMap map[string]string
var envWasInitialized bool = false

func getEnv() {
	if envWasInitialized {
		return
	}
	env := os.Environ()
	envMap = make(map[string]string)
	for _, envVar := range env {
		keyAndVal := strings.Split(envVar, "=")
		envMap[keyAndVal[0]] = keyAndVal[1]
	}
	envWasInitialized = true
}

type DBInputFormat struct {
	Wpm       float64
	Accuracy  float64
	Passage   string
	TimeTaken int64
}

type DBOutputFormat struct {
	Timestamp string
	Wpm       float64
	Accuracy  float64
	Passage   string
	TimeTaken int64
}

func writeToDB(stats Statistics, state State) error {
	// must access ~/.termracer/db.sqlite3
	// create folder .termracer if it does not exist
	getEnv()
	if homeDir, ok := envMap["HOME"]; ok {
		os.Mkdir(fmt.Sprintf("%s/.termracer", homeDir), 0644)
		db, err := sql.Open("sqlite3", fmt.Sprintf("%s/.termracer/db.sqlite3", homeDir))
		if err != nil {
			return err
		}
		defer db.Close()

		_, err = db.Exec("CREATE TABLE IF NOT EXISTS sprints (id INTEGER PRIMARY KEY AUTOINCREMENT, creation_time DATETIME DEFAULT CURRENT_TIMESTAMP, stats JSONB NOT NULL)")
		if err != nil {
			return err
		}

		stmt, err := db.Prepare("INSERT INTO sprints (stats) VALUES (?)")
		if err != nil {
			return err
		}
		defer stmt.Close()

		record := DBInputFormat{
			Wpm:       stats.Wpm,
			Accuracy:  stats.Accuracy,
			Passage:   string(state.runes),
			TimeTaken: state.timeTaken,
		}

		record_marshalled, err := json.Marshal(record)
		if err != nil {
			return err
		}

		if _, err := stmt.Exec(record_marshalled); err != nil {
			return err
		}
	}

	return nil
}

type HistoryFormat struct {
	rows    []DBOutputFormat
	average float64
}

func readFromDB() (history HistoryFormat, err error) {
	getEnv()
	if homeDir, ok := envMap["HOME"]; ok {
		db, err := sql.Open("sqlite3", fmt.Sprintf("%s/.termracer/db.sqlite3", homeDir))
		if err != nil {
			return HistoryFormat{}, err
		}
		defer db.Close()

		// getting the stats of all sprints
		rows, err := db.Query("SELECT creation_time, stats FROM sprints ORDER BY creation_time DESC")
		if err != nil {
			return HistoryFormat{}, err
		}
		defer rows.Close()

		for rows.Next() {
			var d DBOutputFormat
			var json_data string
			rows.Scan(&d.Timestamp, &json_data)
			json.Unmarshal([]byte(json_data), &d)
			history.rows = append(history.rows, d)
		}

		// getting the average WPM across the history of all sprints
		rows, err = db.Query("select AVG(json_extract(stats, '$.Wpm')) from sprints;")
		for rows.Next() {
			rows.Scan(&history.average)
		}
	}

	return
}
