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
	"runtime"
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

func getManga(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	title := r.URL.Query().Get("title")
	chapter := r.URL.Query().Get("chapter")
	mode := r.URL.Query().Get("mode")

	if title == "" {
		http.Error(w, "Title query parameter is required", http.StatusBadRequest)
		return
	}

	var images []ImageData

	err := chromedp.Run(ctx,
		chromedp.Navigate(fmt.Sprintf("https://rawkuma.com/%s-chapter-%s/", title, chapter)),
		chromedp.Evaluate(`Array.from(document.querySelectorAll('img.ts-main-image')).map(img => ({ src: img.src, dataIndex: parseInt(img.getAttribute('data-index')) }))`, &images),
		chromedp.Text(`h1`, &title, chromedp.ByQuery),
	)

	if err != nil {
		http.Error(w, "Failed to scrape website", http.StatusInternalServerError)
		return
	}

	folderName := fmt.Sprintf("images/%s", title)
	os.MkdirAll(folderName, os.ModePerm)

	for _, image := range images {
		if err := lib.SaveImage(image.Src, folderName, image.DataIndex); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if err := lib.GeneratePDF(folderName, title); err != nil {
		http.Error(w, "Failed to generate PDF", http.StatusInternalServerError)
		return
	}

	if mode == "open" {
		newTitle := strings.ReplaceAll(title, " ", "_")
		pdfFilePath := fmt.Sprintf("manga-pdf/pdf/%s.pdf", newTitle)

		if _, err := os.Stat(pdfFilePath); os.IsNotExist(err) {
			http.Error(w, "PDF file does not exist", http.StatusNotFound)
			return
		}

		// Check the OS
		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			cmd = exec.Command("start", pdfFilePath) 
		} else if runtime.GOOS == "linux" {
			cmd = exec.Command("xdg-open", pdfFilePath) 
		} else if runtime.GOOS == "darwin" {
			cmd = exec.Command("open", pdfFilePath) 
		} else {
			http.Error(w, "Unsupported operating system", http.StatusInternalServerError)
			return
		}

		if err := cmd.Start(); err != nil {
			http.Error(w, "Failed to open PDF in GUI", http.StatusInternalServerError)
			return
		}

		response := Response{
			Status: "success",
			Data:   []string{title},
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	response := Response{
		Status: "success",
		Data:   []string{title},
	}
	json.NewEncoder(w).Encode(response)
}

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}
}

func main() {
	rateLimiter := middleware.NewRateLimiter(1, 5*time.Second) // 1 request per 5 seconds rate limit
	http.Handle("/manga", rateLimiter.Limit(http.HandlerFunc(getManga)))

	fmt.Println("Server is running on port 8000")
	log.Fatal(http.ListenAndServe(":8000", nil))
}
