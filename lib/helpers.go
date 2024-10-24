package lib

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jung-kurt/gofpdf"
)

func SaveImage(url string, folderName string, dataIndex int) error {
	filePath := fmt.Sprintf("%s/%d.jpg", folderName, dataIndex)
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create image file: %v", err)
	}
	defer file.Close()

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download image: %v", err)
	}
	defer resp.Body.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to save image: %v", err)
	}

	return nil
}

func GeneratePDF(folderName, title string) error {
    pdfFolder := "manga-pdf/pdf"
    os.MkdirAll(pdfFolder, os.ModePerm)

    pdf := gofpdf.New("P", "mm", "A4", "")
    imageFiles, err := os.ReadDir(folderName)
    if err != nil {
        return err
    }

    var orderedFiles []string
    for _, file := range imageFiles {
        if !file.IsDir() {
            orderedFiles = append(orderedFiles, file.Name())
        }
    }

    sort.Slice(orderedFiles, func(i, j int) bool {
        iIndex, _ := strconv.Atoi(orderedFiles[i][:len(orderedFiles[i])-4])
        jIndex, _ := strconv.Atoi(orderedFiles[j][:len(orderedFiles[j])-4])
        return iIndex < jIndex
    })

    for _, filename := range orderedFiles {
        imgPath := filepath.Join(folderName, filename)
        pdf.AddPage()
        pdf.Image(imgPath, 10, 10, 190, 0, false, "", 0, "")
    }

    newTitle := strings.ReplaceAll(title, " ", "_")
	fmt.Println(newTitle)
    pdfFilePath := filepath.Join(pdfFolder, newTitle + ".pdf") 

    if err := pdf.OutputFileAndClose(pdfFilePath); err != nil {
        return err
    }

    return os.RemoveAll(folderName)
}