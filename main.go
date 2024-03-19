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
)

// Point with x and y coord
type Pt struct {
	x float64
	y float64
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

// Parse txt file line by line and check if a line is in the provided bbox
func parseCsv(id int, path string, pt1 Pt, pt2 Pt) Result {
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
		if x >= pt1.x && x <= pt2.x && y >= pt2.y && y <= pt1.y {
			matchedRows = append(matchedRows, scanner.Text())
		}
		nRows++
	}
	return Result{id: id, nRows: nRows, matchedRows: matchedRows}
}

// Worker that processes a task from a task channel and sends the result to results channel
func worker(id int, pt1 Pt, pt2 Pt, tasksChen <-chan Task, resultsChen chan<- Result) {
	for task := range tasksChen {
		result := parseCsv(id, task.file, pt1, pt2)
		resultsChen <- result
	}
}

func main() {
	// Parse user input
	var res int
	pt1raw := flag.String("pt1", "", "Zgornja leva točka območja")
	pt2raw := flag.String("pt2", "", "Spodnja desna točka območja")
	resRaw := flag.Int("res", 5, "Resolucija. Po defaultu so podatki na 5 (5m X 5m). Lahko se nastavi na: 50 (50m X 50m), 500 (500m X 500m), 5000 (5000m X 5000m)")
	output := flag.String("output", "output.csv", "Izvožen csv")
	shouldDownload := flag.Bool("download", false, "Prenesi DMV podatke? Nastavi kot false, če jih že imaš")
	dataFolder := flag.String("data", "godmv_data", "Mapa, kjer so vse DMV .xyz datoteke")
	flag.Parse()

	pt1xy := strings.Split(*pt1raw, " ")
	pt2xy := strings.Split(*pt2raw, " ")
	if !(len(pt1xy) == 2 && len(pt2xy) == 2) {
		fmt.Println("invalid pt values")
		return
	}
	pt1x, _ := strconv.ParseFloat(pt1xy[0], 64)
	pt1y, _ := strconv.ParseFloat(pt1xy[1], 64)
	pt2x, _ := strconv.ParseFloat(pt2xy[0], 64)
	pt2y, _ := strconv.ParseFloat(pt2xy[1], 64)
	pt1 := Pt{x: pt1x, y: pt1y}
	pt2 := Pt{x: pt2x, y: pt2y}

	switch *resRaw {
	case 5, 50, 500, 5000:
		res = *resRaw / 5
	default:
		res = 1
	}

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
		go worker(w, pt1, pt2, tasksChan, resultsChan)
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
		counter := 0
		for _, row := range result.matchedRows {
			if counter%res == 0 {
				writer.WriteString(row + "\n")
				counter = 0
			}
			counter++
		}
		nRows = nRows + result.nRows
	}
	fmt.Printf("Processed rows: %d \n", nRows)
}
