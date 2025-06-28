package main

import (
	"database/sql"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type SentWorklist struct {
	ID               int
	NomorOrder       string
	Worklist         string
	TglMasukWorklist *time.Time
	TglKirimWorklist *time.Time
	TglTerimaHasil   *time.Time
	TglSimpanHasil   *time.Time
	HasilOrthanc     string
}

// Koneksi ke database middleware (bisa sama dengan Khanza, atau DB terpisah)
func ConnectMiddlewareDB(cfg Config) (*sql.DB, error) {
	dsn := cfg.DBUser + ":" + cfg.DBPassword + "@tcp(" + cfg.DBHost + ":" + cfg.DBPort + ")/" + cfg.DBName
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	log.Println("Successfully connected to Middleware DB")
	return db, nil
}

// Mengecek apakah worklist sudah pernah dikirim berdasarkan nomor_order
func IsWorklistSent(db *sql.DB, nomorOrder string) bool {
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM sent_worklist WHERE nomor_order=?)", nomorOrder).Scan(&exists)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("Error cek sent_worklist: %v", err)
		return false
	}
	return exists
}

// Mencatat worklist yang sudah dikirim
func InsertSentWorklist(db *sql.DB, nomorOrder, worklist string) {
	_, err := db.Exec(`INSERT INTO sent_worklist (nomor_order, worklist, tgl_masuk_worklist) VALUES (?, ?, NOW()) ON DUPLICATE KEY UPDATE tgl_kirim_worklist=NOW()`, nomorOrder, worklist)
	if err != nil {
		log.Printf("Error insert sent_worklist: %v", err)
	}
}

// Update hasil orthanc dan tanggal simpan hasil
func UpdateHasilOrthanc(db *sql.DB, nomorOrder, hasilOrthanc string) {
	_, err := db.Exec(`UPDATE sent_worklist SET hasil_orthanc=?, tgl_simpan_hasil=NOW() WHERE nomor_order=?`, hasilOrthanc, nomorOrder)
	if err != nil {
		log.Printf("Error update hasil_orthanc: %v", err)
	}
}
