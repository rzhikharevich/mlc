package main

import (
	"errors"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

type Resource struct {
	data []byte
	modt time.Time
	ctyp string
}

func (res Resource) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", res.ctyp)
	w.Header().Set("Last-Modified", res.modt.Format(http.TimeFormat))
	w.Write(res.data)
}

func TextPlaceholderSplit(data []byte) []string {
	return strings.Split(string(data), "${???}")
}

func (res Resource) CopyWithInserted(spl []string, ins ...string) (n Resource, err error) {
	if len(spl) < 2 {
		err = errors.New("expected at least 2 split components")
		return
	}

	if len(spl)-1 != len(ins) {
		err = errors.New(
			"expected " + strconv.FormatInt(int64(len(spl)-1), 10) + " insertions for " + strconv.FormatInt(int64(len(spl)), 10) + " components",
		)
		return
	}

	n.ctyp = res.ctyp
	n.modt = time.Now()

	s := spl[0]

	for i, c := range spl[1:] {
		s += ins[i] + c
	}

	n.data = []byte(s)

	return
}

func LoadResource(resPath string) (res Resource, err error) {
	f, err := os.Open(resPath)
	if err != nil {
		return
	}

	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return
	}

	res.modt = st.ModTime()

	res.data = make([]byte, st.Size())
	_, err = f.Read(res.data)

	res.ctyp = mime.TypeByExtension(path.Ext(resPath))

	return
}

func LoadResourceOrDie(resPath string, e *log.Logger) Resource {
	res, err := LoadResource(resPath)
	if err != nil {
		e.Fatalln(err.Error())
	}

	return res
}

func HandleAllResources(path string) (err error) {
	files := []string{
		"common.css",
		"logo.png",
	}

	for _, file := range files {
		var res Resource

		res, err = LoadResource(path + "/" + file)
		if err != nil {
			return
		}

		http.Handle("/"+file, res)
	}

	return
}
