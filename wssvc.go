package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const (
	pingMeCmd       = "ping me"
	pongMeCmd       = "pong me"
	startPingMeCmd  = "start ping me"
	stopPingMeCmd   = "stop ping me"
	disconnectMeCmd = "disconnect me"
	sendMeCmdPrefix = "send me file://"
	controlWait     = 10 * time.Second
	pingTimeout     = 5 * time.Second
	lineSeparator = "============================="
)

var (
	stopOnClose  = make(chan int)
	stopOnCancel = make(chan string)
)

func homePage(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "<h1> Home Page </h1>")
}

func startPingCmd(conn *websocket.Conn) {
	for {
		err := sendControlMessage(conn, websocket.PingMessage, startPingMeCmd)
		if err != nil {
			return
		}
		select {
		case stop := <-stopOnClose:
			if stop != 1 {
				log.Printf("got signal, but not exit: %d", stop)
				return
			}
			log.Printf("received stop all signal, existing")
			return
		case cancel := <-stopOnCancel:
			if cancel != "cancelPing" {
				log.Printf("got stopOnCancel, but not exit: %s", cancel)
				return
			}
			log.Printf("received cancel signal, existing")
			return
		default:
			time.Sleep(pingTimeout / 2)
		}
	}
}

func sendMeFile(conn *websocket.Conn, msg string, fsRoot string) error {
	filePath := strings.ReplaceAll(msg, sendMeCmdPrefix, "")
	log.Printf("Got filePath %s", filePath)
	if strings.Contains(filePath, "..") {
		log.Printf("Only support specify file under %s", fsRoot)
		err := sendControlMessage(conn, websocket.CloseMessage, "close on read")
		if err != nil {
			log.Printf("Error: %v", err)
		}
		return errors.New("Reading file outside root")
	}
	fullPath := fmt.Sprintf("%s/%s", fsRoot, filePath)
	log.Printf("Reading file: %s", fullPath)
	fileContentBytes, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	log.Print("Sending file content to client")
	err = conn.WriteMessage(websocket.BinaryMessage, fileContentBytes)
	if err != nil {
		log.Print("error when sending file content")
		return err
	}
	log.Print("Sending file content to client success")
	return nil
}

func sendControlMessage(conn *websocket.Conn, messageType int, message string) error {
	log.Printf("sending control: type=%d, message=%s", messageType, message)
	deadline := time.Now().Add(controlWait)
	err := conn.WriteControl(messageType, []byte(message), deadline)
	if err != nil {
		log.Printf("got error when sending control message: %v", err)
		return err
	}
	return nil
}

func handleTextMessage(msg string, conn *websocket.Conn) error {
	switch msg {
	case pingMeCmd:
		return sendControlMessage(conn, websocket.PingMessage, pingMeCmd)
	case pongMeCmd:
		return sendControlMessage(conn, websocket.PongMessage, pongMeCmd)
	case disconnectMeCmd:
		return sendControlMessage(conn, websocket.CloseMessage, disconnectMeCmd)
	case startPingMeCmd:
		go startPingCmd(conn)
	case stopPingMeCmd:
		stopOnCancel <- "cancelPing"
	default:
		if strings.HasPrefix(msg, sendMeCmdPrefix) {
			return sendMeFile(conn, msg, "/tmp")
		}
		log.Printf("echoing generic text message: %s", msg)
		return conn.WriteMessage(websocket.TextMessage, []byte(msg))
	}
	return nil
}

func wsEndpoint(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatalf("Connection upgrade failed, %v", err)
		return
	}
	defer conn.Close()

	conn.SetCloseHandler(func(code int, text string) error {
		log.Printf("Close %d, text=%s, cancel task", code, text)
		stopOnClose <- 1
		return nil
	})
	conn.SetPingHandler(func(s string) error {
		log.Printf("PingHandler:: got ping")
		err = conn.WriteControl(websocket.PongMessage, []byte(s), time.Now().Add(controlWait))
		if err != nil {
			log.Printf("Error: %s", err)
			return err
		}
		log.Println("PingHandler:: sent pong")
		return nil
	})
	conn.SetPongHandler(func(s string) error {
		log.Print("PongHandler:: got pong")
		return nil
	})

	for {
		mt, msg, err := conn.ReadMessage()
		if err != nil {
			log.Println(err)
			return
		}

		log.Printf("received %d: %s", mt, msg)

		switch mt {
		case websocket.CloseMessage:
			log.Print("Got client close, closing")
		case websocket.BinaryMessage:
			log.Printf("Got binary, len=%d, sending content back to client", len(msg))
			if err := conn.WriteMessage(mt, msg); err != nil {
				log.Println(err)
				return
			}
		case websocket.TextMessage:
			log.Printf("Got text: %s", msg)
			if err = handleTextMessage(string(msg), conn); err != nil {
				log.Println(err)
				return
			}
		default:
			log.Printf("default %d", mt)
		}

	}
}

func setupRoutes() {
	http.HandleFunc("/", homePage)
	http.HandleFunc("/ws", wsEndpoint)
}

func main() {
	log.SetPrefix("[wsserver] ")
	var (
		server   = flag.String("serve", "ws://:8080", "Server string to serve")
		cert     = flag.String("cert", "", "Server certificate if server is wss://")
		pkey     = flag.String("pkey", "", "Server private key if server is wss://")
		fileRoot = flag.String("fileroot", "/tmp", "Default root path for file sending")
	)
	flag.Parse()

	log.Printf(lineSeparator)
	log.Print("Server configuration:")
	log.Printf(lineSeparator)
	log.Printf("Server: %s", *server)
	log.Printf("Certificate: %s", *cert)
	log.Printf("PrivateKey: %s", *pkey)
	log.Printf("FileRoot: %s", *fileRoot)
	log.Printf(lineSeparator)
	setupRoutes()
	if strings.HasPrefix(*server, "wss://") {
		log.Println("Starting websocket server with TLS")
		if len(*cert) == 0 || len(*pkey) == 0 {
			panic("Certificate cannot be empty if server is wss://")
		}
		s := strings.ReplaceAll(*server, "wss://", "")
		log.Fatal(http.ListenAndServeTLS(s, *cert, *pkey, nil))
	}
	s := strings.ReplaceAll(*server, "ws://", "")
	log.Println("Starting websocket server without TLS")
	log.Fatal(http.ListenAndServe(s, nil))
}
