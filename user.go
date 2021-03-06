package main

import (
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
	"sync"

	"net/http"
)

const (
	LoginErrorNone    = 0x00
	LoginErrorOffline = 0x01
	LoginErrorBusy    = 0x02
)

type User struct {
	br         *Broker
	sid        string
	devid      string
	conn       *websocket.Conn
	closeMutex sync.Mutex
	closed     bool
}

type UsrMessage struct {
	sid     string
	msgType int
	data    []byte
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (u *User) writeMessage(messageType int, data []byte) {
	u.conn.WriteMessage(messageType, data)
}

func (u *User) close() {
	defer u.closeMutex.Unlock()

	u.closeMutex.Lock()

	if !u.closed {
		u.closed = true
		u.conn.Close()
		u.br.logout <- u.sid
	}
}

func (u *User) loginAck(code int) {
	msg := fmt.Sprintf(`{"type":"login","err":%d}`, code)
	u.writeMessage(websocket.TextMessage, []byte(msg))
}

func (u *User) readLoop() {
	defer u.close()

	for {
		msgType, data, err := u.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Error().Msg(err.Error())
			}
			break
		}

		u.br.userMessage <- &UsrMessage{u.sid, msgType, data}
	}
}

func serveUser(br *Broker, w http.ResponseWriter, r *http.Request) {
	devid := r.URL.Query().Get("devid")
	if devid == "" {
		http.Error(w, "devid required", http.StatusForbidden)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "Upgrade fail", http.StatusBadRequest)
		log.Error().Msg(err.Error())
		return
	}

	user := &User{
		br:    br,
		conn:  conn,
		devid: devid,
	}

	go user.readLoop()

	br.login <- user
}
