package main

import (
	"database/sql"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"sync"
)

type Worklist struct {
	AccessionNumber string
	PatientName     string
	Modality        string
	StudyDate       string
}

type Status struct {
	KhanzaDB     bool `json:"khanza_db"`
	MiddlewareDB bool `json:"middleware_db"`
	Orthanc      bool `json:"orthanc"`
	OHIF         bool `json:"ohif"`
}

var tmpl = `
<!DOCTYPE html>
<html>
<head>
    <title>Portal Worklist Radiologi</title>
    <style>
        body { font-family: Arial; margin: 40px; }
        table { border-collapse: collapse; width: 100%; }
        th, td { border: 1px solid #ccc; padding: 8px; text-align: left; }
        th { background: #f0f0f0; }
    </style>
</head>
<body>
    <h2>Daftar Worklist Radiologi</h2>
    <table>
        <tr>
            <th>Accession Number</th>
            <th>Patient Name</th>
            <th>Modality</th>
            <th>Study Date</th>
        </tr>
        {{range .}}
        <tr>
            <td>{{.AccessionNumber}}</td>
            <td>{{.PatientName}}</td>
            <td>{{.Modality}}</td>
            <td>{{.StudyDate}}</td>
        </tr>
        {{end}}
    </table>
</body>
</html>
`

var (
	CurrentStatus    Status
	CurrentWorklists []Worklist
	statusMutex      sync.RWMutex
	worklistMutex    sync.RWMutex
)

// Fungsi untuk update status dari main.go
func UpdateStatus(s Status) {
	statusMutex.Lock()
	defer statusMutex.Unlock()
	CurrentStatus = s
}

// Fungsi untuk update worklist dari main.go
func UpdateWorklists(wl []Worklist) {
	worklistMutex.Lock()
	defer worklistMutex.Unlock()
	CurrentWorklists = wl
}

// Fungsi untuk baca status (untuk handler)
func GetStatus() Status {
	statusMutex.RLock()
	defer statusMutex.RUnlock()
	return CurrentStatus
}

// Fungsi untuk baca worklist (untuk handler)
func GetWorklists() []Worklist {
	worklistMutex.RLock()
	defer worklistMutex.RUnlock()
	return CurrentWorklists
}

func SavePortalLog(db *sql.DB, msg string) {
	_, _ = db.Exec("INSERT INTO log_portal (waktu, pesan) VALUES (NOW(), ?)", msg)
}

func GetPortalLogs(db *sql.DB, limit int) ([]string, error) {
	rows, err := db.Query("SELECT CONCAT(DATE_FORMAT(waktu, '%Y-%m-%d %H:%i:%s'), ' : ', pesan) FROM log_portal ORDER BY waktu DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var logs []string
	for rows.Next() {
		var msg string
		if err := rows.Scan(&msg); err == nil {
			logs = append(logs, msg)
		}
	}
	// reverse agar urutan lama ke baru
	for i, j := 0, len(logs)-1; i < j; i, j = i+1, j-1 {
		logs[i], logs[j] = logs[j], logs[i]
	}
	return logs, nil
}

func StartPortalServer(db *sql.DB, mwdb *sql.DB) {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		status := GetStatus()
		logs, _ := GetPortalLogs(mwdb, 200)
		tmpl := `
<!DOCTYPE html>
<html>
<head>
    <title>Dashboard Monitoring Radiologi</title>
    <style>
        body { font-family: Arial; margin: 40px; }
        table { border-collapse: collapse; width: 400px; }
        th, td { border: 1px solid #ccc; padding: 8px; text-align: left; }
        th { background: #f0f0f0; }
        .ok { color: green; font-weight: bold; }
        .fail { color: red; font-weight: bold; }
        .logbox { background: #222; color: #eee; padding: 10px; margin-top: 30px; height: 200px; overflow-y: scroll; font-size: 13px; }
    </style>
</head>
<body>
    <h2>Dashboard Monitoring Koneksi</h2>
    <table>
        <tr><th>Komponen</th><th>Status</th></tr>
        <tr><td>DB Khanza</td><td id="status-khanza">{{if .Status.KhanzaDB}}<span class='ok'>Tersambung</span>{{else}}<span class='fail'>Gagal</span>{{end}}</td></tr>
        <tr><td>DB Middleware</td><td id="status-mw">{{if .Status.MiddlewareDB}}<span class='ok'>Tersambung</span>{{else}}<span class='fail'>Gagal</span>{{end}}</td></tr>
        <tr><td>Orthanc</td><td id="status-orthanc">{{if .Status.Orthanc}}<span class='ok'>Tersambung</span>{{else}}<span class='fail'>Gagal</span>{{end}}</td></tr>
        <tr><td>OHIF</td><td id="status-ohif">{{if .Status.OHIF}}<span class='ok'>Tersambung</span>{{else}}<span class='fail'>Gagal</span>{{end}}</td></tr>
    </table>
    <div id="logbox" class="logbox">
    {{range .Logs}}{{.}}<br>{{end}}
    </div>
    <script>
    function updateLogs() {
        fetch('/logs').then(r => r.json()).then(arr => {
            var logbox = document.getElementById('logbox');
            if (!Array.isArray(arr) || !logbox) return;
            logbox.innerHTML = arr.map(x => x + '<br>').join('');
            setTimeout(function() {
                logbox.scrollTop = logbox.scrollHeight;
            }, 50);
        });
    }
    function updateStatus() {
        fetch('/status').then(r => r.json()).then(st => {
            document.getElementById('status-khanza').innerHTML = st.khanza_db ? "<span class='ok'>Tersambung</span>" : "<span class='fail'>Gagal</span>";
            document.getElementById('status-mw').innerHTML = st.middleware_db ? "<span class='ok'>Tersambung</span>" : "<span class='fail'>Gagal</span>";
            document.getElementById('status-orthanc').innerHTML = st.orthanc ? "<span class='ok'>Tersambung</span>" : "<span class='fail'>Gagal</span>";
            document.getElementById('status-ohif').innerHTML = st.ohif ? "<span class='ok'>Tersambung</span>" : "<span class='fail'>Gagal</span>";
        });
    }
    window.onload = function() {
        updateLogs();
        updateStatus();
    };
    setInterval(updateLogs, 5000);
    setInterval(updateStatus, 5000);
    </script>
</body>
</html>
`
		t, _ := template.New("dashboard").Parse(tmpl)
		t.Execute(w, struct {
			Status Status
			Logs   []string
		}{status, logs})
	})

	http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		status := GetStatus()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	})

	http.HandleFunc("/api/worklists", func(w http.ResponseWriter, r *http.Request) {
		worklists := GetWorklists()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(worklists)
	})

	http.HandleFunc("/logs", func(w http.ResponseWriter, r *http.Request) {
		logs, _ := GetPortalLogs(db, 200)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(logs)
	})

	log.Println("Portal web berjalan di http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
