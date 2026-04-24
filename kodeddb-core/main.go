package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/Koded0214h/kodeddb-core/internal/api"
)

func main() {
	var (
		addr     = flag.String("addr", "0.0.0.0:6380", "Address to listen on")
		dataDir  = flag.String("data", "./data", "Directory for database files")
		dataFile = flag.String("db", "", "Database file path (overrides --data)")
		walFile  = flag.String("wal", "", "WAL file path (overrides --data)")
	)
	flag.Parse()

	// Resolve file paths
	dbPath  := *dataFile
	walPath := *walFile
	if dbPath == "" {
		os.MkdirAll(*dataDir, 0755)
		dbPath  = filepath.Join(*dataDir, "koded.db")
		walPath = filepath.Join(*dataDir, "koded.wal")
	}

	fmt.Printf("╔═══════════════════════════════════════╗\n")
	fmt.Printf("║         KodedDB Server v0.1.0         ║\n")
	fmt.Printf("╚═══════════════════════════════════════╝\n")
	fmt.Printf("  data:  %s\n", dbPath)
	fmt.Printf("  wal:   %s\n", walPath)
	fmt.Printf("  addr:  %s\n\n", *addr)

	srv, err := api.NewServer(api.Options{
		DataFile: dbPath,
		WALFile:  walPath,
		Addr:     *addr,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Graceful shutdown on SIGINT / SIGTERM
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\n[kodeddb] shutting down...")
		srv.Close()
	}()

	if err := srv.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}