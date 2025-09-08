package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"
)

/*
The structs define the data structures for our application.
PostPageData is new and holds the data needed for the single post view.
*/
type Post struct {
	Title   string
	Slug    string
	Date    time.Time
	Content template.HTML
}

type PageData struct {
	Title string
	Posts []*Post
}

type PostPageData struct {
	Title string
	Post  *Post
}

/*
The 'posts' slice acts as our simple in-memory database.
*/
var posts = []*Post{
	{
		Title:   "My First Notes Post",
		Slug:    "hello-world",
		Date:    time.Date(2025, 9, 8, 0, 0, 0, 0, time.UTC),
		Content: "Welcome to my notes! This is a simple note to get you started.\n\nThis is the second line, which you can now see on the full post page.",
	},
}

/*
We add a 'postTemplate' variable to hold our new single-post page template.
*/
var (
	homeTemplate *template.Template
	newTemplate  *template.Template
	postTemplate *template.Template // Add this line
)

/*
The init function now also parses the 'post.html' template.
*/
func init() {
	funcMap := template.FuncMap{
		"truncate": truncate,
	}
	homeTemplate = template.Must(template.New("home").Funcs(funcMap).ParseFiles("templates/base.html", "templates/index.html"))
	newTemplate = template.Must(template.New("new").Funcs(funcMap).ParseFiles("templates/base.html", "templates/new.html"))
	postTemplate = template.Must(template.New("post").Funcs(funcMap).ParseFiles("templates/base.html", "templates/note.html"))
}

/*
homeHandler and newNoteHandler remain the same.
*/
func homeHandler(w http.ResponseWriter, r *http.Request) {
	data := PageData{
		Title: "My Go Notes",
		Posts: posts,
	}
	err := homeTemplate.ExecuteTemplate(w, "base.html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println(err)
	}
}

func newNoteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		err := newTemplate.ExecuteTemplate(w, "base.html", &PageData{Title: "Create a New Note"})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Println(err)
		}
		return
	}
	if r.Method == http.MethodPost {
		r.ParseForm()
		title := r.FormValue("title")
		content := r.FormValue("content")
		slug := fmt.Sprintf("note-%d", time.Now().Unix())
		newPost := &Post{
			Title:   title,
			Slug:    slug,
			Date:    time.Now(),
			Content: template.HTML(content),
		}
		posts = append([]*Post{newPost}, posts...)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

/*
The postHandler is now fully implemented.
It finds a post by its slug and renders the post page. If not found, it shows a 404 error.
*/
func postHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Get the slug from the URL.
	slug := r.URL.Path[len("/note/"):]

	// 2. Search the 'posts' slice for a matching slug.
	var foundPost *Post
	for _, p := range posts {
		if p.Slug == slug {
			foundPost = p
			break
		}
	}

	// 3. If no post is found, return a 404 Not Found error.
	if foundPost == nil {
		http.NotFound(w, r)
		return
	}

	// 4. If a post is found, prepare the data and render the template.
	data := PostPageData{
		Title: foundPost.Title,
		Post:  foundPost,
	}
	err := postTemplate.ExecuteTemplate(w, "base.html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println(err)
	}
}

/*
The truncate function remains the same.
*/
func truncate(content template.HTML) string {
	plainText := string(content)
	firstNewLine := strings.Index(plainText, "\n")
	if firstNewLine != -1 {
		return plainText[:firstNewLine] + "..."
	}
	return plainText
}

/*
The main function now includes the updated route for /note/.
*/
func main() {
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/note/", postHandler) // Updated route from previous step
	http.HandleFunc("/new", newNoteHandler)

	log.Println("Server starting on http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
