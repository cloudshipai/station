package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"path/filepath"
	
	"station/pkg/debug"
	
	_ "github.com/mattn/go-sqlite3"
)

// Simple utility to fix tools_used field by parsing existing debug logs
func main() {
	// Find the database
	configDir := os.ExpandEnv("$HOME/.config/station")
	dbPath := filepath.Join(configDir, "station.db")
	
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()
	
	// Get all runs with debug logs but no tools_used
	rows, err := db.Query(`
		SELECT id, debug_logs 
		FROM agent_runs 
		WHERE debug_logs IS NOT NULL 
		AND (tools_used IS NULL OR tools_used = 0)
		ORDER BY id DESC
		LIMIT 10
	`)
	if err != nil {
		log.Fatalf("Failed to query runs: %v", err)
	}
	defer rows.Close()
	
	updated := 0
	for rows.Next() {
		var id int64
		var debugLogs string
		
		if err := rows.Scan(&id, &debugLogs); err != nil {
			log.Printf("Failed to scan row: %v", err)
			continue
		}
		
		// Parse debug logs to extract tool usage
		toolCount, toolNames := debug.ExtractToolUsageFromDebugLogs(debugLogs)
		
		if toolCount > 0 {
			// Update the database
			_, err := db.ExecContext(context.Background(),
				"UPDATE agent_runs SET tools_used = ? WHERE id = ?",
				toolCount, id)
			
			if err != nil {
				log.Printf("Failed to update run %d: %v", id, err)
			} else {
				log.Printf("✅ Updated run %d: %d tools used (%v)", id, toolCount, toolNames)
				updated++
			}
		}
	}
	
	if updated > 0 {
		log.Printf("🎉 Successfully updated %d runs with tool usage data", updated)
	} else {
		log.Printf("ℹ️ No runs needed updating")
	}
}