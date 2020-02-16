package cmd

import (
	"fmt"
	"github.com/angelorc/go-uploader/db"
	"github.com/angelorc/go-uploader/services"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/angelorc/go-uploader/server"
)

const (
	logLevelJSON = "json"
	logLevelText = "text"
	dbPath       = ".bitsongms"
	listenAddr   = "127.0.0.1:8081"
)

var (
	logLevel  string
	logFormat string
	wg        sync.WaitGroup
)

var rootCmd = &cobra.Command{
	Use:   "bitsongms",
	Short: "bitsongms implements a BitSong Media Server utility API.",
	RunE:  bitsongmsCmdHandler,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", zerolog.InfoLevel.String(), "logging level")
	rootCmd.PersistentFlags().StringVar(&logFormat, "log-format", logLevelJSON, "logging format; must be either json or text")
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func bitsongmsCmdHandler(cmd *cobra.Command, args []string) error {
	logLvl, err := zerolog.ParseLevel(logLevel)
	if err != nil {
		return err
	}

	zerolog.SetGlobalLevel(logLvl)

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		if err := os.Mkdir(dbPath, os.ModePerm); err != nil {
			return err
		}
	}

	// create and open key/value DB
	db, err := db.NewBadgerDB(dbPath, "db")
	if err != nil {
		return err
	}
	defer db.Close()

	// make a queue with a capacity of 1 transcoder.
	queue := make(chan *services.Audio, 1)

	go func() {
		for q := range queue {
			doTranscode(q)
		}
	}()

	// create HTTP router and mount routes
	router := mux.NewRouter()
	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
	})

	server.RegisterRoutes(db, router, queue)

	srv := &http.Server{
		Handler:      c.Handler(router),
		Addr:         listenAddr,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Info().Str("address", listenAddr).Msg("starting API server...")
	return srv.ListenAndServe()
}

func doTranscode(a *services.Audio) {
	a.Transcode()
}
