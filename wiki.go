package main

import (
	"context"
	"errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"html/template"
	"log"
	"net/http"
	"regexp"
	"time"
)

var templates = template.Must(template.ParseFiles("edit.html", "view.html"))

var validPath = regexp.MustCompile("^/(edit|save|view)/([a-zA-Z0-9]+)$")

var svc = MustCreateService()

func getTitle(w http.ResponseWriter, r *http.Request) (string, error) {
	m := validPath.FindStringSubmatch(r.URL.Path)
	if m == nil {
		http.NotFound(w, r)
		return "", errors.New("invalid Page Title")
	}
	return m[2], nil // The title is the second subexpression.
}

type Page struct {
	Title   string
	Content []byte
}

func (p *Page) save() error {
	_, err := svc.Pages.DeleteMany(context.TODO(), bson.M{"title": p.Title})
	res, err := svc.Pages.InsertOne(context.TODO(), bson.M{
		"title": p.Title, "content": p.Content,
	})
	log.Print(res)
	return err
}

func loadPage(title string) (*Page, error) {
	var page Page
	err := svc.Pages.FindOne(context.TODO(), bson.M{"title": title}).Decode(&page)
	if err != nil {
		log.Print(err)
		return nil, err
	}
	log.Printf("Page found: title=%s, body=%s", page.Title, page.Content)
	return &page, nil
}

func renderTemplate(w http.ResponseWriter, tmpl string, p *Page) {
	err := templates.ExecuteTemplate(w, tmpl+".html", p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func viewHandler(w http.ResponseWriter, r *http.Request) {
	title, err := getTitle(w, r)
	if err != nil {
		return
	}
	p, err := loadPage(title)
	if err != nil {
		http.Redirect(w, r, "/edit/"+title, http.StatusFound)
		return
	}
	renderTemplate(w, "view", p)
}

func editHandler(w http.ResponseWriter, r *http.Request) {
	title, err := getTitle(w, r)
	if err != nil {
		return
	}
	p, err := loadPage(title)
	if err != nil {
		p = &Page{Title: title}
	}
	renderTemplate(w, "edit", p)
}

func saveHandler(w http.ResponseWriter, r *http.Request) {
	title, err := getTitle(w, r)
	if err != nil {
		return
	}
	body := r.FormValue("body")
	p := &Page{Title: title, Content: []byte(body)}
	err = p.save()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/view/"+title, http.StatusFound)
}

type Service struct {
	Pages *mongo.Collection
}

func MustCreateService() *Service {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	clientOptions := options.Client().ApplyURI(
		"mongodb://localhost:27017",
	).SetAuth(
		options.Credential{Username: "admin", Password: "secret"},
	)
	client, _ := mongo.Connect(ctx, clientOptions)

	err := client.Ping(ctx, readpref.Primary())
	if err != nil {
		panic(err)
	}

	collection := client.Database("kira").Collection("pages")
	log.Println("Connected to database")
	return &Service{collection}
}

func main() {
	http.HandleFunc("/view/", viewHandler)
	http.HandleFunc("/edit/", editHandler)
	http.HandleFunc("/save/", saveHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
