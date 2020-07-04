package backend

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"log"
	"net/http"
	"os"

	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

type Backend struct {
	dbFile   string
	inetAddr string
	dbHandle *sql.DB
}

func InitBackend(dbFile string, inetAddr string) *Backend {
	return &Backend{
		dbFile:   dbFile,
		inetAddr: inetAddr,
	}
}

func (b *Backend) createSchema() {
	createSnapshotTableSQL := `
CREATE TABLE doorbell_snapshot (
"id" integer NOT NULL PRIMARY KEY AUTOINCREMENT,
"datetime" DATE DEFAULT (datetime('now')),
"photo" BLOB NOT NULL
);`

	log.Println("Creating doorbell_snapshot table")
	s, err := b.dbHandle.Prepare(createSnapshotTableSQL)
	if err != nil {
		log.Fatalln(err.Error())
	}
	s.Exec()
	log.Println("Table doorbell_snapshot created")

}

func (b *Backend) openDB() {
	newDB := false

	// check if the database exists otherwise create it
	if _, err := os.Stat(b.dbFile); os.IsNotExist(err) {
		f, err := os.Create(b.dbFile)
		if err != nil {
			log.Fatalln(err.Error())
		}
		f.Close()
		log.Println("Database created")
		newDB = true
	}

	// open the database
	log.Println("Open database")
	var err error
	b.dbHandle, err = sql.Open("sqlite3", b.dbFile)
	if err != nil {
		log.Fatalln(err.Error())
	}

	if newDB {
		b.createSchema()
	}
}

func (b *Backend) closeDB() {
	b.dbHandle.Close()
}

func (b *Backend) InsertSnapshot(image *image.Image) {
	// we permit at most N snapshot
	maxSnapshot := 100

	// delete snapshot over maxSnapshot
	q := `
DELETE from doorbell_snapshot WHERE id IN
(SELECT id FROM doorbell_snapshot ORDER BY id DESC LIMIT -1 OFFSET ?)
`
	log.Println("Delete old snapshot")
	s, err := b.dbHandle.Prepare(q)
	if err != nil {
		log.Println(err.Error())
	}
	_, err = s.Exec(maxSnapshot)
	if err != nil {
		log.Println(err.Error())
	}

	// insert new snapshot
	q = `INSERT INTO doorbell_snapshot(photo) VALUES (?)`
	log.Println("Insert new snapshot")
	s, err = b.dbHandle.Prepare(q)
	if err != nil {
		log.Println(err.Error())
	}

	buf := new(bytes.Buffer)
	err = jpeg.Encode(buf, *image, nil)
	if err != nil {
		fmt.Println("JPEG: failed to create buffer", err)
		return
	}
	_, err = s.Exec(buf.Bytes())
	if err != nil {
		log.Println(err.Error())
	}
}

// thanks https://stackoverflow.com/questions/19991541/dumping-mysql-tables-to-json-with-golang
func (b *Backend) getJSON(sqlString string) (string, error) {
	stmt, err := b.dbHandle.Prepare(sqlString)
	if err != nil {
		return "", err
	}
	defer stmt.Close()

	rows, err := stmt.Query()
	if err != nil {
		return "", err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return "", err
	}

	tableData := make([]map[string]interface{}, 0)

	count := len(columns)
	values := make([]interface{}, count)
	scanArgs := make([]interface{}, count)
	for i := range values {
		scanArgs[i] = &values[i]
	}

	for rows.Next() {
		err := rows.Scan(scanArgs...)
		if err != nil {
			return "", err
		}

		entry := make(map[string]interface{})
		for i, col := range columns {
			v := values[i]

			b, ok := v.([]byte)
			if ok {
				// bytes are encoded in base64
				entry[col] = base64.StdEncoding.EncodeToString(b)
			} else {
				entry[col] = v
			}
		}

		tableData = append(tableData, entry)
	}

	jsonData, err := json.Marshal(tableData)
	if err != nil {
		return "", err
	}

	return string(jsonData), nil
}

func (b *Backend) getSnapshots(w http.ResponseWriter, r *http.Request) {
	log.Println("WebService: getSnapshots requested")
	json, err := b.getJSON("SELECT * from doorbell_snapshot")
	if err != nil {
		log.Println(err.Error())
	}
	fmt.Fprintf(w, json)
}

func (b *Backend) getHome(w http.ResponseWriter, r *http.Request) {

	log.Println("WebService: getHome requested")

	stmt, err := b.dbHandle.Prepare("SELECT datetime, photo FROM doorbell_snapshot ORDER BY id DESC")
	if err != nil {
		fmt.Fprintf(w, err.Error())
	}
	defer stmt.Close()

	rows, err := stmt.Query()
	if err != nil {
		fmt.Fprintf(w, err.Error())
	}
	defer rows.Close()

	homepage := `
<html>
<head>
<title>Backend</title>
<style>
th, td {
  padding: 15px;
  border-spacing: 5px;
  text-align: center;
}
</style>
</head>
<body>
<div id="printSnap">
<table style="width:800;margin-left:auto;margin-right:auto;">
<tr>
<th>Date and Time</th>
<th>Snapshot</th>
</tr>
`
	for rows.Next() {
		var (
			datetime string
			photo    []byte
		)
		if err := rows.Scan(&datetime, &photo); err != nil {
			log.Fatal(err)
		}
		homepage += fmt.Sprintf(
			"<tr><td><script type='text/javascript'>document.write(new Date('%s'))</script></td><td><img src='data:image/jpeg;base64,%s' alt=snapshot width=300 /></td></tr>",
			datetime,
			base64.StdEncoding.EncodeToString(photo))
	}

	homepage += `
<table>
</div>
</body>
</html>
`
	fmt.Fprintf(w, homepage)
}

func (b *Backend) StartWebService() {

	b.openDB()

	http.HandleFunc("/", b.getHome)
	http.HandleFunc("/getSnapshots", b.getSnapshots)

	log.Println("Backend is listening at " + b.inetAddr)
	log.Fatalln(http.ListenAndServe(b.inetAddr, nil))
}

func (b *Backend) StopWebService() {
	b.closeDB()
}
