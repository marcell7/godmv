package main

import (
	"archive/zip"
	"bufio"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/twpayne/go-proj/v10"
)

type BBox struct {
	minLat float64
	maxLat float64
	minLon float64
	maxLon float64
	// Transformed
	tminX float64
	tmaxX float64
	tminY float64
	tmaxY float64
}

// One task for each text file
type Task struct {
	id   int
	file string
}

// Result of each task
type Result struct {
	id          int
	nRows       int32
	matchedRows []string
}

// Urls to download DMV data from
var urls = []string{
	"https://ipi.eprostor.gov.si/jgp-service-api/display-views/groups/113/files/516",
	"https://ipi.eprostor.gov.si/jgp-service-api/display-views/groups/113/files/517",
	"https://ipi.eprostor.gov.si/jgp-service-api/display-views/groups/113/files/518",
	"https://ipi.eprostor.gov.si/jgp-service-api/display-views/groups/113/files/469",
}

// Downloaded zip file names
var zips = []string{
	"jv.zip",
	"jz.zip",
	"sv.zip",
	"sz.zip",
}

// Unzips a file to a folder
func unzip(file string, folder string) {
	dst := folder
	archive, err := zip.OpenReader(file)
	if err != nil {
		panic(err)
	}
	defer archive.Close()

	for _, f := range archive.File {
		filePath := filepath.Join(dst, strings.Split(f.Name, "/")[1])
		if !strings.HasPrefix(filePath, filepath.Clean(dst)+string(os.PathSeparator)) {
			return
		}
		if f.FileInfo().IsDir() {
			os.MkdirAll(filePath, os.ModePerm)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
			panic(err)
		}
		dstFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			panic(err)
		}
		fileInArchive, err := f.Open()
		if err != nil {
			panic(err)
		}
		if _, err := io.Copy(dstFile, fileInArchive); err != nil {
			panic(err)
		}

		dstFile.Close()
		fileInArchive.Close()
	}
}

// Downloads a zip file from the provided url
func download(url string, folder string, file string) string {
	fmt.Println("Downloading from: ", url)
	filepath := folder + "/" + file
	// Create file
	out, err := os.Create(filepath)
	if err != nil {
		panic(err)
	}
	defer out.Close()
	// Get http
	resp, err := http.Get(url)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	// Check for req errors
	if resp.StatusCode != http.StatusOK {
		panic(fmt.Errorf("download failed: %s", resp.Status))
	}
	// Write the response body to the file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		panic(err)
	}
	return filepath
}

// Converts user provided coordinates to the ones in the files (don't know much about these coordinate systems, but we are allegedly using EPSG:3794)
func getBBox(coords []string) BBox {
	var bbox BBox
	for i, boundry := range coords {
		boundryf, err := strconv.ParseFloat(boundry, 64)
		if err != nil {
			panic(err)
		}
		switch i {
		case 0:
			bbox.minLat = boundryf
		case 1:
			bbox.maxLat = boundryf
		case 2:
			bbox.minLon = boundryf
		case 3:
			bbox.maxLon = boundryf
		}
	}
	// Some conversion magic...Using Proj for this
	pj, err := proj.NewCRSToCRS("EPSG:4326", "EPSG:3794", nil)
	if err != nil {
		panic(err)
	}
	maxCoords := proj.NewCoord(bbox.maxLat, bbox.minLon, 0, 0)
	minCoords := proj.NewCoord(bbox.minLat, bbox.maxLon, 0, 0)
	tMaxCoords, _ := pj.Forward(maxCoords)
	tMinCoords, _ := pj.Forward(minCoords)
	bbox.tminX = tMaxCoords.X()
	bbox.tmaxY = tMaxCoords.Y()
	bbox.tmaxX = tMinCoords.X()
	bbox.tminY = tMinCoords.Y()
	return bbox
}

// Parse txt file line by line and check if a line is in the provided bbox
func parseCsv(id int, path string, bbox BBox) Result {
	var nRows int32
	var matchedRows []string
	file, err := os.Open(path)
	if err != nil {
		fmt.Println(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		x, _ := strconv.ParseFloat(strings.Split(scanner.Text(), " ")[0], 32)
		y, _ := strconv.ParseFloat(strings.Split(scanner.Text(), " ")[1], 32)
		if x >= bbox.tminX && x <= bbox.tmaxX && y >= bbox.tminY && y <= bbox.tmaxY {
			matchedRows = append(matchedRows, scanner.Text())
		}
		nRows++

	}
	return Result{id: id, nRows: nRows, matchedRows: matchedRows}
}

// Worker that processes a task from a task channel and sends the result to results channel
func worker(id int, bbox BBox, tasksChen <-chan Task, resultsChen chan<- Result) {
	for task := range tasksChen {
		result := parseCsv(id, task.file, bbox)
		resultsChen <- result
	}
}

func main() {
	// Parse user input
	userbbox := flag.String("bbox", "", "Robne koordinate v obliki 'minLat,maxLat,minLon,maxLon'")
	output := flag.String("output", "output.csv", "Izvožen csv")
	shouldDownload := flag.Bool("download", false, "Prenesi DMV podatke? Nastavi kot false, če jih že imaš")
	dataFolder := flag.String("data", "godmv_data", "Mapa, kjer so vse DMV .xyz datoteke")
	flag.Parse()
	if *userbbox == "" {
		fmt.Println("bbox missing")
		return
	}
	coords := strings.Split(*userbbox, ";")
	if len(coords) != 4 {
		return
	}
	// Get bbox
	bbox := getBBox(coords)
	if *shouldDownload {
		// Download each file and unzip it
		filesFolder := *dataFolder
		os.MkdirAll(filesFolder, os.ModePerm)

		for i, url := range urls {
			path := download(url, *dataFolder, zips[i])
			unzip(path, *dataFolder)
		}
	}
	fmt.Println("Processing...")
	var nRows int32
	files, err := filepath.Glob(*dataFolder + "/*.xyz")
	if err != nil {
		return
	}
	// Tasks channel
	tasksChan := make(chan Task, len(files))
	// Results channel
	resultsChan := make(chan Result, len(files))
	// Create workers to process tasks
	for w := 1; w <= len(files); w++ {
		go worker(w, bbox, tasksChan, resultsChan)
	}
	// Create tasks that need to be processed
	for idx, file := range files {
		tasksChan <- Task{id: idx, file: file}
	}
	// Close tasks channel - all tasks have been created
	close(tasksChan)
	// Create csv file
	file, _ := os.Create(*output)
	writer := bufio.NewWriter(file)
	// Collect and write all results
	for i := 1; i <= len(files); i++ {
		result := <-resultsChan
		for _, row := range result.matchedRows {
			writer.WriteString(row + "\n")
		}
		nRows = nRows + result.nRows
	}
	fmt.Printf("Processed rows: %d \n", nRows)

}
