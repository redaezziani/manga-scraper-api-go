package main

import (
	"context"
	"database/sql" 
	"encoding/json"
	"fmt"
	"log"
	"manga-scraper-api/lib"
	"manga-scraper-api/lib/middleware"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"

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

var db *sql.DB 

func initDB() *sql.DB {
	db, err := sql.Open("sqlite3", "manga-scraper.db")
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	// Create rate_limits table
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS rate_limits (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		client_ip TEXT NOT NULL,
		request_count INTEGER NOT NULL,
		chapter_number INTEGER NOT NULL,
		manga_title TEXT NOT NULL,
		last_request_timestamp DATETIME NOT NULL,
		timestamp DATETIME NOT NULL
	);`)
	if err != nil {
		log.Fatalf("Failed to create rate_limits table: %v", err)
	}

	// Create manga_images table
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS manga_images (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		chapter TEXT NOT NULL,
		image_path TEXT NOT NULL
	);`)
	if err != nil {
		log.Fatalf("Failed to create manga_images table: %v", err)
	}

	return db
}


type MangaResponse struct {
    Exists     bool   `json:"exists"`
    MangaTitle string `json:"manga_title,omitempty"`
}

func isMangaAlreadyExist(title, chapter string) (bool, string, error) {
	var count int
	var mangaTitle string

	// Query the database
	row := db.QueryRow(`SELECT COUNT(*), manga_title FROM rate_limits WHERE manga_title = ? AND chapter_number = ?`, title, chapter)
	err := row.Scan(&count, &mangaTitle) 
	if err != nil {
		return false, "", err 
	}

	return count > 0, mangaTitle, nil 
}

func getManga(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	title := r.URL.Query().Get("title")
	chapter := r.URL.Query().Get("chapter")
	mode := r.URL.Query().Get("mode")
	clientIP := r.RemoteAddr 

	if title == "" {
		http.Error(w, "Title query parameter is required", http.StatusBadRequest)
		return
	}

    isMangaDownloaded, existingTitle,fail := isMangaAlreadyExist(title, chapter)
    if fail != nil {
        http.Error(w, "Failed to check if manga is already downloaded", http.StatusInternalServerError)
        return
    }
    if isMangaDownloaded {
        stm := `UPDATE rate_limits SET request_count = request_count + 1 WHERE manga_title = ?`
		_, err := db.Exec(stm, title)
		if err != nil {
			http.Error(w, "Failed to update request count", http.StatusInternalServerError)
			return
		}

		if mode == "download" {
            
			http.Redirect(w, r, fmt.Sprintf("/d?manga=%s", existingTitle), http.StatusSeeOther)
			return
		}
	} else {
        stm := `INSERT INTO rate_limits (client_ip, request_count, chapter_number, manga_title, last_request_timestamp, timestamp) VALUES (?, ?, 
              ?, ?, ?, ?)`
        new_title := strings.ReplaceAll(title, " ", "_")
		_, err := db.Exec(stm,
			clientIP, 1, chapter, new_title, time.Now(), time.Now())
		if err != nil {
			http.Error(w, "Failed to insert rate limit data", http.StatusInternalServerError)
			return
		}
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
        stm := `INSERT INTO manga_images (title, chapter, image_path) VALUES (?, ?, ?)`
        new_chapter, _ := strconv.Atoi(chapter)
		_, err := db.Exec(stm, title, new_chapter, fmt.Sprintf("%s/%d.jpg", folderName, image.DataIndex))
		if err != nil {
			http.Error(w, "Failed to store image in database", http.StatusInternalServerError)
			return
		}
	}
    stm := `INSERT INTO rate_limits (client_ip, request_count, chapter_number, manga_title, last_request_timestamp, 
           timestamp) VALUES (?, ?, ?, ?, ?, ?)`

    new_manga_title := strings.ReplaceAll(title, " ", "_")
	_, err = db.Exec(stm, clientIP, 1, chapter, new_manga_title, time.Now(), time.Now())
	if err != nil {
		http.Error(w, "Failed to insert rate limit data", http.StatusInternalServerError)
		return
	}

	if err := lib.GeneratePDF(folderName, new_manga_title); err != nil {
		http.Error(w, "Failed to generate PDF", http.StatusInternalServerError)
		return
	}

	if mode == "download" {
		http.Redirect(w, r, fmt.Sprintf("/d?manga=%s", title), http.StatusSeeOther)
		return
	}

	response := Response{
		Status: "success",
		Data:   []string{title},
	}
	json.NewEncoder(w).Encode(response)
}

func downloadPDF(w http.ResponseWriter, r *http.Request) {
	title := r.URL.Query().Get("manga")
	if title == "" {
		http.Error(w, "Manga title query parameter is required", http.StatusBadRequest)
		return
	}

	newTitle := strings.ReplaceAll(title, " ", "_")
	pdfFilePath := fmt.Sprintf("manga-pdf/pdf/%s.pdf", newTitle)

	if _, err := os.Stat(pdfFilePath); os.IsNotExist(err) {
		http.Error(w, "PDF file does not exist", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.pdf", newTitle))
	w.Header().Set("Content-Type", "application/pdf")

	// Serve the PDF file
	http.ServeFile(w, r, pdfFilePath)
}

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}
}

func main() {
	rateLimiter := middleware.NewRateLimiter(1, 5*time.Second) 
	http.Handle("/manga", rateLimiter.Limit(http.HandlerFunc(getManga)))
	http.HandleFunc("/d", downloadPDF) 

	db = initDB() 
	defer db.Close() 

	fmt.Println("Server is running on port 8000")
	log.Fatal(http.ListenAndServe(":8000", nil))
}
