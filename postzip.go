package main

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func postZipRequest(w http.ResponseWriter, r *http.Request) {
	zipFile, fileHeader, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Cant read file", http.StatusBadRequest)
		return
	}
	defer zipFile.Close()

	archive, err := zip.OpenReader(fileHeader.Filename)
	if err != nil {
		panic(err)
	}
	defer archive.Close()

	// Read the file content into a byte buffer
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, zipFile); err != nil {
		http.Error(w, "Error reading file content", http.StatusInternalServerError)
		return
	}

	zipReader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		http.Error(w, "Unable to read zip content", http.StatusInternalServerError)
		return
	}

	tx, err := postgresDb.Begin()
	if err != nil {
		http.Error(w, "Cant create transaction", http.StatusInternalServerError)
		return
	}
	defer func() {
		if err != nil {
			tx.Rollback()
			return
		}
		err = tx.Commit()
	}()

	itemCount, err := processFiles(tx, zipReader)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp, err := getTotalInfo(tx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp.TotalItems = itemCount

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	return
}

func processFiles(tx *sql.Tx, zipReader *zip.Reader) (int, error) {
	itemCount := 0

	for _, zipFile := range zipReader.File {
		if strings.HasSuffix(zipFile.Name, ".csv") {
			zipFileOpened, err := zipFile.Open()
			if err != nil {
				return 0, fmt.Errorf("failed to unzip archive: %w", err)
			}
			defer zipFileOpened.Close()

			csvReader := csv.NewReader(zipFileOpened)
			csvReader.FieldsPerRecord = 5

			_, err = csvReader.Read()
			if err != nil {
				return itemCount, fmt.Errorf("failed to read first line: %w", err)
			}

			for {
				record, err := csvReader.Read()
				if err != nil {
					if errors.Is(err, io.EOF) {
						return itemCount, nil
					}

					return itemCount, fmt.Errorf("failed to read data from data.csv: %w", err)
				}

				// createDate to time.Date
				formatedDate, err := time.Parse("2006-01-02", record[4])
				if err != nil {
					return itemCount, fmt.Errorf("failed to format date: %w", err)
				}

				_, err = tx.Exec(`INSERT INTO prices (name, category, price, create_date) VALUES ($1, $2, $3, $4)`,
					record[1], record[2], record[3], formatedDate)
				if err != nil {
					return itemCount, fmt.Errorf("failed to write data to database: %w", err)

				}

				itemCount++
			}
		} else {
			return itemCount, errors.New("csv file not found")
		}
	}

	return itemCount, nil
}

func getTotalInfo(tx *sql.Tx) (response, error) {
	var (
		totalCategories int
		totalPrice      float64
	)

	rowSelect := tx.QueryRow(`
    SELECT 
        COUNT(DISTINCT category) AS total_categories,
        COALESCE(SUM(CAST(price AS numeric)), 0) AS total_price
    FROM prices;
	`)

	if err := rowSelect.Scan(&totalCategories, &totalPrice); err != nil {
		return response{}, fmt.Errorf("failed to scan row: %w", err)
	}

	return response{
		TotalCategories: totalCategories,
		TotalPrice:      totalPrice,
	}, nil
}
