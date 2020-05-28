package main

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/websocket"

	_ "github.com/go-sql-driver/mysql"
)

var cookieHandler = securecookie.New(
	securecookie.GenerateRandomKey(64),
	securecookie.GenerateRandomKey(32))

type User struct {
	Name     string
	Messages string
}

var (
	map1 = make(map[string]*websocket.Conn)
	map2 = make(map[*websocket.Conn]string)
)
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/", beg)
	r.HandleFunc("/signup", signupUser).Methods("POST")
	r.HandleFunc("/signup", signup).Methods("GET")
	r.HandleFunc("/login", loginUser).Methods("POST")
	r.HandleFunc("/login", login).Methods("GET")
	r.HandleFunc("/chat", chatBox)
	r.HandleFunc("/ws", wsHandler)
	r.HandleFunc("/logout", logout)
	http.ListenAndServe(":80", r)
}

func signup(w http.ResponseWriter, r *http.Request) {
	msg := ""
	t, _ := template.ParseFiles("signup.html")
	t.Execute(w, struct {
		Alert   bool
		Pushmsg string
	}{false, msg})
}

func login(w http.ResponseWriter, r *http.Request) {
	username := GetUserName(r)
	if username == "" {
		msg := ""
		t, _ := template.ParseFiles("login.html")
		t.Execute(w, struct {
			Alert   bool
			Pushmsg string
		}{false, msg})
	} else {
		http.Redirect(w, r, "/chat", http.StatusFound)
	}
}

func signupUser(w http.ResponseWriter, r *http.Request) {

	db, err := sql.Open("mysql", "khyati:ascii123@(127.0.0.1:3306)/mysql?parseTime=true")
	if err != nil {
		fmt.Println(err)
		fmt.Printf("%T", db)
		os.Exit(1)
	}
	username := r.FormValue("username")
	password := r.FormValue("password")
	createdAt := time.Now()
	var pass string
	e := db.QueryRow(`SELECT password from users WHERE username = ?`, username).Scan(&pass)
	if e != nil {
		result, err := db.Exec(`INSERT INTO users (username,password,created_at) VALUES (?,?,?)`, username, password, createdAt)
		if err != nil {
			log.Fatal(err)
		}
		id, err := result.LastInsertId()
		fmt.Println(id)
		msg := ""
		t, _ := template.ParseFiles("login.html")
		t.Execute(w, struct {
			Alert   bool
			Pushmsg string
		}{false, msg})
	} else {
		msg := "<script>alert('Username already exists')</script>"
		t, _ := template.ParseFiles("signup.html")
		t.Execute(w, struct {
			Alert   bool
			Pushmsg string
		}{true, msg})
	}

}

func loginUser(w http.ResponseWriter, r *http.Request) {

	db, err := sql.Open("mysql", "khyati:ascii123@(127.0.0.1:3306)/mysql?parseTime=true")
	if err != nil {
		fmt.Println(err)
		fmt.Printf("%T", db)
		os.Exit(1)
	}
	username := GetUserName(r)
	if username == "" {
		var pass string
		uname := r.FormValue("username")
		pswd := r.FormValue("password")
		err := db.QueryRow(`SELECT password from users WHERE username = ?`, uname).Scan(&pass)
		if err != nil || pass != pswd {
			fmt.Println(err)
			msg := "<script>alert('Invalid Credentials')</script>"
			t, _ := template.ParseFiles("login.html")
			t.Execute(w, struct {
				Alert   bool
				Pushmsg string
			}{true, msg})
		}

		if pass == pswd {
			setCookie(uname, w)
			http.Redirect(w, r, "/chat", http.StatusFound)

		}
	} else {
		http.Redirect(w, r, "/chat", http.StatusFound)
	}
}

func chatBox(w http.ResponseWriter, r *http.Request) {
	uname := GetUserName(r)
	if uname != "" {
		t, _ := template.ParseFiles("chat.html")
		username := GetUserName(r)
		filename := "public/" + username + ".txt"
		dat, err := ioutil.ReadFile(filename)
		var data string
		if err != nil {
			fmt.Println("No File Exists")
			data = string("")
		} else {
			data = string(dat)
		}
		LoggedUser := User{
			Name:     username,
			Messages: data,
		}
		t.Execute(w, LoggedUser)
	} else {
		http.Redirect(w, r, "/login", http.StatusFound)
	}
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)

	if err != nil {
		fmt.Println("can't connect to the socket, ", err)
		return
	}
	username := GetUserName(r)
	map1[username] = conn
	map2[conn] = username

	go conversation(conn)

}

func logout(w http.ResponseWriter, r *http.Request) {

	ClearCookie(w)
	http.Redirect(w, r, "/login", http.StatusFound)
}

func beg(w http.ResponseWriter, r *http.Request) {
	username := GetUserName(r)
	if username != "" {
		http.Redirect(w, r, "/login", http.StatusFound)
	} else {
		http.Redirect(w, r, "/chat", http.StatusFound)
	}
}

func conversation(conn *websocket.Conn) {
	for {
		msgType, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		arr := strings.Split(string(msg), " ")
		tosend := map1[arr[0]]
		if arr[0] == map2[conn] {
			msgToSend := "You can't message yourself"
			if err = conn.WriteMessage(msgType, []byte(msgToSend)); err != nil {
				return
			}
		} else if tosend != nil {

			filename := "public/" + map2[conn] + ".txt"
			msg := "You : " + strings.Join(arr[1:], " ") + " to " + arr[0] + "\n"
			writeToFile(filename, msg)
			senderFile := "public/" + arr[0] + ".txt"
			msgToSend := map2[conn] + ": " + strings.Join(arr[1:], " ") + "\n"
			writeToFile(senderFile, msgToSend)
			if err = tosend.WriteMessage(msgType, []byte(msgToSend)); err != nil {
				return
			}
		} else {
			msgToSend := "No such user"
			if err = conn.WriteMessage(msgType, []byte(msgToSend)); err != nil {
				return
			}
		}
	}
}

func writeToFile(filename string, msg string) {

	_, err := os.Stat(filename) // Truncates if file already exists, be careful!
	if err != nil {
		if os.IsNotExist(err) {
			newfile, _ := os.Create(filename)
			fmt.Println(newfile)
		} else {
			return
		}
	}
	file, _ := os.OpenFile(filename, os.O_APPEND, 0644)

	defer file.Close() // Make sure to close the file when you're done

	if _, err := file.WriteString(msg); err != nil {
		log.Fatal(err)
	}
}

func setCookie(userName string, w http.ResponseWriter) {
	value := map[string]string{
		"name": userName,
	}
	if encoded, err := cookieHandler.Encode("cookie", value); err == nil {
		cookie := &http.Cookie{
			Name:  "cookie",
			Value: encoded,
			Path:  "/",
		}
		http.SetCookie(w, cookie)
	}

}

func ClearCookie(response http.ResponseWriter) {
	cookie := &http.Cookie{
		Name:   "cookie",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	}
	http.SetCookie(response, cookie)
}

func GetUserName(request *http.Request) (userName string) {
	if cookie, err := request.Cookie("cookie"); err == nil {
		cookieValue := make(map[string]string)
		if err = cookieHandler.Decode("cookie", cookie.Value, &cookieValue); err == nil {
			userName = cookieValue["name"]
		}
	}
	return userName
}
