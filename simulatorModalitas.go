package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

type ConfigMod struct {
	AETitle  string
	PACSIP   string
	PACSPort string
	StoreDir string
}

var config = ConfigMod{
	AETitle:  "MODALITAS_SIM",
	PACSIP:   "127.0.0.1",
	PACSPort: "4242",
	StoreDir: "./uploads",
}

func simulatorModalitas() {
	// Create upload folder if not exists
	os.MkdirAll(config.StoreDir, os.ModePerm)
	os.MkdirAll("logs", os.ModePerm)

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.HandleFunc("/mod", indexHandler)
	http.HandleFunc("/mod/store", storeHandler)
	http.Handle("/mod/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("uploads"))))
	http.Handle("/mod/viewer_static/", http.StripPrefix("/viewer_static/", http.FileServer(http.Dir("viewer/dwv"))))

	log.Println("Simulator modalitas berjalan di http://localhost:8000/mod")
	log.Fatal(http.ListenAndServe(":8000", nil))
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	worklistDir := os.Getenv("FOLDER_WORKLIST")
	os.MkdirAll(worklistDir, os.ModePerm)

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<style>
	body { background: #f4f8fb; font-family: 'Segoe UI', Arial, sans-serif; }
	.header-mod { background: #2a4d7a; color: #fff; padding: 24px 0 12px 0; text-align: center; letter-spacing: 2px; font-size: 2.1em; font-weight: 600; box-shadow: 0 2px 8px #b0c4de; }
	.worklist-table { border-collapse: collapse; width: 92%%; margin: 24px auto 0 auto; background: #fff; box-shadow: 0 2px 8px #ccc; border-radius: 8px; overflow: hidden; }
	.worklist-table th, .worklist-table td { border: 1px solid #bbb; padding: 10px 18px; text-align: left; font-size: 1.08em; }
	.worklist-table th { background: #eaf1fa; color: #2a4d7a; font-weight: 600; }
	.worklist-table tr:nth-child(even) { background: #f7fafd; }
	.worklist-table tr:hover { background: #e6f7ff; }
	.worklist-title { text-align:center; color:#2a4d7a; margin-top:32px; font-size:1.3em; letter-spacing:1px; }
	.upload-form { width:92%%; margin:24px auto 0 auto; background:#f7f7f7; padding:18px; border-radius:8px; box-shadow:0 1px 4px #ccc; display:flex; align-items:center; gap:12px; }
	.upload-form label { font-weight:500; color:#2a4d7a; }
	.upload-form input[type=file] { margin-right:10px; }
	.upload-form button { padding:7px 22px; background:#2a4d7a; color:#fff; border:none; border-radius:4px; cursor:pointer; font-size:1em; }
	.upload-form button:hover { background:#1a3550; }
	.action-btn { padding:6px 16px; background:#4caf50; color:#fff; border:none; border-radius:4px; cursor:pointer; font-size:0.98em; }
	.action-btn:hover { background:#357a38; }
</style>`)
	fmt.Fprintf(w, `<div class='header-mod'>Simulator Modalitas Radiologi</div>`)

	// Tabel worklist
	fmt.Fprintf(w, `<h2 class='worklist-title'>Worklist Pasien (Simulasi Modalitas)</h2><table class='worklist-table'><thead><tr><th>PatientID</th><th>PatientName</th><th>AccessionNumber</th><th>Modality</th></tr></thead><tbody>`)
	// Ambil worklist dari Orthanc REST API (C-FIND)
	orthancURL := os.Getenv("ORTHANC_URL")
	findURL := orthancURL + "/tools/find"
	// Query worklist: ModalityWorklist (MWL)
	query := map[string]interface{}{
		"Level": "Worklist",
		"Query": map[string]interface{}{},
	}
	client := &http.Client{}
	jsonQuery, _ := json.Marshal(query)
	req, _ := http.NewRequest("POST", findURL, bytes.NewReader(jsonQuery))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err == nil && resp.StatusCode == 200 {
		defer resp.Body.Close()
		var results []map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&results)
		for _, item := range results {
			patID := getDicomStr(item, "0010,0020")
			patName := getDicomStr(item, "0010,0010")
			accNum := getDicomStr(item, "0008,0050")
			modality := getDicomStr(item, "0008,0060")
			fmt.Fprintf(w, "<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>", patID, patName, accNum, modality)
		}
	}
	fmt.Fprintf(w, "</tbody></table>")
	// Form store DICOM ke PACS (Orthanc)
	fmt.Fprintf(w, `<form class='upload-form' id='uploadForm' action='/mod/store' method='post' enctype='multipart/form-data'>
		<label>Store DICOM ke PACS (Orthanc):</label>
		<input type='file' name='dicomFile' required>
		<button type='submit'>Store ke PACS</button>
	</form>`)

	// Tambahkan script AJAX upload + alert
	fmt.Fprintf(w, `<script>
	document.getElementById('uploadForm').onsubmit = function(e) {
		e.preventDefault();
		var form = this;
		var data = new FormData(form);
		var xhr = new XMLHttpRequest();
		xhr.open('POST', form.action, true);
		xhr.setRequestHeader('X-Requested-With', 'XMLHttpRequest');
		xhr.onreadystatechange = function() {
			if (xhr.readyState == 4) {
				if (xhr.status == 200) {
					try {
						var resp = JSON.parse(xhr.responseText);
						alert('Sukses: ' + (resp.message || 'DICOM berhasil di-store ke Orthanc!'));
						form.reset();
						window.location.reload();
					} catch (e) {
						alert('Sukses, tapi response tidak valid!');
						window.location.reload();
					}
				} else {
					try {
						var resp = JSON.parse(xhr.responseText);
						alert('Gagal: ' + (resp.error || xhr.statusText));
					} catch (e) {
						alert('Gagal upload: ' + xhr.responseText);
					}
				}
			}
		};
		xhr.send(data);
	};
</script>`)
}

// splitLines membagi string menjadi slice baris (tanpa \r\n)
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func storeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Redirect(w, r, "/mod", http.StatusSeeOther)
		return
	}
	file, handler, err := r.FormFile("dicomFile")
	if err != nil {
		respondStoreError(w, "Gagal ambil file DICOM", 500)
		return
	}
	defer file.Close()

	// Simpan file sementara
	tempPath := filepath.Join(os.TempDir(), handler.Filename)
	f, err := os.Create(tempPath)
	if err != nil {
		respondStoreError(w, "Gagal simpan file sementara", 500)
		return
	}
	defer f.Close()
	io.Copy(f, file)

	dicomPath := tempPath

	// Kirim ke Orthanc REST API /instances
	orthancURL := os.Getenv("ORTHANC_URL")
	storeURL := orthancURL + "/instances"
	f2, err := os.Open(dicomPath)
	if err != nil {
		respondStoreError(w, "Gagal buka file untuk upload ke Orthanc", 500)
		return
	}
	defer f2.Close()
	resp, err := http.Post(storeURL, "application/dicom", f2)
	if err != nil {
		respondStoreError(w, "Gagal upload ke Orthanc: "+err.Error(), 500)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		msg, _ := io.ReadAll(resp.Body)
		respondStoreError(w, fmt.Sprintf("Orthanc error: %s", string(msg)), 500)
		return
	}
	log.Println("📤 DICOM berhasil di-store ke Orthanc REST API:", handler.Filename)
	if isAjax(r) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": fmt.Sprintf("DICOM '%s' berhasil di-store ke Orthanc!", handler.Filename),
		})
	} else {
		http.Redirect(w, r, "/mod", http.StatusSeeOther)
	}
}

func isAjax(r *http.Request) bool {
	return r.Header.Get("X-Requested-With") == "XMLHttpRequest" || r.Header.Get("Accept") == "application/json"
}

func respondStoreError(w http.ResponseWriter, msg string, code int) {
	if code == 0 {
		code = 500
	}
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// Helper untuk ambil string DICOM tag dari hasil Orthanc REST API
func getDicomStr(m map[string]interface{}, tag string) string {
	if v, ok := m[tag]; ok {
		if arr, ok := v.([]interface{}); ok && len(arr) > 0 {
			if s, ok := arr[0].(string); ok {
				return s
			}
		}
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
