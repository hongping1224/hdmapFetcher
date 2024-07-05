package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/tidwall/rtree"
)

func main() {
	map_path := flag.String("map", "./point_cloud_map_list.csv", "map list with colume in this order \"path,minx,miny,_,maxx,maxy,...\"")
	vec_path := flag.String("vec", "./vector_map_list.csv", "vector list with colume in this order \"path,minx,miny,maxx,maxy,...\"")
	vec_name := flag.String("vec_name", "", "vector name i.e sign , pole, markline")
	pos_path := flag.String("pos", "./pos.csv", "pos with colume in this order \"_,_,_,_,x,y,...\"")
	buffersize := flag.Float64("buffer", 50, "buffer size to find path")
	onlyvec := flag.Bool("onlyvec", false, "only download vec")
	out := flag.String("out", "./download.sh", "Output path")
	mode := flag.Uint("mode", 0, "download mode, 0 is gsutil, 1 is gcloud storage")
	flag.Parse()
	if *mode >= 2 {
		log.Printf("mode %d is not support, use mode 0", *mode)
		*mode = 0
	}
	var sb strings.Builder
	sb.WriteString("mkdir hd_maps\n")
	sb.WriteString("cd hd_maps\n")
	if !*onlyvec {
		err := generatePointCloudDownload(*map_path, *pos_path, *buffersize, *mode, &sb)
		if err != nil {
			log.Println(err)
			return
		}
	}
	err := generateVectorDownload(*vec_path, *pos_path, *vec_name, *buffersize, *mode, &sb)
	if err != nil {
		log.Println(err)
		return
	}
	//	gcloud storage -m cp "gs://hdmrc-setting/hd_maps/THSR/point_cloud_map/T61_North" .
	err = os.WriteFile(*out, []byte(sb.String()), 0644)
	if err != nil {
		log.Println(err)
		return
	}
}

func generatePointCloudDownload(map_path, pos_path string, buffersize float64, mode uint, sb *strings.Builder) error {
	start := time.Now()
	// create a 2D RTree
	pctree, err := GenerateTree(map_path, buffersize, 1, 2, 4, 5)
	if err != nil {
		return err
	}
	elapsed := time.Since(start)
	log.Printf("build took %s", elapsed)
	start = time.Now()
	pos, err := ReadPos(pos_path)
	if err != nil {
		return err
	}
	found := make(map[string]bool)

	for _, p := range pos {
		// search
		x := p[0]
		y := p[1]
		pctree.Search([2]float64{x, y}, [2]float64{x, y},
			func(min, max [2]float64, data string) bool {
				found[data] = true

				//println(data.(string)) // prints "PHX"
				return true
			},
		)
	}

	elapsed = time.Since(start)
	log.Printf("Search took %s", elapsed)
	log.Printf("Map Count %d", len(found))
	if len(found) == 0 {
		log.Printf("no map found , skip operation")
		return nil
	}
	sb.WriteString("mkdir point_cloud_map\n")
	sb.WriteString("cd point_cloud_map\n")
	if mode == 0 {
		sb.WriteString("gsutil -m cp")
	} else {
		sb.WriteString("gcloud storage cp")

	}
	for k := range found {
		sb.WriteString(" gs://hdmrc-setting/hd_maps/THSR/")
		sb.WriteString(k)
	}

	sb.WriteString(" .\n")
	sb.WriteString("cd ..\n")

	return nil
}

func generateVectorDownload(vector_path, pos_path, vec_name string, buffersize float64, mode uint, sb *strings.Builder) error {
	start := time.Now()
	// create a 2D RTree
	pctree, err := GenerateTree(vector_path, buffersize, 1, 2, 3, 4)
	if err != nil {
		return err
	}
	elapsed := time.Since(start)
	log.Printf("build took %s", elapsed)
	start = time.Now()
	pos, err := ReadPos(pos_path)
	if err != nil {
		return err
	}
	found := make(map[string]bool)
	vec_name = strings.ToLower(vec_name)
	for _, p := range pos {
		// search
		x := p[0]
		y := p[1]
		pctree.Search([2]float64{x, y}, [2]float64{x, y},
			func(min, max [2]float64, data string) bool {
				if strings.Contains(strings.ToLower(data), vec_name) {
					found[data] = true
				}
				//println(data.(string)) // prints "PHX"
				return true
			},
		)
	}

	elapsed = time.Since(start)
	log.Printf("Search took %s", elapsed)
	log.Printf("Map Count %d", len(found))
	if len(found) == 0 {
		log.Printf("no map found , skip operation")
		return nil
	}
	sb.WriteString("mkdir vector_map\n")
	sb.WriteString("cd vector_map\n")
	modestring := "gsutil -m cp"
	if mode == 1 {
		modestring = "gcloud storage cp"
	}
	for k := range found {
		dir := filepath.Dir(k)
		parent := filepath.Base(dir)
		sb.WriteString("mkdir ")
		sb.WriteString(parent)
		sb.WriteString("\n")
		sb.WriteString(modestring)
		sb.WriteString(" gs://hdmrc-setting/hd_maps/THSR/")
		sb.WriteString(strings.Replace(k, ".shp", ".*", -1))
		sb.WriteString(" ./")
		sb.WriteString(parent)
		sb.WriteString("/\n")
	}
	sb.WriteString("cd ..\n")

	return nil
}

func GenerateTree(paths string, buffersize float64, minxIndex, minyIndex, maxxIndex, maxyIndex int) (rtree.RTreeG[string], error) {
	var tr rtree.RTreeG[string]
	f, err := os.Open(paths)
	if err != nil {
		return tr, err
	}
	defer f.Close()

	csvr := csv.NewReader(f)
	for {
		row, err := csvr.Read()
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return tr, err
		}
		var minx, miny, maxx, maxy float64
		if minx, err = strconv.ParseFloat(row[minxIndex], 64); err != nil {
			fmt.Println(row)
			continue
		}
		if miny, err = strconv.ParseFloat(row[minyIndex], 64); err != nil {
			fmt.Println(row)
			continue
		}
		if maxx, err = strconv.ParseFloat(row[maxxIndex], 64); err != nil {
			fmt.Println(row)
			continue
		}
		if maxy, err = strconv.ParseFloat(row[maxyIndex], 64); err != nil {
			fmt.Println(row)
			continue
		}
		tr.Insert([2]float64{minx - buffersize, miny - buffersize}, [2]float64{maxx + buffersize, maxy + buffersize}, row[0])
	}
}

func ReadPos(path string) ([][2]float64, error) {
	var pos [][2]float64

	f, err := os.Open(path)
	if err != nil {
		return pos, err
	}
	defer f.Close()

	csvr := csv.NewReader(f)
	for {
		row, err := csvr.Read()
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return pos, err
		}
		var x, y float64
		if x, err = strconv.ParseFloat(row[4], 64); err != nil {
			fmt.Println(row)
			continue
		}
		if y, err = strconv.ParseFloat(row[5], 64); err != nil {
			fmt.Println(row)
			continue
		}
		pos = append(pos, [2]float64{x, y})
	}

}
