package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
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

// Kirim worklist ke Orthanc dengan field DICOM dari WorklistRequest
func SendWorklistToOrthanc(cfg Config, wl WorklistRequest) error {
	payload := map[string]interface{}{
		"PatientName":                     wl.PatientName,
		"PatientID":                       wl.PatientID,
		"PatientBirthDate":                wl.PatientBirthDate,
		"PatientSex":                      wl.PatientSex,
		"AccessionNumber":                 wl.AccessionNumber,
		"RequestedProcedureID":            wl.RequestedProcedureID,
		"RequestedProcedureDescription":   wl.RequestedProcedureDescription,
		"ScheduledProcedureStepID":        wl.ScheduledProcedureStepID,
		"ScheduledProcedureStepStartDate": wl.ScheduledProcedureStepStartDate,
		"ScheduledProcedureStepStartTime": wl.ScheduledProcedureStepStartTime,
		"Modality":                        wl.Modality,
	}
	jsonData, _ := json.MarshalIndent(payload, "", "  ")
	accssNmbr := strings.ReplaceAll(wl.AccessionNumber, "/", "-")
	filename := os.Getenv("FOLDER_WORKLIST") + accssNmbr + "_" + time.Now().Format("20060102_150405") + ".wl"
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(jsonData)
	if err != nil {
		return err
	}
	log.Printf("Worklist file written: %s", filename)
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

// Deteksi study yang memiliki hasil DICOM SR dari Orthanc
func DetectSRStudiesFromOrthanc(cfg Config) ([]OrthancStudy, error) {
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
	var srStudies []OrthancStudy
	for _, id := range studyIDs {
		seriesURL := cfg.OrthancURL + "/studies/" + id + "/series"
		resp2, err := http.Get(seriesURL)
		if err != nil {
			continue
		}
		if resp2.StatusCode != 200 {
			resp2.Body.Close()
			continue
		}
		var seriesIDs []string
		if err := json.NewDecoder(resp2.Body).Decode(&seriesIDs); err != nil {
			resp2.Body.Close()
			continue
		}
		resp2.Body.Close()
		for _, sid := range seriesIDs {
			seriesInfoURL := cfg.OrthancURL + "/series/" + sid
			resp3, err := http.Get(seriesInfoURL)
			if err != nil {
				continue
			}
			if resp3.StatusCode != 200 {
				resp3.Body.Close()
				continue
			}
			var seriesInfo struct {
				MainDicomTags struct {
					Modality string `json:"Modality"`
				} `json:"MainDicomTags"`
			}
			if err := json.NewDecoder(resp3.Body).Decode(&seriesInfo); err != nil {
				resp3.Body.Close()
				continue
			}
			resp3.Body.Close()
			if seriesInfo.MainDicomTags.Modality == "SR" {
				// Ambil info study
				studyURL := cfg.OrthancURL + "/studies/" + id
				resp4, err := http.Get(studyURL)
				if err != nil {
					continue
				}
				if resp4.StatusCode != 200 {
					resp4.Body.Close()
					continue
				}
				var study OrthancStudy
				if err := json.NewDecoder(resp4.Body).Decode(&study); err != nil {
					resp4.Body.Close()
					continue
				}
				resp4.Body.Close()
				srStudies = append(srStudies, study)
				break // satu SR cukup
			}
		}
	}
	return srStudies, nil
}

// Parsing isi Structured Report (SR) dari Orthanc
func ParseSRContentFromOrthanc(cfg Config, instanceID string) (map[string]interface{}, error) {
	// instanceID adalah ID instance DICOM SR
	url := cfg.OrthancURL + "/instances/" + instanceID + "/content"
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Orthanc error: %s", resp.Status)
	}
	var srContent map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&srContent); err != nil {
		return nil, err
	}
	return srContent, nil
}
