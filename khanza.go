package main

import (
	"database/sql"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type WorklistRequest struct {
	PatientID                         string
	PatientName                       string
	AccessionNumber                   string
	PatientBirthDate                  string
	PatientSex                        string
	RequestedProcedureID              string
	RequestedProcedureDescription     string
	ScheduledProcedureStepID          string
	ScheduledProcedureStepStartDate   string
	ScheduledProcedureStepStartTime   string
	Modality                          string
	ScheduledStationAETitle           string
	ScheduledStationName              string
	ScheduledProcedureStepDescription string
	ScheduledPerformingPhysicianName  string
}

func ConnectKhanzaDB(cfg Config) (*sql.DB, error) {
	dsn := cfg.DBKhanzaUser + ":" + cfg.DBKhanzaPassword + "@tcp(" + cfg.DBKhanzaHost + ":" + cfg.DBKhanzaPort + ")/" + cfg.DBKhanzaName
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	// Tambahkan pengaturan pool berikut:
	db.SetMaxIdleConns(5)
	db.SetMaxOpenConns(20)
	db.SetConnMaxLifetime(60 * time.Minute) // koneksi akan di-refresh setiap 60 menit
	db.SetConnMaxIdleTime(5 * time.Minute)  // idle lebih dari 5 menit akan ditutup

	if err := db.Ping(); err != nil {
		return nil, err
	}
	log.Println("Successfully connected to Khanza DB")
	return db, nil
}

func GetPendingWorklist(db *sql.DB, tglPermintaan string) ([]WorklistRequest, error) {
	query := `SELECT
    pr.noorder AS PatientID,
    p.nm_pasien AS PatientName,
    DATE_FORMAT(p.tgl_lahir, '%Y%m%d') AS PatientBirthDate,
    CASE p.jk
        WHEN 'L' THEN 'M'
        WHEN 'P' THEN 'F'
        ELSE 'O'
    END AS PatientSex,
    p.no_rkm_medis AS AccessionNumber,
	IFNULL(pj.kd_jenis_prw, '') AS RequestedProcedureID,
	IFNULL(jpr.nm_perawatan, '') AS RequestedProcedureDescription,
    CONCAT(pr.noorder, DATE_FORMAT(pr.tgl_permintaan, '%Y%m%d'), REPLACE(pr.jam_permintaan, ':', '')) AS ScheduledProcedureStepID,
    DATE_FORMAT(pr.tgl_permintaan, '%Y%m%d') AS ScheduledProcedureStepStartDate,
    REPLACE(pr.jam_permintaan, ':', '') AS ScheduledProcedureStepStartTime,
	CASE
    WHEN jpr.nm_perawatan LIKE '%CR%' OR jpr.nm_perawatan LIKE '%Rontgen%' OR jpr.nm_perawatan LIKE '%Computed Radiography%' THEN 'CR'
    WHEN jpr.nm_perawatan LIKE '%CT%' OR jpr.nm_perawatan LIKE '%CT Scan%' THEN 'CT'
    WHEN jpr.nm_perawatan LIKE '%DX%' OR jpr.nm_perawatan LIKE '%General Radiography%' THEN 'DX'
    WHEN jpr.nm_perawatan LIKE '%ECG%' OR jpr.nm_perawatan LIKE '%Elektrokardiogram%' OR jpr.nm_perawatan LIKE '%Electrocardiogram%' THEN 'ECG'
    WHEN jpr.nm_perawatan LIKE '%EPS%' OR jpr.nm_perawatan LIKE '%Electrophysiology%' THEN 'EPS'
    WHEN jpr.nm_perawatan LIKE '%ES%' OR jpr.nm_perawatan LIKE '%Endoscopy%' THEN 'ES'
    WHEN jpr.nm_perawatan LIKE '%MG%' OR jpr.nm_perawatan LIKE '%Mammo%' OR jpr.nm_perawatan LIKE '%Mammografi%' OR jpr.nm_perawatan LIKE '%Mammography%' THEN 'MG'
    WHEN jpr.nm_perawatan LIKE '%MR%' OR jpr.nm_perawatan LIKE '%MRI%' OR jpr.nm_perawatan LIKE '%Magnetic Resonance%' THEN 'MR'
    WHEN jpr.nm_perawatan LIKE '%NM%' OR jpr.nm_perawatan LIKE '%Nuklir%' OR jpr.nm_perawatan LIKE '%Nuclear Medicine%' THEN 'NM'
    WHEN jpr.nm_perawatan LIKE '%OT%' OR jpr.nm_perawatan LIKE '%Other%' OR jpr.nm_perawatan LIKE '%Lain%' THEN 'OT'
    WHEN jpr.nm_perawatan LIKE '%PT%' OR jpr.nm_perawatan LIKE '%PET%' OR jpr.nm_perawatan LIKE '%Positron%' THEN 'PT'
    WHEN jpr.nm_perawatan LIKE '%SC%' OR jpr.nm_perawatan LIKE '%Scan Sekunder%' OR jpr.nm_perawatan LIKE '%Secondary Capture%' THEN 'SC'
    WHEN jpr.nm_perawatan LIKE '%SR%' OR jpr.nm_perawatan LIKE '%Structured%' THEN 'SR'
    WHEN jpr.nm_perawatan LIKE '%US%' OR jpr.nm_perawatan LIKE '%USG%' OR jpr.nm_perawatan LIKE '%Ultrasound%' THEN 'US'
    WHEN jpr.nm_perawatan LIKE '%XA%' OR jpr.nm_perawatan LIKE '%Angio%' OR jpr.nm_perawatan LIKE '%Angiography%' THEN 'XA'
    WHEN jpr.nm_perawatan LIKE '%XC%' OR jpr.nm_perawatan LIKE '%X-Ray Cine%' THEN 'XC'
    ELSE 'OT'
    END AS Modality
FROM
    permintaan_radiologi pr
JOIN
    reg_periksa r ON pr.no_rawat = r.no_rawat
JOIN
    pasien p ON r.no_rkm_medis = p.no_rkm_medis
LEFT JOIN
    permintaan_pemeriksaan_radiologi pj ON pj.noorder LIKE CONCAT(pr.no_rawat, '%')
LEFT JOIN
    jns_perawatan_radiologi jpr ON pj.kd_jenis_prw = jpr.kd_jenis_prw
WHERE
    pr.tgl_permintaan = ?`

	rows, err := db.Query(query, tglPermintaan)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var requests []WorklistRequest
	var newReq []WorklistRequest
	for rows.Next() {
		var req WorklistRequest
		err := rows.Scan(
			&req.PatientID,
			&req.PatientName,
			&req.PatientBirthDate,
			&req.PatientSex,
			&req.AccessionNumber,
			&req.RequestedProcedureID,
			&req.RequestedProcedureDescription,
			&req.ScheduledProcedureStepID,
			&req.ScheduledProcedureStepStartDate,
			&req.ScheduledProcedureStepStartTime,
			&req.Modality,
		)
		if err != nil {
			log.Println("Error scan:", err)
			continue
		}
		requests = append(requests, req)
	}
	for _, v := range requests {
		v.AccessionNumber = v.Modality + v.AccessionNumber + time.Now().Format("2006010215")
		newReq = append(newReq, v)
	}
	return newReq, nil
}

func UpdateWorklistStatus(db *sql.DB, id int, status string) error {
	_, err := db.Exec("UPDATE permintaan_radiologi SET status=? WHERE id=?", status, id)
	return err
}

func SaveStudyLinkToKhanza(db *sql.DB, noRawat string, link string) error {
	_, err := db.Exec("UPDATE permintaan_radiologi SET link_hasil=? WHERE no_rawat=?", link, noRawat)
	return err
}

func SaveRadiologyResult(db *sql.DB, noorder, tglPeriksa, jam, hasil string) error {
	var noRawatDB string
	err := db.QueryRow("SELECT no_rawat FROM permintaan_radiologi WHERE noorder = ?", noorder).Scan(&noRawatDB)
	if err != nil {
		return err
	}
	noRawat := noRawatDB
	_, err = db.Exec(
		"INSERT INTO hasil_radiologi (no_rawat, tgl_periksa, jam, hasil) VALUES (?, ?, ?, ?) ON DUPLICATE KEY UPDATE hasil=?, no_rawat=?",
		noRawat, tglPeriksa, jam, hasil, hasil, noRawat,
	)
	return err
}
