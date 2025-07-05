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
	KdJenisPrw                        string
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
    LEFT(CONCAT(pr.noorder, DATE_FORMAT(pr.tgl_permintaan, '%Y%m%d'), REPLACE(pr.jam_permintaan, ':', '')), 16) AS ScheduledProcedureStepID,
    DATE_FORMAT(pr.tgl_permintaan, '%Y%m%d') AS ScheduledProcedureStepStartDate,
    REPLACE(pr.jam_permintaan, ':', '') AS ScheduledProcedureStepStartTime,
    CASE
    -- CT Scan
    WHEN UPPER(nm_perawatan) LIKE '%CT SCAN%' OR UPPER(nm_perawatan) LIKE 'CT%' THEN 'CT'
    
    -- USG/Ultrasound
    WHEN UPPER(nm_perawatan) LIKE '%USG%' OR UPPER(nm_perawatan) LIKE 'USG%' OR UPPER(nm_perawatan) LIKE '%ULTRASOUND%' THEN 'US'
    
    -- MRI
    WHEN UPPER(nm_perawatan) LIKE '%MRI%' OR UPPER(nm_perawatan) LIKE 'MR%' OR UPPER(nm_perawatan) LIKE '%MAGNETIC RESONANCE%' THEN 'MR'
    
    -- Mammografi
    WHEN UPPER(nm_perawatan) LIKE '%MAMMO%' OR UPPER(nm_perawatan) LIKE '%MAMMOGRAFI%' THEN 'MG'
    
    -- Angiografi
    WHEN UPPER(nm_perawatan) LIKE '%ANGIO%' OR UPPER(nm_perawatan) LIKE '%XA%' OR UPPER(nm_perawatan) LIKE '%ANGIOGRAPHY%' THEN 'XA'
    
    -- PET Scan
    WHEN UPPER(nm_perawatan) LIKE '%PET%' OR UPPER(nm_perawatan) LIKE '%POSITRON%' THEN 'PT'
    
    -- Elektrokardiogram
    WHEN UPPER(nm_perawatan) LIKE '%EKG%' OR UPPER(nm_perawatan) LIKE '%ECG%' OR UPPER(nm_perawatan) LIKE '%ELEKTROKARDIOGRAM%' THEN 'ECG'
    
    -- EPS (Electrophysiology)
    WHEN UPPER(nm_perawatan) LIKE '%EPS%' OR UPPER(nm_perawatan) LIKE '%ELECTROPHYSIOLOGY%' THEN 'EPS'
    
    -- Endoscopy
    WHEN UPPER(nm_perawatan) LIKE '%ENDOSCOPY%' OR UPPER(nm_perawatan) LIKE '%ENDOSKOPI%' OR UPPER(nm_perawatan) LIKE '%ES%' THEN 'ES'
    
    -- Nuklir
    WHEN UPPER(nm_perawatan) LIKE '%NUKLIR%' OR UPPER(nm_perawatan) LIKE '%NUCLEAR%' OR UPPER(nm_perawatan) LIKE '%NM%' THEN 'NM'
    
    -- Structured Report
    WHEN UPPER(nm_perawatan) LIKE '%SR%' OR UPPER(nm_perawatan) LIKE '%STRUCTURED%' THEN 'SR'
    
    -- Secondary Capture
    WHEN UPPER(nm_perawatan) LIKE '%SC%' OR UPPER(nm_perawatan) LIKE '%SECONDARY CAPTURE%' THEN 'SC'
    
    -- X-Ray Cine
    WHEN UPPER(nm_perawatan) LIKE '%XC%' OR UPPER(nm_perawatan) LIKE '%CINE%' THEN 'XC'
    
    -- BabyGram (X-ray seluruh bayi)
    WHEN UPPER(nm_perawatan) LIKE '%BABYGRAM%' THEN 'CR'
    
    -- Pemeriksaan yang sudah jelas X-Ray/CR (umum di Indonesia)
    WHEN UPPER(nm_perawatan) LIKE '%THORAX%' THEN 'CR'
    WHEN UPPER(nm_perawatan) LIKE '%LUMBOSACRAL%' THEN 'CR'
    WHEN UPPER(nm_perawatan) LIKE '%VERTEBRA%' THEN 'CR'
    WHEN UPPER(nm_perawatan) LIKE '%PELVIS%' THEN 'CR'
    WHEN UPPER(nm_perawatan) LIKE '%FEMUR%' THEN 'CR'
    WHEN UPPER(nm_perawatan) LIKE '%HIP JOINT%' THEN 'CR'
    WHEN UPPER(nm_perawatan) LIKE '%HUMERUS%' THEN 'CR'
    WHEN UPPER(nm_perawatan) LIKE '%ANKLE%' THEN 'CR'
    WHEN UPPER(nm_perawatan) LIKE '%WRIST%' THEN 'CR'
    WHEN UPPER(nm_perawatan) LIKE '%MANUS%' THEN 'CR'
    WHEN UPPER(nm_perawatan) LIKE '%SCAPULA%' THEN 'CR'
    WHEN UPPER(nm_perawatan) LIKE '%CLAVICULA%' THEN 'CR'
    WHEN UPPER(nm_perawatan) LIKE '%CRANIUM%' THEN 'CR'
    WHEN UPPER(nm_perawatan) LIKE '%NASAL%' THEN 'CR'
    WHEN UPPER(nm_perawatan) LIKE '%GENU%' THEN 'CR'
    WHEN UPPER(nm_perawatan) LIKE '%CALCANEUS%' THEN 'CR'
    WHEN UPPER(nm_perawatan) LIKE '%ART GENU%' THEN 'CR'
    WHEN UPPER(nm_perawatan) LIKE '%CRURIS%' THEN 'CR'
    WHEN UPPER(nm_perawatan) LIKE '%ELBOW%' THEN 'CR'
    WHEN UPPER(nm_perawatan) LIKE '%ANTEBRACHI%' THEN 'CR'
    WHEN UPPER(nm_perawatan) LIKE '%BABYGRAM%' THEN 'CR'
    WHEN UPPER(nm_perawatan) LIKE '%BNO%' THEN 'CR'
    WHEN UPPER(nm_perawatan) LIKE '%APPENDICOGRAM%' THEN 'CR'
    WHEN UPPER(nm_perawatan) LIKE '%SACRUM%' THEN 'CR'
    WHEN UPPER(nm_perawatan) LIKE '%COCCYGEUS%' THEN 'CR'
    WHEN UPPER(nm_perawatan) LIKE '%ABDOMEN%' THEN 'CR'
    WHEN UPPER(nm_perawatan) LIKE '%PEDIS%' THEN 'CR'
    WHEN UPPER(nm_perawatan) LIKE '%SCAPULA%' THEN 'CR'
    WHEN UPPER(nm_perawatan) LIKE '%SHOULDER%' THEN 'CR'
    WHEN UPPER(nm_perawatan) LIKE '%GENU%' THEN 'CR'
    WHEN UPPER(nm_perawatan) LIKE '%SURVEY%' THEN 'CR'
    WHEN UPPER(nm_perawatan) LIKE '%CHARGE%' THEN 'CR'
    WHEN UPPER(nm_perawatan) LIKE '%PRINT FILM%' THEN 'CR'
    WHEN UPPER(nm_perawatan) LIKE '%TOP LORDOTIK%' THEN 'CR'
    WHEN UPPER(nm_perawatan) LIKE '%HSG%' OR UPPER(nm_perawatan) LIKE '%CHARGER CHATETER HSG%' THEN 'CR'
    -- Default mapping jika tidak terdeteksi
    ELSE 'CR'
    END AS Modality,
	pj.kd_jenis_prw as KdJenisPrw
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
			&req.KdJenisPrw,
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

func InsertPermintaanPemeriksaanRadiologi(db *sql.DB, noorder, kdJenisPrw, status string) error {
	// Cek apakah sudah ada data dengan noorder dan kd_jenis_prw yang sama
	var count int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM permintaan_pemeriksaan_radiologi WHERE noorder = ? AND kd_jenis_prw = ?`,
		noorder, kdJenisPrw,
	).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		// Sudah ada, tidak perlu insert lagi
		return nil
	}

	// Insert data baru
	_, err = db.Exec(
		`INSERT INTO permintaan_pemeriksaan_radiologi (noorder, kd_jenis_prw, status) VALUES (?, ?, ?)`,
		noorder, kdJenisPrw, status,
	)

	return err
}

func InsertPeriksaRadiologiFromPermintaan(db *sql.DB, noorder string) error {
	// Ambil data dari relasi tabel yang diperlukan
	var (
		noRawat, tglPeriksa, jam, kdDokter, nipPetugas, kdJenisPrw, hasil, biaya, status, kdBagian, nipPerujuk, kdPenjab string
	)
	query := `
SELECT
    pr.no_rawat,
    pr.tgl_permintaan,
    pr.jam_permintaan,
    pr.kd_dokter_perujuk,
    pr.nip_petugas,
    pj.kd_jenis_prw,
    IFNULL(hr.hasil, '') AS hasil,
    IFNULL(jpr.total_byr, 0) AS biaya,
    pr.status,
    IFNULL(pr.kd_bagian_radiologi, '') AS kd_bagian,
    IFNULL(pr.nip_perujuk, '') AS nip_perujuk,
    IFNULL(r.kd_pj, '') AS kd_penjab
FROM permintaan_radiologi pr
LEFT JOIN permintaan_pemeriksaan_radiologi pj ON pj.noorder = pr.noorder
LEFT JOIN hasil_radiologi hr ON hr.no_rawat = pr.no_rawat
LEFT JOIN jns_perawatan_radiologi jpr ON pj.kd_jenis_prw = jpr.kd_jenis_prw
LEFT JOIN reg_periksa r ON pr.no_rawat = r.no_rawat
WHERE pr.noorder = ?
LIMIT 1
`
	err := db.QueryRow(query, noorder).Scan(
		&noRawat, &tglPeriksa, &jam, &kdDokter, &nipPetugas, &kdJenisPrw, &hasil, &biaya, &status, &kdBagian, &nipPerujuk, &kdPenjab,
	)
	if err != nil {
		return err
	}

	// Insert ke tabel periksa_radiologi
	_, err = db.Exec(`
INSERT INTO periksa_radiologi (
    no_rawat, tgl_periksa, jam, kd_dokter, nip, kd_jenis_prw, hasil, biaya, status, kd_bagian_radiologi, nip_perujuk, kd_pj
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`, noRawat, tglPeriksa, jam, kdDokter, nipPetugas, kdJenisPrw, hasil, biaya, status, kdBagian, nipPerujuk, kdPenjab)
	return err
}
