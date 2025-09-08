## Go Notes Application

A simple web application built with the Go standard library. This project was created as a personal exercise to practice backend development skills, including handling HTTP requests, serving HTML templates, and managing data.

### Features
- Create & View Notes: A simple interface to add new notes and view a list of all existing notes.
- Edit & Delete Notes: Seamless editing of notes and deletion.
- Note Previews: The homepage displays a condensed preview of each note for quick scanning.
- Labels/Tags: Organize notes with comma-separated labels.
- Clean UI: Styled with CSS for a clean, modern card-based layout.

### Running Locally
Prerequisites: Go, Docker

Instructions:
1. Clone the repository: `git clone https://github.com/adammcgrogan/go-notes.git`, `cd go-notes`.
2. Set up the database: `docker-compose up -d`
3. Run the application: `go run .`
4. Open `http://localhost:8080` in your browser.
