package main

// socksie is a SOCKS4/5 compatible proxy that forwards connections via 
// SSH to a remote host

import (
	"code.google.com/p/go.crypto/ssh"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"
)

var (
	USER  = flag.String("user", os.Getenv("USER"), "ssh username")
	HOST  = flag.String("host", "127.0.0.1", "ssh server hostname")
	PASS  = flag.String("pass", os.Getenv("SSH_PASSWORD"), "ssh password")
	CONNS = flag.Int("c", 7, "concurrent connections")
	CHANS = flag.Int("C", 7, "concurrent channels per connection")
	shuffle chan io.Reader
)

func init() { 
	flag.Parse() 
	shuffle = make(chan io.Reader, *CHANS)
	go func() {
		for {
			c := <- shuffle
			shuffle <- c
		}
	}()
}

func main() {
	config := &ssh.ClientConfig{
		User: *USER,
		Auth: []ssh.ClientAuth{
			ssh.ClientAuthPassword(password(*PASS)),
		},
	}

	log.Println("Starting ... ")
	t1 := time.Now()
	var conns sync.WaitGroup
	conns.Add(*CONNS)
	for i := 0; i < *CONNS; i++ {
		go startConn(config, &conns)
	}
	conns.Wait()
	t2 := time.Since(t1)
	log.Printf("Test duration %v", t2)
}

func startConn(config *ssh.ClientConfig, wg *sync.WaitGroup) {
	defer wg.Done()
	addr := fmt.Sprintf("%s:%d", *HOST, 22)
	conn, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		log.Printf("unable to connect to [%s]: %v", addr, err)
		return
	}
	defer conn.Close()

	var chans sync.WaitGroup
	chans.Add(*CHANS)
	for i := 0; i < *CHANS; i++ {
		go startChan(conn, &chans)
	}
	chans.Wait()
}

func startChan(conn *ssh.ClientConn, wg *sync.WaitGroup) {
	defer wg.Done()
	ch, err := conn.NewSession()
	if err != nil {
		log.Printf("unable to open session: %v", err)
		return
	}
	defer ch.Close()
	stdin, err := ch.StdinPipe()
	if err != nil {
		log.Printf("unable to open stdin: %v", err)
		return
	}
	stdout, err := ch.StdoutPipe()
	if err != nil {
		log.Printf("unable to open stdout: %v", err)
		return
	}
	shuffle <- stdout
	if err := ch.Start("cat"); err != nil {
		log.Printf("unable to start command: %v", err)
		return
	}
	defer ch.Wait()
	// prime the pump
	if _, err := stdin.Write(make([]byte, 37000)); err != nil {
		log.Printf("unable to write: %v", err)
		return
	}
	_, err = io.Copy(stdin, <- shuffle)
	log.Println(err)
}
