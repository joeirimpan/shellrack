package main

import (
	"bufio"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

const (
	backupTable    = "SHELLRACK"
	tableExistsSQL = `
	SELECT EXISTS (
		SELECT 1 FROM sqlite_master WHERE type='table' AND name='%s' LIMIT 1
	) AS table_exists`
	createTableSQL = `CREATE TABLE %s (history_line text, timestamp UNSIGNED BIG INT)`
	insertTableSQL = `REPLACE INTO %s ('history_line', 'timestamp') VALUES(?,?)`
	readTableSQL   = `SELECT history_line from %s ORDER BY timestamp DESC`
)

var homeLoc = os.Getenv("HOME")

type lineInfo struct {
	Line      string
	Timestamp string
}

func initDB(backupFile string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", backupFile)
	if err != nil {
		return nil, err
	}

	var tableExists bool
	err = db.QueryRow(fmt.Sprintf(tableExistsSQL, backupTable)).Scan(&tableExists)
	if err != nil {
		return nil, err
	}
	// Create table if it does not exist.
	if !tableExists {
		if _, err := db.Exec(fmt.Sprintf(createTableSQL, backupTable)); err != nil {
			return nil, err
		}
		log.Print("Table created successfully")
	}

	return db, nil
}

func backupDB(db *sql.DB, historyFile string) error {
	file, err := os.Open(historyFile)
	if err != nil {
		return err
	}
	defer file.Close()

	// Read line by line and insert into map.
	var (
		line    string
		reader  = bufio.NewReader(file)
		cmdHist = make(map[string]lineInfo, 0)
	)
	for {
		line, err = reader.ReadString('\n')
		if err != nil {
			break
		}

		arr := strings.Split(line, ";")
		if len(arr) > 1 {
			cmdHist[arr[1]] = lineInfo{
				Line:      line,
				Timestamp: strings.Split(strings.Split(arr[0], ": ")[1], ":")[0],
			}
		}
	}
	if err != io.EOF {
		return err
	}

	// Batch replace inserts.
	txn, err := db.Begin()
	if err != nil {
		return err
	}
	i := 0
	for _, info := range cmdHist {
		if _, err := txn.Exec(fmt.Sprintf(insertTableSQL, backupTable),
			info.Line, info.Timestamp); err != nil {
			return err
		}
		i++
	}
	if err := txn.Commit(); err != nil {
		return err
	}
	log.Printf("%d commands saved to db", i)

	return nil
}

func restoreHistory(db *sql.DB, historyFile string) error {
	rows, err := db.Query(fmt.Sprintf(readTableSQL, backupTable))
	if err != nil {
		return err
	}
	defer rows.Close()

	file, err := os.OpenFile(historyFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	i := 0
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return err
		}
		if _, err := file.WriteString(line + "\n"); err != nil {
			return err
		}
		i++
	}
	log.Printf("%d commands restore to %s", i, historyFile)

	return nil
}

func main() {
	var (
		backup, restore         bool
		historyFile, backupFile string
	)
	flag.BoolVar(&backup, "backup", false, "To backup shell history into sqlite db")
	flag.BoolVar(&restore, "restore", false, "To restore shell history into file")
	flag.StringVar(&historyFile, "history", homeLoc+"/.zsh_history", "Shell history file")
	flag.StringVar(&backupFile, "db", homeLoc+"/.zsh_history.sqlite", "Sqlite db name")
	flag.Parse()

	// Init db
	db, err := initDB(backupFile)
	if err != nil {
		log.Fatal(err)
	}

	// Backup
	if backup {
		log.Print("Backing up shell history.")
		if err := backupDB(db, historyFile); err != nil {
			log.Fatal(err)
		}
		return
	}

	// Restore
	if restore {
		log.Print("Restoring shell history.")
		if err := restoreHistory(db, historyFile); err != nil {
			log.Fatal(err)
		}
		return
	}
}
