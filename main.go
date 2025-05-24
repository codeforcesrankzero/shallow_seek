package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/shallowseek/cache"
	"github.com/shallowseek/config"
	"github.com/shallowseek/elasticsearch"
	"github.com/shallowseek/handlers"
)

func main() {
	if err := elasticsearch.Init(); err != nil {
		log.Fatalf("Failed to initialize Elasticsearch: %v", err)
	}

	if err := cache.Init(); err != nil {
		log.Printf("Warning: Failed to initialize cache: %v", err)
	}

	r := gin.Default()

	r.Static("/static", "./static")
	r.LoadHTMLGlob("templates/*")

	api := r.Group("/api")
	{
		api.GET("/search", gin.WrapF(handlers.SearchHandler))
		api.POST("/upload", handlers.UploadFileHandler)
		api.GET("/documents/:id/download", handlers.DownloadDocumentHandler)
		api.GET("/documents/:id/view", handlers.ViewDocumentHandler)
		api.GET("/status", gin.WrapF(handlers.StatusHandler))
	}

	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})

	port := config.GetPort()
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	serverErrors := make(chan error, 1)

	go func() {
		log.Printf("Starting ShallowSeek on port %s", port)
		serverErrors <- srv.ListenAndServe()
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		log.Printf("Error starting server: %v", err)

	case sig := <-shutdown:
		log.Printf("Shutdown signal received: %v", sig)

		handlers.BatchProcessor.Stop()

		if err := srv.Close(); err != nil {
			log.Printf("Error closing server: %v", err)
		}
	}

	log.Println("Shutdown complete")
}
