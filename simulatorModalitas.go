package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
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
	http.HandleFunc("/mod/upload", uploadHandler)
	http.HandleFunc("/mod/send", sendHandler)
	http.HandleFunc("/mod/viewer", viewerHandler)
	http.HandleFunc("/mod/worklist", worklistHandler) // Endpoint baru untuk simulasi ambil worklist dari Orthanc
	http.Handle("/mod/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("uploads"))))
	http.Handle("/mod/viewer_static/", http.StripPrefix("/viewer_static/", http.FileServer(http.Dir("viewer/dwv"))))

	log.Println("Simulator modalitas berjalan di http://localhost:8000/mod")
	log.Fatal(http.ListenAndServe(":8000", nil))
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("templates/index.html"))
	tmpl.Execute(w, config)
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Redirect(w, r, "/mod", http.StatusSeeOther)
		return
	}
	file, handler, err := r.FormFile("dicomFile")
	if err != nil {
		http.Error(w, "Gagal ambil file", 500)
		return
	}
	defer file.Close()

	f, err := os.Create(filepath.Join(config.StoreDir, handler.Filename))
	if err != nil {
		http.Error(w, "Gagal simpan file", 500)
		return
	}
	defer f.Close()
	io.Copy(f, file)

	log.Println("📥 DICOM berhasil di-upload:", handler.Filename)
	http.Redirect(w, r, "/mod", http.StatusSeeOther)
}

func sendHandler(w http.ResponseWriter, r *http.Request) {
	file := r.URL.Query().Get("file")
	fullPath := filepath.Join(config.StoreDir, file)

	cmd := exec.Command("storescu", "-v", "-aec", config.AETitle, config.PACSIP, config.PACSPort, fullPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		http.Error(w, fmt.Sprintf("❌ Gagal kirim DICOM: %s", string(out)), 500)
		return
	}
	log.Println("📤 DICOM berhasil dikirim ke PACS:", file)
	http.Redirect(w, r, "/mod", http.StatusSeeOther)
}
func viewerHandler(w http.ResponseWriter, r *http.Request) {
	file := r.URL.Query().Get("file")
	tmpl := template.Must(template.ParseFiles("viewer/viewer.html"))
	tmpl.Execute(w, map[string]string{"File": file})
}

// Handler untuk simulasi modalitas mengambil worklist berupa file .wl dari folder lokal
func worklistHandler(w http.ResponseWriter, r *http.Request) {
	worklistDir := os.Getenv("FOLDER_WORKLIST")
	os.MkdirAll(worklistDir, os.ModePerm)

	file := r.URL.Query().Get("file")
	if file != "" {
		// Jika ada parameter file, tampilkan/unduh file .wl
		fullPath := filepath.Join(worklistDir, file)
		f, err := os.Open(fullPath)
		if err != nil {
			http.Error(w, "File tidak ditemukan", 404)
			return
		}
		defer f.Close()
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Disposition", "attachment; filename="+file)
		io.Copy(w, f)
		return
	}

	// Tampilkan isi semua file .wl dalam satu tabel gabungan (header: PatientID, PatientName, AccessionNumber, Modality)
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, "<h2>Isi Semua Worklist (.wl)</h2><table border='1' cellpadding='5' style='font-family:monospace;'><thead><tr><th>PatientID</th><th>PatientName</th><th>AccessionNumber</th><th>Modality</th></tr></thead><tbody>")
	files, err := os.ReadDir(worklistDir)
	if err == nil {
		for _, f := range files {
			if !f.IsDir() && filepath.Ext(f.Name()) == ".wl" {
				fullPath := filepath.Join(worklistDir, f.Name())
				content, err := os.ReadFile(fullPath)
				if err != nil {
					continue
				}
				// Coba parse JSON worklist
				var wl struct {
					PatientID       string `json:"PatientID"`
					PatientName     string `json:"PatientName"`
					AccessionNumber string `json:"AccessionNumber"`
					Modality        string `json:"Modality"`
				}
				if err := json.Unmarshal(content, &wl); err == nil {
					fmt.Fprintf(w, "<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>",
						wl.PatientID, wl.PatientName, wl.AccessionNumber, wl.Modality)
				}
			}
		}
	}
	fmt.Fprintf(w, "</tbody></table>")

	// Tampilkan hasil C-FIND ke Orthanc (tanpa MWL, misal query study list)
	orthancAET := "ORTHANC"
	cmd := exec.Command("findscu", "-v", "-S", "-aec", orthancAET, os.Getenv("ORTHANC_IP"), os.Getenv("ORTHANC_PORT"))
	out, _ := cmd.CombinedOutput()
	fmt.Fprintf(w, "<h2>Hasil Query C-FIND Study List (findscu)</h2><pre style='background:#eee;padding:10px;'>%s</pre>", template.HTMLEscapeString(string(out)))
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
