package main

import (
	"encoding/json"
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
	getIps()
	http.ListenAndServe(":8000", nil)
}

func proxy(w http.ResponseWriter, req *http.Request) {
	t1 := time.Now()
	backend, _ := req.Cookie("backend")
	target := urls[random(0, 3)]
	var isCookieSet = false
	if backend != nil && backend.Value != "" {
		target = backend.Value
		isCookieSet = true
	}
	lbReq, _ := http.NewRequest("GET", "http://["+target+"]:1300"+req.URL.String(), nil)
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
			if !isCookieSet {
				expiration := time.Now().Add(24 * time.Hour)
				http.SetCookie(w, &http.Cookie{Name: "backend", Value: target, Expires: expiration, Path: "/"})
			}
			w.WriteHeader(response.StatusCode)
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
	backend, _ := r.Cookie("backend")
	target := urls[random(0, 3)]
	if backend.Value != "" {
		target = backend.Value
	}
	targetURL := "[" + target + "]:1300" + r.URL.String()
	d, err := net.Dial("tcp", targetURL)
	if err != nil {
		http.Error(w, "Error contacting backend server.", 500)
		log.Printf("Error dialing websocket backend %s: %v", targetURL, err)
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

func getIps() {
	go func() {
		for _ = range time.Tick(5 * time.Second) {
			resp, err := httpClient.Get("http://192.168.0.6:8080/api/v1/namespaces/default/pods")
			if err != nil {
				log.Println(err)
			}
			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			var parsed map[string][]map[string]map[string]interface{}
			json.Unmarshal(body, &parsed)
			var items = parsed["items"]
			for _, item := range items {
				status := item["status"]
				hostIP := status["hostIP"].(string)
				if hostIP[0:7] == "192.168" {
					//fmt.Println(status["podIP"])
				}
			}
		}
	}()
}
