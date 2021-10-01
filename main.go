package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
)

var tpl *template.Template

func init() {
	tpl = template.Must(template.ParseGlob("./views/*.gohtml"))
}

type user struct {
	name string
}

func main() {
	http.HandleFunc("/login", loginHandle)
	http.Handle("/favicon.ico", http.NotFoundHandler())
	http.Handle("/views/", http.StripPrefix("/views/", http.FileServer(http.Dir("views"))))
	http.HandleFunc("/main/", mainHandle)
	http.ListenAndServe(":8080", nil)
}

func loginHandle(w http.ResponseWriter, r *http.Request) {
	//var u user
	if r.Method == http.MethodPost {
		un := r.FormValue("username")
		fmt.Println(un)
		http.Redirect(w, r, "/main", http.StatusSeeOther)
	}
	err := tpl.ExecuteTemplate(w, "login.gohtml", "Sam")
	if err != nil {
		log.Fatalln(err)
	}

}

func mainHandle(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		lout := r.FormValue("logout")
		if lout == "true" {
			fmt.Println("Logged out!")
			http.Redirect(w, r, "/login", http.StatusSeeOther)
		}
	} else if r.Method == http.MethodPost {
		err := r.ParseForm()
		if err != nil {
			log.Fatalln(err)
		}
		sButton := r.PostForm.Get("submitmsg")
		sMess := r.PostForm.Get("usermsg")

		if sButton == "Send" && sMess != "" {
			fmt.Println(sMess)
		}
	}
	err := tpl.ExecuteTemplate(w, "index.gohtml", "Sam")
	if err != nil {
		log.Fatalln(err)
	}
}
