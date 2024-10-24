package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"manga-scraper-api/lib"
	"manga-scraper-api/lib/middleware"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/joho/godotenv"
)

type Response struct {
	Status string      `json:"status"`
	Data   interface{} `json:"data"`
}

type ImageData struct {
	Src       string
	DataIndex int
}

// Function to handle the manga scraping and PDF generation
func getManga(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := chromedp.NewContext(context.Background())
    defer cancel()

    ctx, cancel = context.WithTimeout(ctx, 15*time.Second)
    defer cancel()

    title := r.URL.Query().Get("title")
    chapter := r.URL.Query().Get("chapter")
    mode := r.URL.Query().Get("mode") // Get the mode parameter

    if title == "" {
        http.Error(w, "Title query parameter is required", http.StatusBadRequest)
        return
    }

    var images []ImageData

    // Scraping the manga images from the website
    err := chromedp.Run(ctx,
        chromedp.Navigate(fmt.Sprintf("https://rawkuma.com/%s-chapter-%s/", title, chapter)),
        chromedp.Evaluate(`Array.from(document.querySelectorAll('img.ts-main-image')).map(img => ({ src: img.src, dataIndex: parseInt(img.getAttribute('data-index')) }))`, &images),
        chromedp.Text(`h1`, &title, chromedp.ByQuery),
    )

    if err != nil {
        http.Error(w, "Failed to scrape website", http.StatusInternalServerError)
        return
    }

    // Create a folder to store images
    folderName := fmt.Sprintf("images/%s", title)
    os.MkdirAll(folderName, os.ModePerm)

    // Save the images to the specified folder
    for _, image := range images {
        if err := lib.SaveImage(image.Src, folderName, image.DataIndex); err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
    }

    // Generate PDF after saving images
    if err := lib.GeneratePDF(folderName, title); err != nil {
        http.Error(w, "Failed to generate PDF", http.StatusInternalServerError)
        return
    }

    // If mode is 'open', open the PDF in the default PDF reader
	if mode == "open" {
		// Generate the PDF file path using the updated title
		newTitle := strings.ReplaceAll(title, " ", "_")
		pdfFilePath := fmt.Sprintf("manga-pdf/pdf/%s.pdf", newTitle)
	
		// Check if the PDF file exists before trying to open it
		if _, err := os.Stat(pdfFilePath); os.IsNotExist(err) {
			http.Error(w, "PDF file does not exist", http.StatusNotFound)
			return
		}
	
		// Attempt to open the PDF in the default GUI PDF viewer
		cmd := exec.Command("xdg-open", pdfFilePath)
		if err := cmd.Start(); err != nil {
			http.Error(w, "Failed to open PDF in GUI", http.StatusInternalServerError)
			return
		}
	
		// Respond to the client indicating success
		response := Response{
			Status: "success",
			Data:   []string{title},
		}
		json.NewEncoder(w).Encode(response)
		return
	}
	

    // Respond with the success message
    response := Response{
        Status: "success",
        Data:   []string{title},
    }
    json.NewEncoder(w).Encode(response)
}

// Load environment variables from .env file
func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}
}

func main() {
	rateLimiter := middleware.NewRateLimiter(3, 5*time.Second)

	// Admin endpoint for testing
	http.Handle("/admin-endpoint", middleware.AdminAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Welcome, Admin! 🌍"))
	})))

	// Hello endpoint with rate limiting
	http.Handle("/hello", rateLimiter.Limit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, World! 🌍"))
	})))

	// Manga endpoint with rate limiting
	http.Handle("/manga", rateLimiter.Limit(http.HandlerFunc(getManga)))

	fmt.Println("Server is running on port 8000... 🌟")
	log.Fatal(http.ListenAndServe(":8000", nil))
}
