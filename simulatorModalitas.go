package main

import (
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

	log.Println("Simulator modalitas berjalan di http://localhost:8090/mod")
	log.Fatal(http.ListenAndServe(":8090", nil))
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
	worklistDir := "./worklists" // Folder penyimpanan file .wl
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

	// Jika tidak ada parameter file, tampilkan daftar file .wl
	files, err := os.ReadDir(worklistDir)
	if err != nil {
		http.Error(w, "Gagal membaca folder worklists", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, "<h2>Daftar Worklist (.wl)</h2><ul>")
	for _, f := range files {
		if !f.IsDir() && filepath.Ext(f.Name()) == ".wl" {
			fmt.Fprintf(w, "<li><a href='/mod/worklist?file=%s'>%s</a></li>", f.Name(), f.Name())
		}
	}
	fmt.Fprintf(w, "</ul>")
}
