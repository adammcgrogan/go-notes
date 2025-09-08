package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // The pgx driver
	"github.com/lib/pq"                // For handling Postgres arrays
)

// The Post struct is updated for the database schema.
type Post struct {
	ID      int
	Title   string
	Slug    string
	Date    time.Time
	Content template.HTML
	Labels  pq.StringArray
}

type PageData struct {
	Title string
	Posts []*Post
}

type PostPageData struct {
	Title string
	Post  *Post
}

var (
	db           *sql.DB
	homeTemplate *template.Template
	newTemplate  *template.Template
	postTemplate *template.Template
)

func init() {
	// --- DATABASE CONNECTION ---
	connStr := "host=localhost port=5432 user=myuser password=mypassword dbname=notes_db sslmode=disable"
	var err error
	db, err = sql.Open("pgx", connStr)
	if err != nil {
		log.Fatal(err)
	}
	if err = db.Ping(); err != nil {
		log.Fatal("Could not ping database:", err)
	}

	// --- CREATE TABLE IF IT DOESN'T EXIST ---
	createTableSQL := `
    CREATE TABLE IF NOT EXISTS notes (
        id SERIAL PRIMARY KEY,
        title TEXT NOT NULL,
        slug TEXT UNIQUE NOT NULL,
        content TEXT,
        date TIMESTAMPTZ DEFAULT NOW(),
        labels TEXT[]
    );`
	if _, err = db.Exec(createTableSQL); err != nil {
		log.Fatalf("Error creating notes table: %v", err)
	}
	log.Println("Database connection successful and table checked.")

	// --- TEMPLATE PARSING ---
	funcMap := template.FuncMap{"truncate": truncate}
	homeTemplate = template.Must(template.New("home").Funcs(funcMap).ParseFiles("templates/base.html", "templates/index.html"))
	newTemplate = template.Must(template.New("new").Funcs(funcMap).ParseFiles("templates/base.html", "templates/new.html"))
	postTemplate = template.Must(template.New("post").Funcs(funcMap).ParseFiles("templates/base.html", "templates/note.html"))
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	// Query the database to get all notes
	rows, err := db.Query("SELECT id, title, slug, date, content, labels FROM notes ORDER BY date DESC")
	if err != nil {
		http.Error(w, "Database query error", http.StatusInternalServerError)
		log.Println(err)
		return
	}
	defer rows.Close()

	var posts []*Post
	for rows.Next() {
		var p Post
		// Scan the row data into the Post struct fields
		if err := rows.Scan(&p.ID, &p.Title, &p.Slug, &p.Date, &p.Content, &p.Labels); err != nil {
			http.Error(w, "Database scan error", http.StatusInternalServerError)
			log.Println(err)
			return
		}
		posts = append(posts, &p)
	}

	data := PageData{Title: "My Go Notes", Posts: posts}
	homeTemplate.ExecuteTemplate(w, "base.html", data)
}

func newNoteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		newTemplate.ExecuteTemplate(w, "base.html", &PageData{Title: "Create a New Note"})
		return
	}
	if r.Method == http.MethodPost {
		r.ParseForm()
		title := r.FormValue("title")
		content := r.FormValue("content")
		labelsStr := r.FormValue("labels")
		var labels []string
		rawLabels := strings.Split(labelsStr, ",")
		for _, label := range rawLabels {
			trimmed := strings.TrimSpace(label)
			if trimmed != "" {
				labels = append(labels, trimmed)
			}
		}

		slug := fmt.Sprintf("note-%d", time.Now().UnixNano())
		// Insert the new note into the database
		_, err := db.Exec("INSERT INTO notes (title, slug, content, labels) VALUES ($1, $2, $3, $4)",
			title, slug, content, pq.Array(labels))
		if err != nil {
			http.Error(w, "Failed to create note", http.StatusInternalServerError)
			log.Println(err)
			return
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func postHandler(w http.ResponseWriter, r *http.Request) {
	slug := r.URL.Path[len("/note/"):]
	var p Post
	// Query for a single row
	row := db.QueryRow("SELECT id, title, slug, date, content, labels FROM notes WHERE slug = $1", slug)
	// Scan the row into the Post struct
	err := row.Scan(&p.ID, &p.Title, &p.Slug, &p.Date, &p.Content, &p.Labels)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	} else if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	data := PostPageData{Title: p.Title, Post: &p}
	postTemplate.ExecuteTemplate(w, "base.html", data)
}

func main() {
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/note/", postHandler)
	http.HandleFunc("/new", newNoteHandler)
	log.Println("Server starting on http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

// truncate function remains the same
func truncate(content template.HTML) string {
	plainText := string(content)
	firstNewLine := strings.Index(plainText, "\n")
	if firstNewLine != -1 {
		return plainText[:firstNewLine] + "..."
	}
	if len(plainText) > 100 {
		return plainText[:100] + "..."
	}
	return plainText
}
