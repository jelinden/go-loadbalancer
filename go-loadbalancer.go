package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

var urls []string
var httpClient = &http.Client{}

func init() {
	urls = strings.Split(os.Getenv("BALANCED_URLS"), ",")
}

func main() {
	http.HandleFunc("/socket.io/", websocketProxy)
	http.HandleFunc("/", proxy)
	http.ListenAndServe(":8000", nil)
}

func proxy(w http.ResponseWriter, req *http.Request) {
	t1 := time.Now()
	lbReq, _ := http.NewRequest("GET", urls[random(0, 3)]+req.URL.String(), nil)
	copyHeader(lbReq.Header, req.Header)
	response, err := httpClient.Do(lbReq)
	if err != nil {
		fmt.Printf("%s", err)
		w.WriteHeader(500)
	} else {
		defer response.Body.Close()
		contents, err2 := ioutil.ReadAll(response.Body)
		if err2 != nil {
			fmt.Printf("%s", err2)
			w.WriteHeader(500)
		} else {
			copyHeader(w.Header(), response.Header)
			w.Write(contents)
		}
	}
	t2 := time.Now()
	log.Printf("[%s] %v %q %v\n", req.Method, response.StatusCode, req.URL.String(), t2.Sub(t1))
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func random(min, max int) int {
	return rand.Intn(max-min) + min
}

// thank you bradfitz, https://groups.google.com/forum/#!topic/golang-nuts/KBx9pDlvFOc
func websocketProxy(w http.ResponseWriter, r *http.Request) {
	target := strings.Replace(urls[random(0, 3)], "http://", "", 1) + r.URL.String()
	d, err := net.Dial("tcp", target)
	if err != nil {
		http.Error(w, "Error contacting backend server.", 500)
		log.Printf("Error dialing websocket backend %s: %v", target, err)
		return
	}
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Not a hijacker?", 500)
		return
	}
	nc, _, err := hj.Hijack()
	if err != nil {
		log.Printf("Hijack error: %v", err)
		return
	}
	defer nc.Close()
	defer d.Close()

	err = r.Write(d)
	if err != nil {
		log.Printf("Error copying request to target: %v", err)
		return
	}

	errc := make(chan error, 2)
	cp := func(dst io.Writer, src io.Reader) {
		_, err := io.Copy(dst, src)
		errc <- err
	}
	go cp(d, nc)
	go cp(nc, d)
	log.Println(r.URL.String())
	<-errc
}
