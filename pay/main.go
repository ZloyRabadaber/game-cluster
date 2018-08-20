package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	//"bytes"
	"bytes"
	"github.com/gorilla/mux"
	"github.com/night-codes/mgo-ai"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"io/ioutil"
	"strconv"
	"strings"
)

type User struct {
	Id         string `json:"id"`         //идентификатор
	Xp_amount  string `json:"xp_amount"`  //текущее значение опыта
	Xp_damount string `json:"xp_damount"` //до следующего уровня
	All_ok     string `json:"all_ok"`     //купили все
	Lvl_ok     string `json:"lvl_ok"`     //номер последнего пройденного уровня
}

type Item struct {
	App_id    int    `json:"app_id"`
	Item      string `json:"item"`
	Title     string `json:"title"`
	Photo_url string `json:"photo_url"`
	Price     int    `json:"price"`
	Item_id   string `json:"item_id"`
}

type ItemResp struct {
	Title      string `json:"title"`
	Photo_url  string `json:"photo_url"`
	Price      int    `json:"price"`
	Item_id    string `json:"item_id"`
	Expiration int    `json:"expiration"`
}

type Order struct {
	App_order_id   int    `json:"app_order_id"`
	App_id         int    `json:"app_id"`
	User_id        int    `json:"user_id"`
	Receiver_id    int    `json:"receiver_id"`
	Order_id       int    `json:"order_id"`
	Date           int    `json:"date"`
	Status         string `json:"status"`
	Item           string `json:"item"`
	Item_id        string `json:"item_id"`
	Item_title     string `json:"item_title"`
	Item_photo_url string `json:"item_photo_url"`
	Item_price     string `json:"item_price"`
}

type OrderResp struct {
	Order_id     int `json:"order_id"`
	App_order_id int `json:"app_order_id"`
}

type ErrorResp struct {
	Error_code int    `json:"error_code"`
	Error_msg  string `json:"error_msg"`
	Critical   bool   `json:"critical"`
}

type ResponseErr struct {
	Error ErrorResp `json:"error"`
}

type ResponseOK struct {
	Response interface{} `json:"response"`
}

func main() {
	conn_string := fmt.Sprintf("mongodb://172.17.0.1:27017/simple")
	log.Println("connection string: " + conn_string)

	session, err := mgo.Dial(conn_string)
	if err != nil {
		panic(err)
	}
	defer session.Close()

	session.SetMode(mgo.Monotonic, true)
	ensureIndex(session)

	r := mux.NewRouter()

	// Routes consist of a path and a handler function.
	r.HandleFunc("/*", preflightHandler).Methods("OPTIONS")
	r.HandleFunc("/", processHandler(session)).Methods("POST")
	r.HandleFunc("/orders/{user}/{app}", ordersHandler(session)).Methods("GET")
	r.HandleFunc("/test/orders/{user}/{app}", orders_testHandler(session)).Methods("GET")
	r.HandleFunc("/healthcheck", healthcheckHandler).Methods("GET")

	log.Println("server started on port ", 8000)
	// Bind to a port and pass our router in
	log.Fatal(http.ListenAndServe(":8000", r))
}

func ensureIndex(s *mgo.Session) {
	session := s.Copy()
	defer session.Close()

	ensureIndexPay(s)
	ensureIndexShowcase(s)
}

func ensureIndexPay(session *mgo.Session) {
	c := session.DB("simple").C("pay")
	index := mgo.Index{
		Key:        []string{"app_order_id"},
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

func ensureIndexShowcase(session *mgo.Session) {
	c := session.DB("simple").C("showcase")
	index := mgo.Index{
		Key:        []string{"item", "app_id"},
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

func ResponseWithString(w http.ResponseWriter, r *http.Request, message string, code int) {
	buf := bytes.NewBufferString("")
	fmt.Fprintf(buf, "{\"message\": %q}", message)
	ResponseWithJSON(w, r, buf.Bytes(), http.StatusOK)
}

func ResponseWithJSON(w http.ResponseWriter, r *http.Request, json []byte, code int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
	w.WriteHeader(code)
	w.Write(json)

	log.Println("response:")
	log.Println(string(json))
}

func preflightHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Accept, Accept-Encoding, Destination, Content-Type, Content-Length")
	w.WriteHeader(http.StatusOK)
}

func healthcheckHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("new healthcheck request")
	ResponseWithString(w, r, "pass", http.StatusOK)
}

func processHandler(s *mgo.Session) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := ioutil.ReadAll(r.Body)
		log.Println("new request:")
		log.Println(string(bodyBytes))

		session := s.Copy()
		defer session.Close()

		c_pay := session.DB("simple").C("pay")
		c_pay_test := session.DB("simple").C("pay_test")
		c_showcase := session.DB("simple").C("showcase")
		ai.Connect(session.DB("simple").C("counters"))

		ss := strings.Split(string(bodyBytes), "&")
		parms := make(map[string]string)
		for _, pair := range ss {
			z := strings.Split(pair, "=")
			parms[z[0]] = z[1]
		}

		log.Println("api request: " + parms["notification_type"])

		switch parms["notification_type"] {
		case "get_item","get_item_test":
			{
				log.Println("find item: app_id=" + parms["app_id"] + " item=\"" + parms["item"] + "\"")
				app_id, _ := strconv.Atoi(parms["app_id"])
				var item Item
				err := c_showcase.Find(bson.M{"app_id": app_id, "item": parms["item"]}).One(&item)
				if err != nil {
					ErrorResponse(w, r, 20, "Товар не существует", true)
					return
				}

				var item_resp ItemResp
				item_resp.Title = item.Title
				item_resp.Photo_url = item.Photo_url
				item_resp.Price = item.Price
				item_resp.Item_id = item.Item_id
				item_resp.Expiration = 600

				OKResponse(w, r, item_resp)
			}
		case "order_status_change":
			{

				if parms["status"] != "chargeable" {
					ErrorResponse(w, r, 101, "Передано непонятно что вместо chargeable", true)
					return
				}

				var err error

				var order Order
				order.App_order_id = (int)(ai.Next("pay"))
				order.App_id, err = strconv.Atoi(parms["app_id"])
				if err != nil {
					ErrorResponse(w, r, 105, "Ошибка конвертации (app_id)", true)
					return
				}
				order.User_id, err = strconv.Atoi(parms["user_id"])
				if err != nil {
					ErrorResponse(w, r, 105, "Ошибка конвертации (user_id)", true)
					return
				}
				order.Receiver_id, err = strconv.Atoi(parms["receiver_id"])
				if err != nil {
					ErrorResponse(w, r, 105, "Ошибка конвертации (receiver_id)", true)
					return
				}
				order.Order_id, err = strconv.Atoi(parms["order_id"])
				if err != nil {
					ErrorResponse(w, r, 105, "Ошибка конвертации (order_id)", true)
					return
				}
				order.Date, err = strconv.Atoi(parms["date"])
				if err != nil {
					ErrorResponse(w, r, 105, "Ошибка конвертации (date)", true)
					return
				}
				order.Status = parms["status"]
				order.Item = parms["item"]
				order.Item_id = parms["item_id"]
				order.Item_title = parms["item_title"]
				order.Item_photo_url = parms["item_photo_url"]
				order.Item_price = parms["item_price"]

				err = c_pay.Insert(order)
				if err != nil {
					if mgo.IsDup(err) {
						ErrorResponse(w, r, 102, "Ордер покупки существует", true)
					} else {
						ErrorResponse(w, r, 2, "Временная ошибка базы данных", true)
					}
					return
				}

				if update_user(w, r, session, parms, order.Item) != true {
					c_pay.Remove(bson.M{"app_order_id": order.App_order_id})
					return
				}

				var order_resp OrderResp
				order_resp.Order_id = order.Order_id
				order_resp.App_order_id = order.App_order_id

				OKResponse(w, r, order_resp)
			}
		case "order_status_change_test":
			{
				if parms["status"] != "chargeable" {
					ErrorResponse(w, r, 101, "Передано непонятно что вместо chargeable", true)
					return
				}

				var order Order
				order.App_order_id = (int)(ai.Next("test"))
				order.App_id, _ = strconv.Atoi(parms["app_id"])
				order.User_id, _ = strconv.Atoi(parms["user_id"])
				order.Receiver_id, _ = strconv.Atoi(parms["receiver_id"])
				order.Order_id, _ = strconv.Atoi(parms["order_id"])
				order.Date, _ = strconv.Atoi(parms["date"])
				order.Status = parms["status"]
				order.Item = parms["item"]
				order.Item_id = parms["item_id"]
				order.Item_title = parms["item_title"]
				order.Item_photo_url = parms["item_photo_url"]
				order.Item_price = parms["item_price"]

				err := c_pay_test.Insert(order)
				if err != nil {
					if mgo.IsDup(err) {
						ErrorResponse(w, r, 102, "Ордер покупки существует", true)
					} else {
						ErrorResponse(w, r, 2, "Временная ошибка базы данных", true)
					}
					return
				}

				if update_user(w, r, session, parms, order.Item) != true {
					c_pay_test.Remove(bson.M{"app_order_id": order.App_order_id})
					return
				}

				var order_resp OrderResp
				order_resp.Order_id = order.Order_id
				order_resp.App_order_id = order.App_order_id

				OKResponse(w, r, order_resp)
			}
		default:
			{
				ErrorResponse(w, r, 100, "Неизвестный notification_type: "+parms["notification_type"], true)
			}
		}
	}
}

func update_user(w http.ResponseWriter, r *http.Request, s *mgo.Session, parms map[string]string, item string) bool {
	users := s.DB("simple").C("arrows_users")

	//------------------------------
	var user User
	err := users.Find(bson.M{"id": parms["receiver_id"]}).One(&user)
	if err != nil {
		ErrorResponse(w, r, 103, "Пользователь не существует (nil)", true)
		return false
	}

	if user.Id == "" {
		ErrorResponse(w, r, 103, "Пользователь не существует", true)
		return false
	}

	if item == "buy_lvl" || strings.Contains(item, "offer") {
		//xp_amount += xp_damount
		xp_amount, err_amount := strconv.Atoi(user.Xp_amount)
		xp_damount, err_damount := strconv.Atoi(user.Xp_damount)
		if err_amount != nil || err_damount != nil {
			log.Println("Ошибка конвертации (xp_amount + xp_damount)")
			user.Xp_amount = "0"
			user.Xp_damount = "10"
		} else {
			xp_amount += xp_damount
			user.Xp_amount = strconv.Itoa(xp_amount)
		}
		//---
	} else {
		if item == "buy_all" {
			user.All_ok = "1"
		}
	}

	err = users.Update(bson.M{"id": user.Id}, &user)
	if err != nil {
		ErrorResponse(w, r, 104, "Ошибка обновления пользователя", true)
		return false
	}
	//------------------------------

	return true
}

func ordersHandler(s *mgo.Session) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()

		c := session.DB("simple").C("pay")

		ordersResponse(w, r, c)
	}
}

func orders_testHandler(s *mgo.Session) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()

		c := session.DB("simple").C("pay_test")

		ordersResponse(w, r, c)
	}
}

func ordersResponse(w http.ResponseWriter, r *http.Request, c *mgo.Collection) {
	vars := mux.Vars(r)
	log.Println("new orders request: user=" + vars["user"] + " app=" + vars["app"])

	receiver, err := strconv.Atoi(vars["user"])
	if err != nil {
		ResponseWithString(w, r, "error params", http.StatusOK)
		return
	}

	app, err := strconv.Atoi(vars["app"])
	if err != nil {
		ResponseWithString(w, r, "error params", http.StatusOK)
		return
	}

	orders := []Order{}
	err = c.Find(bson.M{"receiver_id": receiver, "app_id": app}).All(&orders)
	if err != nil {
		ResponseWithString(w, r, "database error", http.StatusOK)
		return
	}

	respBody, err := json.MarshalIndent(orders, "", "  ")
	if err != nil {
		log.Fatal(err)
	}

	ResponseWithJSON(w, r, respBody, http.StatusOK)
}

func ErrorResponse(w http.ResponseWriter, r *http.Request, error_code int, error_msg string, critical bool) {
	var responseErr ResponseErr
	responseErr.Error.Error_code = error_code
	responseErr.Error.Error_msg = error_msg
	responseErr.Error.Critical = critical

	respBody, err := json.Marshal(responseErr)
	if err != nil {
		log.Println("Error Marshal object")
	}
	ResponseWithJSON(w, r, respBody, http.StatusOK)
}

func OKResponse(w http.ResponseWriter, r *http.Request, i interface{}) {
	var rsp ResponseOK
	rsp.Response = i

	respBody, err := json.Marshal(rsp)
	if err != nil {
		log.Println("Error Marshal object")
	}
	ResponseWithJSON(w, r, respBody, http.StatusOK)
}
