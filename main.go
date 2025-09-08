package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/lib/pq"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/renderer/html" // Import the html renderer for options
)

// The PostPageData struct is updated to track the editing state.
type PostPageData struct {
	Title     string
	Post      *Post
	IsEditing bool // This new field controls the template
}

// Other structs are unchanged
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

// Global variables are simplified (editTemplate is removed)
var (
	db           *sql.DB
	homeTemplate *template.Template
	newTemplate  *template.Template
	postTemplate *template.Template
)

// Configure Goldmark to treat single newlines as hard line breaks (<br>).
var md = goldmark.New(
	goldmark.WithRendererOptions(
		html.WithHardWraps(),
	),
)

// Template functions are unchanged
func markdownify(content template.HTML) template.HTML {
	var buf strings.Builder
	if err := md.Convert([]byte(string(content)), &buf); err != nil {
		return content
	}
	return template.HTML(buf.String())
}
func truncate(content template.HTML) string {
	plainText := string(content)
	if len(plainText) > 100 {
		return plainText[:100] + "..."
	}
	return plainText
}
func join(s pq.StringArray, sep string) string {
	return strings.Join(s, sep)
}

func init() {
	connStr := "host=localhost port=5432 user=myuser password=mypassword dbname=notes_db sslmode=disable"
	var err error
	db, err = sql.Open("pgx", connStr)
	if err != nil {
		log.Fatal(err)
	}
	if err = db.Ping(); err != nil {
		log.Fatal("Could not ping database:", err)
	}

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

	funcMap := template.FuncMap{
		"truncate":    truncate,
		"markdownify": markdownify,
		"join":        join,
	}

	homeTemplate = template.Must(template.New("home").Funcs(funcMap).ParseFiles("templates/base.html", "templates/index.html"))
	newTemplate = template.Must(template.New("new").Funcs(funcMap).ParseFiles("templates/base.html", "templates/new.html"))
	postTemplate = template.Must(template.New("post").Funcs(funcMap).ParseFiles("templates/base.html", "templates/note.html"))
}

// homeHandler and newNoteHandler are unchanged
func homeHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, title, slug, date, content, labels FROM notes ORDER BY date DESC")
	if err != nil {
		http.Error(w, "Database query error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var posts []*Post
	for rows.Next() {
		var p Post
		if err := rows.Scan(&p.ID, &p.Title, &p.Slug, &p.Date, &p.Content, &p.Labels); err != nil {
			http.Error(w, "Database scan error", http.StatusInternalServerError)
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
		_, err := db.Exec("INSERT INTO notes (title, slug, content, labels) VALUES ($1, $2, $3, $4)",
			title, slug, content, pq.Array(labels))
		if err != nil {
			http.Error(w, "Failed to create note", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

// The postHandler now handles viewing, editing, and updating.
func postHandler(w http.ResponseWriter, r *http.Request) {
	slug := r.URL.Path[len("/note/"):]

	// Handle form submission for UPDATING a note
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

		_, err := db.Exec("UPDATE notes SET title = $1, content = $2, labels = $3 WHERE slug = $4",
			title, content, pq.Array(labels), slug)
		if err != nil {
			http.Error(w, "Failed to update note", http.StatusInternalServerError)
			log.Println(err)
			return
		}
		// Redirect back to the clean URL (without "?edit=true") to show the view mode.
		http.Redirect(w, r, "/note/"+slug, http.StatusSeeOther)
		return
	}

	// Handle VIEWING or showing the EDIT form
	var p Post
	row := db.QueryRow("SELECT id, title, slug, date, content, labels FROM notes WHERE slug = $1", slug)
	err := row.Scan(&p.ID, &p.Title, &p.Slug, &p.Date, &p.Content, &p.Labels)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	} else if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Check for the "?edit=true" query parameter.
	isEditing := r.URL.Query().Get("edit") == "true"

	data := PostPageData{
		Title:     p.Title,
		Post:      &p,
		IsEditing: isEditing,
	}
	postTemplate.ExecuteTemplate(w, "base.html", data)
}

// deleteNoteHandler is unchanged
func deleteNoteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	slug := r.URL.Path[len("/note/delete/"):]
	if slug == "" {
		http.Error(w, "Missing note identifier", http.StatusBadRequest)
		return
	}
	_, err := db.Exec("DELETE FROM notes WHERE slug = $1", slug)
	if err != nil {
		http.Error(w, "Failed to delete note", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// main function is simplified and corrected.
func main() {
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/new", newNoteHandler)
	http.HandleFunc("/note/delete/", deleteNoteHandler)
	// The postHandler now handles all /note/ requests (view, edit, update).
	http.HandleFunc("/note/", postHandler)

	log.Println("Server starting on http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
