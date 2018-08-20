// Zloy Rabadaber
// zrabadaber@gmail.com

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"goji.io"
	"goji.io/pat"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

func responsePreflight(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Accept, Accept-Encoding, Destination, Content-Type, Content-Length")
	w.WriteHeader(http.StatusOK)
}

func errorWithJSON(w http.ResponseWriter, r *http.Request, message string, code int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
	w.WriteHeader(code)
	fmt.Fprintf(w, "{\"message\": %q}", message)
}

func responseWithJSON(w http.ResponseWriter, r *http.Request, json []byte, code int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
	w.WriteHeader(code)
	w.Write(json)
}

type userTold struct {
	ID         string `json:"id"`         //идентификатор
	Xp_amount  string `json:"xp_amount"`  //текущее значение опыта
	Xp_damount string `json:"xp_damount"` //до следующего уровня
	All_ok     string `json:"all_ok"`     //купили все
	Lvl_ok     string `json:"lvl_ok"`     //номер последнего пройденного уровня
}

type userT struct {
	ID         string `json:"id"`           //Идентификатор
	LvlOk      string `json:"lvl_ok"`       //Номер последнего пройденного уровня
	AllOk      string `json:"all_ok"`       //Купили все
	HintFstep  string `json:"hint_fstep"`   //Количество подсказок первого хода
	HintBack   string `json:"hint_back"`    //Количество подсказок отмены хода
	LiveCount  string `json:"live_count"`   //Количество жизней
	LiveTime   string `json:"live_time"`    //Время в секундах
	PriceTime  string `json:"price_time"`   //Время в секундах
	GameTime   string `json:"game_time"`    //Время в секундах
	GamePoints string `json:"game_points"`  //Очки игрока (для рейтинговой системы)
	GameLvlTry string `json:"game_lvl_try"` //Попытки прохождения уровня
	Sound      string `json:"sound"`        //Включен ли звук
	Music      string `json:"music"`        //Включена ли музыка
	Reserve1   string `json:"reserve_1"`
	Reserve2   string `json:"reserve_2"`
	Reserve3   string `json:"reserve_3"`
	Reserve4   string `json:"reserve_4"`
}

const userCollectionOld = "arrows_users"
const userCollection string = "users_arrows"

func main() {
	connString := fmt.Sprintf("mongodb://172.17.0.1:27017/simple")
	log.Println("connection string: " + connString)

	session, err := mgo.Dial(connString)
	if err != nil {
		panic(err)
	}
	defer session.Close()

	session.SetMode(mgo.Monotonic, true)

	migrate(session)

	ensureIndex(session)

	mux := goji.NewMux()
	mux.HandleFunc(pat.Options("/*"), preflight(session))

	mux.HandleFunc(pat.Get("/users"), allUsers(session))
	mux.HandleFunc(pat.Post("/users"), addUser(session))

	mux.HandleFunc(pat.Get("/users/:id"), userByID(session))
	mux.HandleFunc(pat.Put("/users/:id"), updateUser(session))
	mux.HandleFunc(pat.Delete("/users/:id"), deleteUser(session))

	mux.HandleFunc(pat.Get("/healthcheck"), test(session))

	log.Println("server started on port 3030")
	http.ListenAndServe("0.0.0.0:3030", mux)
}

func migrate(s *mgo.Session) {
	session := s.Copy()
	defer session.Close()

	cOld := session.DB("simple").C(userCollectionOld)

	var usersOld []userTold
	err := cOld.Find(bson.M{}).All(&usersOld)
	if err != nil {
		panic(err)
	}

	log.Println("found ", len(usersOld), " users to migrate")

	users := make([]userT, len(usersOld))
	for i, it := range usersOld {
		users[i].ID = it.ID
		users[i].LvlOk = it.Lvl_ok
		users[i].AllOk = ""
		users[i].HintFstep = ""
		users[i].HintBack = ""
		users[i].LiveCount = ""
		users[i].LiveTime = ""
		users[i].PriceTime = ""
		users[i].GameTime = ""
		users[i].GamePoints = ""
		users[i].GameLvlTry = ""
		users[i].Sound = ""
		users[i].Music = ""
		users[i].Reserve1 = ""
		users[i].Reserve2 = ""
		users[i].Reserve3 = ""
		users[i].Reserve4 = ""
	}

	c := session.DB("simple").C(userCollection)
	for _, it := range users {
		err := c.Insert(it)
		if err != nil {
			if mgo.IsDup(err) {
				log.Println("User with ID=" + it.ID + " already exists")
			} else {
				log.Println("Error insert user with ID=" + it.ID)
			}
		}
	}
}

func ensureIndex(s *mgo.Session) {
	session := s.Copy()
	defer session.Close()

	c := session.DB("simple").C(userCollection)

	index := mgo.Index{
		Key:        []string{"id"},
		Unique:     true,
		DropDups:   true,
		Background: true,
		Sparse:     true,
	}
	err := c.EnsureIndex(index)
	if err != nil {
		panic(err)
	}
}

func preflight(s *mgo.Session) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		responsePreflight(w, r)
	}
}

func allUsers(s *mgo.Session) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()

		c := session.DB("simple").C(userCollection)

		var users []userT
		err := c.Find(bson.M{}).All(&users)
		if err != nil {
			errorWithJSON(w, r, "Database error", http.StatusOK)
			log.Println("Failed get all users: ", err)
			return
		}

		respBody, err := json.MarshalIndent(users, "", "  ")
		if err != nil {
			log.Fatal(err)
		}

		responseWithJSON(w, r, respBody, http.StatusOK)
	}
}

func addUser(s *mgo.Session) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()

		var user userT
		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&user)
		if err != nil {
			errorWithJSON(w, r, "Incorrect body", http.StatusBadRequest)
			return
		}

		c := session.DB("simple").C(userCollection)

		err = c.Insert(user)
		if err != nil {
			if mgo.IsDup(err) {
				errorWithJSON(w, r, "User with this ID already exists", http.StatusOK)
				log.Println("Failed insert user: ", err)
				return
			}

			errorWithJSON(w, r, "Failed insert user", http.StatusOK)
			log.Println("Failed insert user: ", err)
			return
		}

		// Marshal provided interface into JSON structure
		respBody, _ := json.Marshal(user)
		w.Header().Set("Location", r.URL.Path+"/"+user.ID)
		responseWithJSON(w, r, respBody, http.StatusCreated)
	}
}

func userByID(s *mgo.Session) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()

		id := pat.Param(r, "id")

		c := session.DB("simple").C(userCollection)

		var user userT
		err := c.Find(bson.M{"id": id}).One(&user)
		if err != nil {
			errorWithJSON(w, r, "User not found", http.StatusOK)
			log.Println("Failed find user by ID: ", err)
			return
		}

		if user.ID == "" {
			errorWithJSON(w, r, "User not found", http.StatusOK)
			log.Println("Failed find user by ID")
			return
		}

		respBody, err := json.MarshalIndent(user, "", "  ")
		if err != nil {
			log.Fatal(err)
		}

		responseWithJSON(w, r, respBody, http.StatusOK)
	}
}

func updateUser(s *mgo.Session) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()

		id := pat.Param(r, "id")

		var user userT
		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&user)
		if err != nil {
			errorWithJSON(w, r, "Incorrect body", http.StatusBadRequest)
			return
		}

		c := session.DB("simple").C(userCollection)

		err = c.Update(bson.M{"id": id}, &user)
		if err != nil {
			switch err {
			default:
				errorWithJSON(w, r, "Failed update user", http.StatusOK)
				log.Println("Failed update user: ", err)
				return
			case mgo.ErrNotFound:
				errorWithJSON(w, r, "User not found", http.StatusOK)
				log.Println("Failed update user")
				return
			}
		}

		// Marshal provided interface into JSON structure
		respBody, _ := json.Marshal(user)
		w.Header().Set("Location", r.URL.Path+"/"+user.ID)
		responseWithJSON(w, r, respBody, http.StatusCreated)
	}
}

func deleteUser(s *mgo.Session) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()

		id := pat.Param(r, "id")

		c := session.DB("simple").C(userCollection)

		err := c.Remove(bson.M{"id": id})
		if err != nil {
			switch err {
			default:
				errorWithJSON(w, r, "Failed delete user", http.StatusOK)
				log.Println("Failed delete user: ", err)
				return
			case mgo.ErrNotFound:
				errorWithJSON(w, r, "User not found", http.StatusOK)
				log.Println("Failed delete user")
				return
			}
		}

		responseWithJSON(w, r, []byte("{\"message\":\"ok\"}"), http.StatusOK)
	}
}

func test(s *mgo.Session) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		responseWithJSON(w, r, []byte("{\"message\":\"passed\"}"), http.StatusOK)
	}
}
