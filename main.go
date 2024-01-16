package main

import (
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Entity struct {
	ID             string  `json:"id"`
	X              float64 `json:"x"`
	Y              float64 `json:"y"`
	Size           float64 `json:"size"`
	Speed          float64 `json:"speed"`
	AngleInDegrees float64 `json:"angle"`
}

var (
	upgrader    = websocket.Upgrader{ReadBufferSize: 1024, WriteBufferSize: 1024}
	entitites   []Entity
	entitiesMux sync.Mutex
)
var gameTicks = 0
var serverTicks = 0

func main() {
	http.HandleFunc("/ws", handleWebSocket)
	go gameLoop()
	go serverLoop()
	go logTicks()

	log.Println("Server is ready and listening on port 8080")
	// get port from env variable
	var SERVER_PORT = os.Getenv("PORT")
	if SERVER_PORT == "" {
		SERVER_PORT = "8080"
	}
	log.Fatal(http.ListenAndServe(":"+SERVER_PORT, nil))
}

var allConnections []*websocket.Conn = make([]*websocket.Conn, 0)

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	allConnections = append(allConnections, conn)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()

	// send the user unique id to the client

	entitiesMux.Lock()
	err = conn.WriteJSON(entitites)

	remoteAddress := conn.RemoteAddr().String()
	err = conn.WriteJSON(remoteAddress)
	entitiesMux.Unlock()
	if err != nil {
		log.Println(err)
		return
	}

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			log.Println(err)
			removePlayer(remoteAddress)
			for i, c := range allConnections {
				if c == conn {
					allConnections = append(allConnections[:i], allConnections[i+1:]...)
					break
				}
			}
			return
		}
		var player Entity
		err = conn.ReadJSON(&player)
		if err != nil {
			log.Println(err)
			return
		}

		entitiesMux.Lock()
		wasFound := false
		for i, p := range entitites {
			if p.ID == player.ID {
				entitites[i] = player
				wasFound = true
				break
			}
		}
		if !wasFound {
			entitites = append(entitites, player)
		}
		entitiesMux.Unlock()

	}

}

func logTicks() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			log.Println("Game ticks:", gameTicks, "Server ticks:", serverTicks)
		}
	}
}

func serverLoop() {
	ticker := time.NewTicker(time.Second / 30)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			serverTicks++
			entitiesMux.Lock()
			for _, conn := range allConnections {
				err := conn.WriteJSON(entitites)
				if err != nil {
					log.Println(err)
				}
			}
		}
		entitiesMux.Unlock()
	}
}

func gameLoop() {
	lastUpdateTime := time.Now()
	ticker := time.NewTicker(time.Second / 60)
	newPlayerTicker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	defer newPlayerTicker.Stop()

	for {
		select {
		case <-ticker.C:
			gameTicks++
			currentTime := time.Now()
			dt := currentTime.Sub(lastUpdateTime).Seconds()

			updatePlayers(dt)

			lastUpdateTime = currentTime

		case <-newPlayerTicker.C:
			newPlayer := generateRandomPlayer()
			entitiesMux.Lock()
			entitites = append(entitites, newPlayer)
			entitiesMux.Unlock()

			if len(entitites) > 10 {
				entitiesMux.Lock()
				entitites = entitites[:0]
				entitiesMux.Unlock()
			}
		}
	}
}

func updatePlayers(dt float64) {
	entitiesMux.Lock()
	defer entitiesMux.Unlock()

	for i := range entitites {
		displacementX := entitites[i].Speed * dt * math.Cos(entitites[i].AngleInDegrees*math.Pi/180)
		displacementY := entitites[i].Speed * dt * math.Sin(entitites[i].AngleInDegrees*math.Pi/180)

		newPositionX := entitites[i].X + displacementX
		newPositionY := entitites[i].Y + displacementY

		colisionPositionX := newPositionX
		colisionPositionY := newPositionY

		// add the size of the player to the colision position
		if displacementX > 0 {
			colisionPositionX += entitites[i].Size
		} else {
			colisionPositionX -= entitites[i].Size
		}
		if displacementY > 0 {
			colisionPositionY += entitites[i].Size
		} else {
			colisionPositionY -= entitites[i].Size
		}

		if colisionPositionX > 1920/2 {
			entitites[i].AngleInDegrees = 180 - entitites[i].AngleInDegrees
			colisionPositionX = 1920 / 2
		} else if colisionPositionX < -1920/2 {
			entitites[i].AngleInDegrees = 180 - entitites[i].AngleInDegrees
			colisionPositionX = -1920 / 2
		} else {
			entitites[i].X = newPositionX

		}

		if colisionPositionY > 1080/2 {
			entitites[i].AngleInDegrees = 360 - entitites[i].AngleInDegrees
			colisionPositionY = 1080 / 2
		}
		if colisionPositionY < -1080/2 {
			entitites[i].AngleInDegrees = 360 - entitites[i].AngleInDegrees
			colisionPositionY = -1080 / 2
		} else {
			entitites[i].Y = newPositionY
		}
	}
}

func generateRandomPlayer() Entity {
	return Entity{
		ID:             string(rune(rand.Intn(10000000000))),
		X:              0,
		Y:              0,
		Size:           (rand.Float64()*50 + 50) / 10,
		Speed:          rand.Float64()*300 + 300,
		AngleInDegrees: rand.Float64() * 360,
	}
}

func removePlayer(playerID string) {
	entitiesMux.Lock()
	defer entitiesMux.Unlock()

	for i, player := range entitites {
		if player.ID == playerID {
			entitites = append(entitites[:i], entitites[i+1:]...)
			break
		}
	}
}
