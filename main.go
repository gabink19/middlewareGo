package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/joho/godotenv"
)

func processWorklist(cfg Config, db, mwdb *sql.DB) {
	for {
		worklists, err := GetPendingWorklist(db, time.Now().Format("2006-01-02"))
		if err != nil {
			log.Printf("Gagal ambil worklist: %v", err)
			SavePortalLog(mwdb, "[Worklist] Gagal ambil worklist: "+err.Error())
			UpdateWorklists(nil)
			time.Sleep(10 * time.Second)
			continue
		}
		// var wlPortal []Worklist
		for _, wl := range worklists {
			if IsWorklistSent(mwdb, wl.AccessionNumber) {
				continue
			}
			SavePortalLog(mwdb, "[Worklist] Proses kirim worklist "+wl.AccessionNumber)
			marsh, _ := json.Marshal(wl)
			err = SendWorklistToOrthanc(cfg, wl)
			if err != nil {
				log.Printf("Gagal kirim worklist ke Orthanc untuk %s: %v", wl.AccessionNumber, err)
				SavePortalLog(mwdb, "[Worklist] Gagal kirim worklist ke Orthanc untuk "+wl.AccessionNumber+": "+err.Error())
				continue
			}
			log.Printf("Worklist %s dikirim ke Orthanc", wl.AccessionNumber)
			SavePortalLog(mwdb, "[Worklist] Worklist "+wl.AccessionNumber+" dikirim ke Orthanc")
			InsertSentWorklist(mwdb, wl.AccessionNumber, string(marsh))
		}
		time.Sleep(30 * time.Second)
	}
}

func processSRWebhook(cfg Config, db, mwdb *sql.DB, bodyBytes []byte) {
	var payload struct {
		Accession        string      `json:"accession"`
		Link             string      `json:"link"`
		PatientIDINT     interface{} `json:"patient_id"`
		PatientID        string      `json:""`
		PatientName      string      `json:"patient_name"`
		StudyInstanceUID string      `json:"study"`
		OrthancUUID      string      `json:"orthanc_uuid"`
		DicomInstanceUID string      `json:"dicom_instance_uid"`
	}

	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		log.Printf("Invalid JSON payload : %v", err)
		SavePortalLog(mwdb, "[SR] Webhook gagal: payload tidak valid")
		return
	}
	SavePortalLog(mwdb, "[SR] Webhook SR diterima dari Orthanc: "+payload.StudyInstanceUID)
	switch v := payload.PatientIDINT.(type) {
	case string:
		payload.PatientID = v
	case float64:
		payload.PatientID = fmt.Sprintf("%.0f", v) // tanpa desimal
	}

	// Gunakan orthanc_uuid jika tersedia untuk langsung ambil instance
	instanceID := payload.OrthancUUID
	SavePortalLog(mwdb, "[SR] Parsing isi SR instance: "+instanceID)
	srContent, err := ParseSRContentFromOrthanc(cfg, instanceID)
	if err != nil {
		SavePortalLog(mwdb, "[SR] Gagal parsing isi SR: "+err.Error())
		log.Printf("Gagal parsing isi SR: %v", err)
		return
	}

	hasilJSON, _ := json.MarshalIndent(srContent, "", "  ")
	tglPeriksa := time.Now().Format("2006-01-02")
	jam := time.Now().Format("15:04:05")
	if err := SaveRadiologyResult(db, payload.PatientID, tglPeriksa, jam, string(hasilJSON)); err != nil {
		log.Printf("Gagal simpan hasil SR ke Khanza untuk %s: %v", payload.PatientID, err)
		SavePortalLog(mwdb, "[SR] Gagal simpan hasil SR ke Khanza untuk "+payload.PatientID+": "+err.Error())
		return
	}
	InsertPeriksaRadiologiFromPermintaan(db, payload.PatientID, payload.Link)
	log.Printf("Hasil SR %s disimpan ke Khanza", payload.PatientID)
	SavePortalLog(mwdb, "[SR] Hasil SR "+payload.PatientID+" disimpan ke Khanza")
	UpdateHasilOrthanc(mwdb, payload.PatientID, string(hasilJSON))
}

func main() {
	godotenv.Load()

	log.Println("Middleware Radiologi Khanza-Orthanc-OHIF berjalan...")
	cfg := LoadConfig()

	db, err := ConnectKhanzaDB(cfg)
	if err != nil {
		log.Fatalf("Gagal koneksi DB Khanza: %v", err)
	}
	defer db.Close()

	mwdb, err := ConnectMiddlewareDB(cfg)
	if err != nil {
		log.Fatalf("Gagal koneksi DB Middleware: %v", err)
	}
	defer mwdb.Close()

	go StartPortalServer(db, mwdb)
	go processWorklist(cfg, db, mwdb)

	http.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
		log.Println("webhook SR diterima....")
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		bodyBytes, _ := io.ReadAll(r.Body)
		log.Printf("Menerima webhook r.Body: %s", string(bodyBytes))
		go processSRWebhook(cfg, db, mwdb, bodyBytes)
		w.Write([]byte("OK"))
	})

	for {
		// Cek status koneksi
		status := Status{
			KhanzaDB:     db.Ping() == nil,
			MiddlewareDB: mwdb.Ping() == nil,
			Orthanc:      checkHTTPConnection("http://localhost:8042/"),
			OHIF:         checkHTTPConnection("http://localhost:3000/"),
		}
		UpdateStatus(status)

		time.Sleep(10 * time.Second)
	}
}

func checkHTTPConnection(url string) bool {
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}
