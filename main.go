package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
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
			err := SendWorklistToOrthanc(cfg, wl)
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

func processSRDetection(cfg Config, db, mwdb *sql.DB) {
	srStudies, err := DetectSRStudiesFromOrthanc(cfg)
	if err != nil {
		log.Printf("Gagal deteksi SR dari Orthanc: %v", err)
		SavePortalLog(mwdb, "[SR] Gagal deteksi SR dari Orthanc: "+err.Error())
	} else {
		SavePortalLog(mwdb, "[SR] Jumlah study SR ditemukan: "+fmt.Sprint(len(srStudies)))
		for _, study := range srStudies {
			SavePortalLog(mwdb, "[SR] Proses study SR: "+study.StudyInstanceUID)
			link := GenerateOHIFLink(cfg, study.StudyInstanceUID)
			if err := SaveStudyLinkToKhanza(db, study.PatientID, link); err != nil {
				log.Printf("Gagal update link hasil SR ke Khanza untuk %s: %v", study.PatientID, err)
				SavePortalLog(mwdb, "[SR] Gagal update link hasil SR ke Khanza untuk "+study.PatientID+": "+err.Error())
			} else {
				log.Printf("Link hasil SR %s disimpan ke Khanza", study.PatientID)
				SavePortalLog(mwdb, "[SR] Link hasil SR "+study.PatientID+" disimpan ke Khanza")
			}
			seriesURL := cfg.OrthancURL + "/studies/" + study.StudyInstanceUID + "/series"
			resp, err := http.Get(seriesURL)
			if err != nil {
				SavePortalLog(mwdb, "[SR] Gagal ambil series SR: "+err.Error())
				continue
			}
			var seriesIDs []string
			if err := json.NewDecoder(resp.Body).Decode(&seriesIDs); err != nil {
				resp.Body.Close()
				SavePortalLog(mwdb, "[SR] Gagal decode series SR: "+err.Error())
				continue
			}
			resp.Body.Close()
			if len(seriesIDs) == 0 {
				SavePortalLog(mwdb, "[SR] Tidak ada series SR ditemukan")
				continue
			}
			instancesURL := cfg.OrthancURL + "/series/" + seriesIDs[0] + "/instances"
			resp2, err := http.Get(instancesURL)
			if err != nil {
				SavePortalLog(mwdb, "[SR] Gagal ambil instance SR: "+err.Error())
				continue
			}
			var instanceIDs []string
			if err := json.NewDecoder(resp2.Body).Decode(&instanceIDs); err != nil {
				resp2.Body.Close()
				SavePortalLog(mwdb, "[SR] Gagal decode instance SR: "+err.Error())
				continue
			}
			resp2.Body.Close()
			if len(instanceIDs) == 0 {
				SavePortalLog(mwdb, "[SR] Tidak ada instance SR ditemukan")
				continue
			}
			SavePortalLog(mwdb, "[SR] Parsing isi SR instance: "+instanceIDs[0])
			srContent, err := ParseSRContentFromOrthanc(cfg, instanceIDs[0])
			if err != nil {
				SavePortalLog(mwdb, "[SR] Gagal parsing isi SR: "+err.Error())
				continue
			}
			hasilJSON, _ := json.MarshalIndent(srContent, "", "  ")
			tglPeriksa := time.Now().Format("2006-01-02")
			jam := time.Now().Format("15:04:05")
			petugas := "SYSTEM"
			if err := SaveRadiologyResult(db, study.PatientID, tglPeriksa, jam, string(hasilJSON), petugas); err != nil {
				log.Printf("Gagal simpan hasil SR ke Khanza untuk %s: %v", study.PatientID, err)
				SavePortalLog(mwdb, "[SR] Gagal simpan hasil SR ke Khanza untuk "+study.PatientID+": "+err.Error())
			} else {
				log.Printf("Hasil SR %s disimpan ke Khanza", study.PatientID)
				SavePortalLog(mwdb, "[SR] Hasil SR "+study.PatientID+" disimpan ke Khanza")
				UpdateHasilOrthanc(db, study.PatientID, study.StudyInstanceUID)
			}
		}
	}
}

func processSRWebhook(cfg Config, db, mwdb *sql.DB, w http.ResponseWriter, r *http.Request) {
	// Webhook Orthanc Lua biasanya mengirim JSON POST berisi StudyInstanceUID, PatientID, dsb
	var payload struct {
		StudyInstanceUID string `json:"StudyInstanceUID"`
		PatientID        string `json:"PatientID"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Invalid JSON payload"))
		return
	}
	SavePortalLog(mwdb, "[SR] Webhook SR diterima dari Orthanc: "+payload.StudyInstanceUID)

	// Proses SR hanya untuk StudyInstanceUID yang dikirim
	link := GenerateOHIFLink(cfg, payload.StudyInstanceUID)
	if err := SaveStudyLinkToKhanza(db, payload.PatientID, link); err != nil {
		log.Printf("Gagal update link hasil SR ke Khanza untuk %s: %v", payload.PatientID, err)
		SavePortalLog(mwdb, "[SR] Gagal update link hasil SR ke Khanza untuk "+payload.PatientID+": "+err.Error())
	} else {
		log.Printf("Link hasil SR %s disimpan ke Khanza", payload.PatientID)
		SavePortalLog(mwdb, "[SR] Link hasil SR "+payload.PatientID+" disimpan ke Khanza")
	}

	seriesURL := cfg.OrthancURL + "/studies/" + payload.StudyInstanceUID + "/series"
	resp, err := http.Get(seriesURL)
	if err != nil {
		SavePortalLog(mwdb, "[SR] Gagal ambil series SR: "+err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Gagal ambil series SR"))
		return
	}
	var seriesIDs []string
	if err := json.NewDecoder(resp.Body).Decode(&seriesIDs); err != nil {
		resp.Body.Close()
		SavePortalLog(mwdb, "[SR] Gagal decode series SR: "+err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Gagal decode series SR"))
		return
	}
	resp.Body.Close()
	if len(seriesIDs) == 0 {
		SavePortalLog(mwdb, "[SR] Tidak ada series SR ditemukan")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Tidak ada series SR ditemukan"))
		return
	}
	instancesURL := cfg.OrthancURL + "/series/" + seriesIDs[0] + "/instances"
	resp2, err := http.Get(instancesURL)
	if err != nil {
		SavePortalLog(mwdb, "[SR] Gagal ambil instance SR: "+err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Gagal ambil instance SR"))
		return
	}
	var instanceIDs []string
	if err := json.NewDecoder(resp2.Body).Decode(&instanceIDs); err != nil {
		resp2.Body.Close()
		SavePortalLog(mwdb, "[SR] Gagal decode instance SR: "+err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Gagal decode instance SR"))
		return
	}
	resp2.Body.Close()
	if len(instanceIDs) == 0 {
		SavePortalLog(mwdb, "[SR] Tidak ada instance SR ditemukan")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Tidak ada instance SR ditemukan"))
		return
	}
	SavePortalLog(mwdb, "[SR] Parsing isi SR instance: "+instanceIDs[0])
	srContent, err := ParseSRContentFromOrthanc(cfg, instanceIDs[0])
	if err != nil {
		SavePortalLog(mwdb, "[SR] Gagal parsing isi SR: "+err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Gagal parsing isi SR"))
		return
	}
	hasilJSON, _ := json.MarshalIndent(srContent, "", "  ")
	tglPeriksa := time.Now().Format("2006-01-02")
	jam := time.Now().Format("15:04:05")
	petugas := "SYSTEM"
	if err := SaveRadiologyResult(db, payload.PatientID, tglPeriksa, jam, string(hasilJSON), petugas); err != nil {
		log.Printf("Gagal simpan hasil SR ke Khanza untuk %s: %v", payload.PatientID, err)
		SavePortalLog(mwdb, "[SR] Gagal simpan hasil SR ke Khanza untuk "+payload.PatientID+": "+err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Gagal simpan hasil SR ke Khanza"))
		return
	} else {
		log.Printf("Hasil SR %s disimpan ke Khanza", payload.PatientID)
		SavePortalLog(mwdb, "[SR] Hasil SR "+payload.PatientID+" disimpan ke Khanza")
		UpdateHasilOrthanc(db, payload.PatientID, payload.StudyInstanceUID)
	}
	w.Write([]byte("SR detection processed (webhook lua orthanc)"))
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
	go simulatorModalitas()

	http.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
		processSRWebhook(cfg, db, mwdb, w, r)
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
