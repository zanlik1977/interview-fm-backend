package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"time"

	jpgresize "github.com/nfnt/resize"
)

func (s *service) processResizes(request resizeRequest, async string) ([]resizeResult, error) {
	switch async {
	case "true":
		// async true case
		results, err := s.processAsyncResize(request, async)
		if err != nil {
			log.Printf("Failed to resize asynchronously")
			return nil, err
		}
		return results, nil
	case "false":
		// asyncs false case
		results, err := s.processBlockResize(request, async)
		if err != nil {
			log.Printf("Failed to resize synchronously")
			return nil, err
		}
		return results, err
	default:
		// default case
		results, err := s.processDefaultResize(request, async)
		if err != nil {
			log.Printf("Failed to resize, default case")
			return nil, err
		}
		return results, nil
	}
}

func (s *service) processAsyncResize(request resizeRequest, async string) ([]resizeResult, error) {
	ch := make(chan *passImage)
	results := make([]resizeResult, 0, len(request.URLs))
	for _, url := range request.URLs {
		result := resizeResult{}
		id := genID(url)
		key := "/v1/image/" + id + ".jpeg"
		newURL := proto + hostport + key

		if s.cache.Contains(key) {
			result.URL = newURL
			result.Result = success
			result.Cached = true
			results = append(results, result)
			continue
		}

		ext := filepath.Ext(url)
		if ext == ".jpeg" {
			result.URL = newURL
			result.Result = success
			result.Cached = false
			results = append(results, result)
		}

		data, err := asyncFetchAndResize(url, request.Width, request.Height, async, ch)
		if err != nil {
			log.Printf("failed to resize")
			result.Result = failure
			results = append(results, result)
			continue
		}

		log.Print("caching ", key)
		s.cache.Add(key, data)
	}
	return results, nil
}

func (s *service) processBlockResize(request resizeRequest, async string) ([]resizeResult, error) {
	ch := make(chan *passImage)
	results := make([]resizeResult, 0, len(request.URLs))
	for _, url := range request.URLs {
		result := resizeResult{}
		id := genID(url)
		key := "/v1/image/" + id + ".jpeg"
		newURL := proto + hostport + key

		if s.cache.Contains(key) {
			result.URL = newURL
			result.Result = success
			result.Cached = true
			results = append(results, result)
			continue
		}

		data, err := blockFetchAndResize(url, request.Width, request.Height, async, ch)
		if err != nil {
			log.Printf("failed to resize")
			result.Result = failure
			results = append(results, result)
			continue
		}

		log.Print("caching ", key)
		s.cache.Add(key, data)

		result.URL = newURL
		result.Result = success
		result.Cached = false
		results = append(results, result)

	}
	return results, nil
}

func (s *service) processDefaultResize(request resizeRequest, async string) ([]resizeResult, error) {
	
	results := make([]resizeResult, 0, len(request.URLs))
	for _, url := range request.URLs {
		result := resizeResult{}
		id := genID(url)
		key := "/v1/image/" + id + ".jpeg"
		newURL := proto + hostport + key

		if s.cache.Contains(key) {
			result.URL = newURL
			result.Result = success
			result.Cached = true
			results = append(results, result)
			continue
		}

		data, err := fetchAndResize(url, request.Width, request.Height)
		if err != nil {
			log.Printf("failed to resize %s: %v", url, err)
			result.Result = failure
			results = append(results, result)
			continue
		}

		log.Print("caching ", key)
		s.cache.Add(key, data)

		result.URL = newURL
		result.Result = success
		result.Cached = false
		results = append(results, result)
	}
	return results, nil
}

func asyncFetchAndResize(url string, width uint, height uint, async string, ch chan *passImage) ([]byte, error) {
	
	data, err := fetch(url)
	if err != nil {
		return nil, err
	}

	return asyncResize(data, width, height, async, ch)
}

func blockFetchAndResize(url string, width uint, height uint, async string, ch chan *passImage) ([]byte, error) {
	
	data, err := fetch(url)
	if err != nil {
		return nil, err
	}

	return blockResize(data, width, height, async, ch)
}

func fetchAndResize(url string, width uint, height uint) ([]byte, error) {
	
	data, err := fetch(url)
	if err != nil {
		return nil, err
	}

	return resize(data, width, height)
}

func fetch(url string) ([]byte, error) {
	log.Print("fetching ", url)
	r, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch failed: %v", err)
	}
	defer r.Body.Close()

	if r.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("non-200 status: %d", r.StatusCode)
	}

	data, err := ioutil.ReadAll(io.LimitReader(r.Body, 15*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("failed to read fetch data: %v", err)
	}

	return data, nil
}

func asyncResize(data []byte, width uint, height uint, async string, ch chan *passImage) ([]byte, error) {
	// decode jpeg into image.Image
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to jped decode: %v", err)
	}

	go asyncThread(width, height, img, ch)
	passedImage := <-ch

	newData := bytes.Buffer{}
	err = jpeg.Encode(bufio.NewWriter(&newData), passedImage.newImage, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to jpeg encode resized image: %v", err)
	}

	return newData.Bytes(), nil
}

func blockResize(data []byte, width uint, height uint, async string, ch chan *passImage) ([]byte, error) {
	// decode jpeg into image.Image
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to jped decode: %v", err)
	}

	go asyncThread(width, height, img, ch)
	
	select {
	case passedImage := <-ch:
		newData := bytes.Buffer{}
		err = jpeg.Encode(bufio.NewWriter(&newData), passedImage.newImage, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to jpeg encode resized image: %v", err)
		}
		return newData.Bytes(), nil
	case <-time.After(3 * time.Second):
		log.Print("timeout")
	}

	return nil, nil
}

func resize(data []byte, width uint, height uint) ([]byte, error) {
	// decode jpeg into image.Image
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to jped decode: %v", err)
	}

	var newImage image.Image
	
	// if either width or height is 0, it will resize respecting the aspect ratio
	newImage = jpgresize.Resize(width, height, img, jpgresize.Lanczos3)
	
	newData := bytes.Buffer{}
	err = jpeg.Encode(bufio.NewWriter(&newData), newImage, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to jpeg encode resized image: %v", err)
	}

	return newData.Bytes(), nil
}

func genID(url string) string {
	hash := sha256.Sum256([]byte(url))
	return base64.URLEncoding.EncodeToString(hash[:])
}

func asyncThread(width, height uint, img image.Image, ch chan *passImage) {
	tempImage := jpgresize.Resize(width, height, img, jpgresize.Lanczos3)
	ch <- &passImage{newImage: tempImage}
}
