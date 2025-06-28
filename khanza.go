package main

import (
	"database/sql"
	"log"

	_ "github.com/go-sql-driver/mysql"
)

type WorklistRequest struct {
	PatientID                       string
	PatientName                     string
	PatientBirthDate                string
	PatientSex                      string
	AccessionNumber                 string
	RequestedProcedureID            *string
	RequestedProcedureDescription   *string
	ScheduledProcedureStepID        string
	ScheduledProcedureStepStartDate string
	ScheduledProcedureStepStartTime string
	Modality                        string
}

func ConnectKhanzaDB(cfg Config) (*sql.DB, error) {
	dsn := cfg.DBKhanzaUser + ":" + cfg.DBKhanzaPassword + "@tcp(" + cfg.DBKhanzaHost + ":" + cfg.DBKhanzaPort + ")/" + cfg.DBKhanzaName
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	log.Println("Successfully connected to Khanza DB")
	return db, nil
}

func GetPendingWorklist(db *sql.DB, tglPermintaan string) ([]WorklistRequest, error) {
	query := `SELECT
    p.no_rkm_medis AS PatientID,
    p.nm_pasien AS PatientName,
    DATE_FORMAT(p.tgl_lahir, '%Y%m%d') AS PatientBirthDate,
    CASE p.jk
        WHEN 'L' THEN 'M'
        WHEN 'P' THEN 'F'
        ELSE 'O'
    END AS PatientSex,
    pr.no_rawat AS AccessionNumber,
    pj.kd_jenis_prw AS RequestedProcedureID,
    jpr.nm_perawatan AS RequestedProcedureDescription,
    CONCAT(pr.no_rawat, DATE_FORMAT(pr.tgl_permintaan, '%Y%m%d'), REPLACE(pr.jam_permintaan, ':', '')) AS ScheduledProcedureStepID,
    DATE_FORMAT(pr.tgl_permintaan, '%Y%m%d') AS ScheduledProcedureStepStartDate,
    REPLACE(pr.jam_permintaan, ':', '') AS ScheduledProcedureStepStartTime,
    CASE
        WHEN jpr.nm_perawatan LIKE '%Rontgen%' THEN 'CR'
        WHEN jpr.nm_perawatan LIKE '%USG%' THEN 'US'
        WHEN jpr.nm_perawatan LIKE '%CT%' THEN 'CT'
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
	return requests, nil
}

func UpdateWorklistStatus(db *sql.DB, id int, status string) error {
	_, err := db.Exec("UPDATE permintaan_radiologi SET status=? WHERE id=?", status, id)
	return err
}

func SaveStudyLinkToKhanza(db *sql.DB, noRawat string, link string) error {
	_, err := db.Exec("UPDATE permintaan_radiologi SET link_hasil=? WHERE no_rawat=?", link, noRawat)
	return err
}

func SaveRadiologyResult(db *sql.DB, noRawat, tglPeriksa, jam, hasil, petugas string) error {
	_, err := db.Exec(
		"INSERT INTO hasil_radiologi (no_rawat, tgl_periksa, jam, hasil, nip) VALUES (?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE hasil=?, nip=?",
		noRawat, tglPeriksa, jam, hasil, petugas, hasil, petugas,
	)
	return err
}
