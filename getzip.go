package main

import (
	"archive/zip"
	"encoding/csv"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
)

func getZipRequest(w http.ResponseWriter, r *http.Request) {
	fileName := "data.csv"

	// 1. Create data.csv file.
	csvFile, err := os.Create(fileName)
	if err != nil {
		http.Error(w, "Cant create data.csv file", http.StatusInternalServerError)
		return
	}
	defer csvFile.Close()

	// 2. Write data from db.
	if err := writeDataToCSV(csvFile); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 3. Create data.zip file.
	zippedFile, err := zipFile(fileName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Send data.zip to user
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="data.zip"`)
	http.ServeFile(w, r, zippedFile.Name())
}

func writeDataToCSV(csvFile *os.File) error {
	// 1. Create file writer
	writer := csv.NewWriter(csvFile)

	// 2. Add CSV header to file.
	headers := []string{"id", "name", "category", "price", "create_date"}
	if err := writer.Write(headers); err != nil {
		return errors.New("failed to add csv header")
	}

	// 3. Get all data from the database.
	rows, err := postgresDb.Query("SELECT id, name, category, price, TO_CHAR(create_date, 'YYYY-MM-DD') FROM prices")
	if err != nil {
		return errors.New("failed to read data from db")
	}
	defer rows.Close()

	// 4. Write data to data.csv.
	for rows.Next() {
		var (
			id         int
			name       string
			category   string
			price      float64
			createDate string
		)

		if err := rows.Scan(&id, &name, &category, &price, &createDate); err != nil {
			return errors.New("failed to scan row")
		}

		formatedPrice := strconv.FormatFloat(price, 'f', 2, 64)

		record := []string{strconv.Itoa(id), name, category, formatedPrice, createDate}
		if err := writer.Write(record); err != nil {
			return errors.New("failed to write data to csv")
		}
	}

	if err := rows.Err(); err != nil {
		return errors.New("error reading from db")
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return errors.New("error flushing writer")
	}

	return nil
}

func zipFile(fileName string) (*os.File, error) {
	zippedFileName := "data.zip"

	// 1. Create data.zip file.
	zippedFile, err := os.Create(zippedFileName)
	if err != nil {
		return nil, errors.New("failed to create data.zip")
	}
	defer zippedFile.Close()

	zipWriter := zip.NewWriter(zippedFile)

	// Add data.csv to zip archive
	zipFileWriter, err := zipWriter.Create(fileName)
	if err != nil {
		return nil, errors.New("failed to archive data.csv")
	}

	// Open data.csv for reading (with data)
	csvFileRead, err := os.Open(fileName)
	if err != nil {
		return nil, errors.New("failed to open data.csv")
	}
	defer csvFileRead.Close()

	// Copy data.csv (with data) to data.csv inside data.zip
	if _, err := io.Copy(zipFileWriter, csvFileRead); err != nil {
		return nil, errors.New("failed to copy data")
	}

	zipWriter.Close()

	return zippedFile, nil
}
