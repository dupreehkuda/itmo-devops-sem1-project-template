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
	"math/bits"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type prices struct {
	name      string
	category  string
	price     float64
	createdAt time.Time
}

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

	prices, err := processFiles(zipReader)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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

	itemCount, err := savePricesToStorage(tx, prices)
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

func processFiles(zipReader *zip.Reader) ([]prices, error) {
	var result []prices

	for _, zipFile := range zipReader.File {
		if strings.HasSuffix(zipFile.Name, ".csv") {
			zipFileOpened, err := zipFile.Open()
			if err != nil {
				return nil, fmt.Errorf("failed to unzip archive: %w", err)
			}
			defer zipFileOpened.Close()

			csvReader := csv.NewReader(zipFileOpened)
			csvReader.FieldsPerRecord = 5

			_, err = csvReader.Read()
			if err != nil {
				return nil, fmt.Errorf("failed to read first line: %w", err)
			}

			for {
				record, err := csvReader.Read()
				if err != nil {
					if errors.Is(err, io.EOF) {
						return nil, nil
					}

					return nil, fmt.Errorf("failed to read data from data.csv: %w", err)
				}

				// price to float
				formatedPrice, err := strconv.ParseFloat(record[3], bits.UintSize)
				if err != nil {
					return nil, fmt.Errorf("failed to format date: %w", err)
				}

				// createDate to time.Time
				formatedDate, err := time.Parse("2006-01-02", record[4])
				if err != nil {
					return nil, fmt.Errorf("failed to format date: %w", err)
				}

				result = append(result, prices{
					name:      record[1],
					category:  record[2],
					price:     formatedPrice,
					createdAt: formatedDate,
				})
			}
		} else {
			return nil, errors.New("csv file not found")
		}
	}

	return result, nil
}

func savePricesToStorage(tx *sql.Tx, prices []prices) (int, error) {
	itemCount := 0

	for _, price := range prices {
		_, err := tx.Exec(
			`INSERT INTO prices (name, category, price, create_date) VALUES ($1, $2, $3, $4)`,
			price.name,
			price.category,
			price.price,
			price.createdAt,
		)
		if err != nil {
			return itemCount, fmt.Errorf("failed to write data to database: %w", err)
		}

		itemCount++
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
        COALESCE(SUM(price), 0) AS total_price
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
