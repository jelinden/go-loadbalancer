package main

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"sync/atomic"
	"time"
)

var urls atomic.Value
var httpClient = &http.Client{}

const domain = "www.uutispuro.fi"

func main() {
	getIps()
	http.HandleFunc("/socket.io/", websocketProxy)
	http.HandleFunc("/", proxy)
	go getIpsTimer()
	log.Println("starting server in port 8000")
	http.ListenAndServe(":8000", nil)
}

func proxy(w http.ResponseWriter, req *http.Request) {
	t1 := time.Now()
	content := getTarget(&w, req)
	if content == nil {
		w.WriteHeader(500)
	} else {
		w.Write(content)
	}
	t2 := time.Now()
	log.Printf("[%s] %v %q %v\n", req.Method, w.Header().Get("status"), req.URL.String(), t2.Sub(t1))
}

func getTarget(w *http.ResponseWriter, req *http.Request) []byte {
	target := urls.Load().([]string)[random(0, len(urls.Load().([]string)))]
	lbReq, _ := http.NewRequest("GET", "http://["+target+"]:1300"+req.URL.String(), nil)
	copyHeader(lbReq.Header, req.Header)
	response, err := httpClient.Do(lbReq)
	if err != nil {
		log.Println(err.Error())
		return nil
	}
	defer response.Body.Close()
	content, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Println(err.Error())
		return nil
	}
	copyHeader((*w).Header(), response.Header)
	(*w).WriteHeader(response.StatusCode)
	return content
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func random(min, max int) int {
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(max-min) + min
}

// thank you bradfitz, https://groups.google.com/forum/#!topic/golang-nuts/KBx9pDlvFOc
func websocketProxy(w http.ResponseWriter, r *http.Request) {
	backend, _ := r.Cookie("backend")
	target := urls.Load().([]string)[random(0, len(urls.Load().([]string)))]
	if backend != nil && backend.Value != "" && contains(urls.Load().([]string), backend.Value) {
		target = backend.Value
	}
	targetURL := "[" + target + "]:1300"
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

func getIpsTimer() {
	for _ = range time.Tick(10 * time.Second) {
		getIps()
	}
}

func getIps() {
	var tempUrls []string
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
		meta := item["metadata"]
		if status["podIP"] != nil && meta["generateName"] == "newsfeedreader-" {
			tempUrls = append(tempUrls, status["podIP"].(string))
		}
	}
	if len(tempUrls) > 0 {
		urls.Store(tempUrls)
	}
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
