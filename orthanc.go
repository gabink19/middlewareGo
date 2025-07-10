package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
)

type DicomWorklist struct {
	PatientName string `json:"PatientName"`
	PatientID   string `json:"PatientID"`
	StudyDesc   string `json:"StudyDescription"`
	Accession   string `json:"AccessionNumber"`
}

type OrthancStudy struct {
	StudyInstanceUID string `json:"MainDicomTags.StudyInstanceUID"`
	PatientID        string `json:"MainDicomTags.PatientID"`
}

func SendWorklistToOrthanc(cfg Config, wl WorklistRequest) error {
	dir := os.Getenv("FOLDER_WORKLIST")
	if dir == "" {
		dir = "./worklists"
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("gagal membuat folder: %v", err)
	}

	txtPath := filepath.Join(dir, wl.AccessionNumber+".txt")
	wlPath := filepath.Join(dir, wl.AccessionNumber+".wl")

	txtContent := fmt.Sprintf(`(0010,0010) PN [%s]
(0010,0020) LO [%s]
(0008,0050) SH [%s]
(0008,0060) CS [%s]
(0032,1060) LO [%s]
(0040,0100) SQ
  (fffe,e000) na
    (0040,0001) AE [%s]
    (0040,0002) DA [%s]
    (0040,0003) TM [%s]
    (0040,0006) PN [%s]
    (0040,0007) LO [%s]
    (0040,0009) SH [%s]
    (0040,0010) SH [%s]
    (0040,0020) CS [SCHEDULED]
  (fffe,e00d) na
(fffe,e0dd) na
`,
		wl.PatientName,
		wl.PatientID,
		wl.AccessionNumber,
		wl.Modality,
		wl.RequestedProcedureDescription,
		wl.ScheduledStationAETitle,
		wl.ScheduledProcedureStepStartDate,
		wl.ScheduledProcedureStepStartTime,
		wl.ScheduledPerformingPhysicianName,
		wl.ScheduledProcedureStepDescription,
		wl.ScheduledProcedureStepID,
		wl.ScheduledStationName,
	)

	// Simpan file txt
	if err := os.WriteFile(txtPath, []byte(txtContent), 0644); err != nil {
		return fmt.Errorf("gagal menyimpan file TXT DICOM: %v", err)
	}

	// Konversi dengan dump2dcm
	cmd := exec.Command("dump2dcm", txtPath, wlPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gagal menjalankan dump2dcm: %v\n%s", err, string(output))
	}

	log.Printf("✅ Worklist berhasil dibuat: %s", wlPath)
	return nil
}

func GetNewStudiesFromOrthanc(cfg Config) ([]OrthancStudy, error) {
	url := cfg.OrthancURL + "/studies"
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Orthanc error: %s", resp.Status)
	}
	var studyIDs []string
	if err := json.NewDecoder(resp.Body).Decode(&studyIDs); err != nil {
		return nil, err
	}
	var studies []OrthancStudy
	for _, id := range studyIDs {
		studyURL := cfg.OrthancURL + "/studies/" + id
		resp2, err := http.Get(studyURL)
		if err != nil {
			continue
		}
		defer resp2.Body.Close()
		if resp2.StatusCode != 200 {
			continue
		}
		var study OrthancStudy
		if err := json.NewDecoder(resp2.Body).Decode(&study); err != nil {
			continue
		}
		studies = append(studies, study)
	}
	return studies, nil
}

// Parsing isi Structured Report (SR) dari Orthanc
func ParseSRContentFromOrthanc(cfg Config, instanceID string) (string, error) {
	url := cfg.OrthancURL + "/instances/" + instanceID + "/tags"
	log.Println("Fetching SR content from:", url)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Orthanc error: %s", resp.Status)
	}
	bodyBytes, _ := io.ReadAll(resp.Body)

	// Unmarshal ke map
	var tags map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &tags); err != nil {
		log.Printf("Gagal decode SR content: %v", err)
		return "", fmt.Errorf("invalid JSON payload: %v", err)
	}

	// Ambil ContentSequence (0040,a730)
	contentSeq, ok := tags["0040,a730"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("ContentSequence (0040,a730) tidak ditemukan")
	}
	valArr, ok := contentSeq["Value"].([]interface{})
	if !ok || len(valArr) == 0 {
		return "", fmt.Errorf("ContentSequence.Value kosong")
	}
	// Ambil item pertama
	item, ok := valArr[0].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("Item pertama ContentSequence tidak valid")
	}
	// Ambil TextValue (0040,a160)
	textVal, ok := item["0040,a160"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("TextValue (0040,a160) tidak ditemukan")
	}
	val, ok := textVal["Value"].(string)
	if ok {
		return val, nil
	}
	// Kadang Value bisa []interface{}
	if arr, ok := textVal["Value"].([]interface{}); ok && len(arr) > 0 {
		if s, ok := arr[0].(string); ok {
			return s, nil
		}
	}
	return "", fmt.Errorf("TextValue (0040,a160) tidak ada Value string")
}
