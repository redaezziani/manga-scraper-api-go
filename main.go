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
	rateLimiter := middleware.NewRateLimiter(3, 5*time.Second)

	http.Handle("/admin-endpoint", middleware.AdminAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Welcome, Admin! 🌍"))
	})))

	http.Handle("/hello", rateLimiter.Limit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, World! 🌍"))
	})))

	http.Handle("/manga", rateLimiter.Limit(http.HandlerFunc(getManga)))
	fmt.Println("Server is running on port 8080... 🌟")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
