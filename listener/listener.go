// Listener capture TCP traffic using RAW SOCKETS.
// Note: it requires sudo or root access.
//
// Rigt now it suport only HTTP
package listener

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"
)

// Enable debug logging only if "--verbose" flag passed
func Debug(v ...interface{}) {
	if Settings.Verbose {
		log.Println(v...)
	}
}

func ReplayServer() (conn net.Conn, err error) {
	// Connection to reaplay server
	conn, err = net.Dial("tcp", Settings.ReplayServer())

	if err != nil {
		log.Println("Connection error ", err, Settings.ReplayAddress)
	}

	return
}

// Because its sub-program, Run acts as `main`
func Run() {
	if os.Getuid() != 0 {
		fmt.Println("Please start the listener as root or sudo!")
		fmt.Println("This is required since listener sniff traffic on given port.")
		os.Exit(1)
	}

	fmt.Println("Listening for HTTP traffic on", Settings.Address+":"+strconv.Itoa(Settings.Port))
	fmt.Println("Forwarding requests to replay server:", Settings.ReplayServer(), "Limit:", Settings.ReplayLimit)

	// Sniffing traffic from given address
	listener := RAWTCPListen(Settings.Address, Settings.Port)

	currentTime := time.Now().UnixNano()
	currentRPS := 0

	var messageLogger *MessageLogger

	if Settings.FileToReplyPath != "" {
		messageLogger = NewLog(Settings.FileToReplyPath)
	}

	for {
		// Receiving TCPMessage object
		m := listener.Receive()

		if Settings.ReplayLimit != 0 {
			if (time.Now().UnixNano() - currentTime) > time.Second.Nanoseconds() {
				currentTime = time.Now().UnixNano()
				currentRPS = 0
			}

			if currentRPS >= Settings.ReplayLimit {
				continue
			}

			currentRPS++
		}

		if messageLogger != nil {
      fmt.Println("FILE: ", Settings.FileToReplyPath)
			go func() {
				messageBuffer := new(bytes.Buffer)
				messageWriter := bufio.NewWriter(messageBuffer)

				fmt.Fprintf(messageWriter, "%s", string(m.Bytes()))
				fmt.Fprintf(messageWriter, "\n--\n")

				messageWriter.Flush()
				messageLogger.messageChannel <- messageBuffer.String()
			}()
    } else {
      go sendMessage(m)
		}

	}
}

func sendMessage(m *TCPMessage) {
	conn, err := ReplayServer()

	if err != nil {
		log.Println("Failed to send message. Replay server not respond.")
		return
	} else {
		defer conn.Close()
	}

	// For debugging purpose
	// Usually request parsing happens in replay part
	if Settings.Verbose {
		buf := bytes.NewBuffer(m.Bytes())
		reader := bufio.NewReader(buf)

		request, err := http.ReadRequest(reader)

		if err != nil {
			Debug("Error while parsing request:", err, string(m.Bytes()))
		} else {
			request.ParseMultipartForm(32 << 20)
			Debug("Forwarding request:", request)
		}
	}

	_, err = conn.Write(m.Bytes())

	if err != nil {
		log.Println("Error while sending requests", err)
	}
}
