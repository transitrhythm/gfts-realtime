package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httptrace"
	"os"
	"time"

	"github.com/beevik/ntp"
	"transitrhythm.com/gtfs/realtime/server/process"
)

var (
	p = fmt.Println
	l = log.Println
)

const (
	ntpSite                 = "time.google.com" //"0.beevik-ntp.pool.ntp.org"
	referenceTimeZone       = "GMT"
	defaultDownloadInterval = time.Duration(time.Second * 1)
)

type httpSchedule struct {
	//	hourTime string
	url string
	dst string
	//next     scheduler.Job
}

func timeTrack(start time.Time, name string) time.Duration {
	elapsed := time.Since(start)
	p(name, "=", elapsed)
	return elapsed
}

// MaxDuration returns the larger of x or y.
func MaxDuration(x, y time.Duration) time.Duration {
	if x < y {
		return y
	}
	return x
}

// MinDuration returns the smaller of x or y.
func MinDuration(x, y time.Duration) time.Duration {
	if x > y {
		return y
	}
	return x
}

// https://transitfeeds.com/p/translink-vancouver/29/latest/download
// https://gtfs.translink.ca/v2/gtfsrealtime?apikey=YUkPsIbQR9hPaB4YWjvy
// https://gtfs.translink.ca/v2/gtfsposition?apikey=YUkPsIbQR9hPaB4YWjvy
// https://gtfs.translink.ca/v2/gtfsalerts?apikey=YUkPsIbQR9hPaB4YWjvy
// Accept application/json
// https://api.translink.ca/rttiapi/v1/buses?apikey=YUkPsIbQR9hPaB4YWjvy - Returns details for all active buses
// https://api.translink.ca/rttiapi/v1/status/all?apikey=YUkPsIbQR9hPaB4YWjvy - Resturns status for location and schedule
// https://api.translink.ca/rttiapi/v1/status/location?apikey=YUkPsIbQR9hPaB4YWjvy - Returns status for location

func main() {
	//cityCount := 6
	cityNames := []string{
		"victoria",
		// "nanaimo",
		// "comox",
		// "kamloops",
		// "kelowna",
		// "squamish"
	}
	//ipAddress := []string{}
	//fileCount := 5
	fileNames := []string{
		// "google_transit.zip",
		// "trip_reference.txt",
		// "gtfrealtime_TripUpdates.bin",
		// "gtfrealtime_ServiceAlerts.bin",
		"gtfrealtime_VehiclePositions.bin",
	}

	site := ".mapstrat.com"
	srcfolder := "/current/"
	dstfolder := "../data/"

	//downloadSchedule := []httpSchedule{}
	/*
		// Worker goroutines to download all files concurrently.
		job := func(cityIndex int, fileIndex int) {
			scheduleIndex := cityIndex*len(fileNames) + fileIndex
			go DownloadFile(httpSchedule[scheduleIndex])
		}
	*/
	ntpTime, _ := ntp.Time(ntpSite)
	p("Real-time latency:", time.Since(ntpTime))

	// Initialize the download schedule table with Url and Dst filespec, and a blank schedule.
	for cityIndex := 0; cityIndex < len(cityNames); cityIndex++ {
		cityName := cityNames[cityIndex]
		ip, err := net.LookupHost(cityName + site)
		if err != nil {
			p("IP Lookup Problem:", cityName+site, ip[0], "=", err)
			return
		}
		//ipAddress = append(ipAddress, ip[0])
		for fileIndex := 0; fileIndex < len(fileNames); fileIndex++ {
			//url := "https://" + ip[0] + folder + fileNames[fileIndex]
			url := "https://" + cityName + site + srcfolder + fileNames[fileIndex]
			dst := dstfolder + cityName + "/" + fileNames[fileIndex]
			entrySchedule := httpSchedule{url, dst}
			//downloadSchedule = append(downloadSchedule, entrySchedule)
			//p(entrySchedule)
			go DownloadFile(entrySchedule)
			//job(cityIndex, fileIndex)
		}
	}

	for {
		time.Sleep(time.Second * 1)
	}
}

// SaveFile copies the file in the body of the reponse into a local file repository
func SaveFile(response *http.Response, filespec string, timestamp string) (written int64, err error) {
	// Create the file
	out, err := os.Create(filespec)
	if err != nil {
		l("File open error: ", err)
		return written, err
	}
	defer out.Close()

	// Write the body to file
	written, err = io.Copy(out, response.Body)
	if err != nil {
		l("File write error: ", err)
		return written, err
	}
	//defer response.Body.Close()

	ftime, err := time.Parse(time.RFC1123, timestamp)
	// change both atime and mtime to lastModifiedTime
	err = os.Chtimes(filespec, ftime, ftime)
	if err != nil {
		l("File timestamp error: ", err)
	}
	return written, err
}

func traceHTTP(req *http.Request) {
	trace := &httptrace.ClientTrace{
		DNSDone: func(dnsInfo httptrace.DNSDoneInfo) {
			p("DNS Info: %+v\n", dnsInfo)
		},
		GotConn: func(connInfo httptrace.GotConnInfo) {
			p("Got Conn: %+v\n", connInfo)
		},
	}
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
	if _, err := http.DefaultTransport.RoundTrip(req); err != nil {
		log.Fatal(err)
	}
}

func getResponse(url string, sinceTime string) (*http.Response, error) {
	// Determine Last-Modified-Time for source file
	var response *http.Response
	response = nil
	client := &http.Client{}
	request, err := http.NewRequest(http.MethodHead, url, nil)
	if err == nil {
		response, err := client.Do(request)
		if err != nil {
			return nil, err
		}
		//defer response.Body.Close()
		if response.StatusCode == http.StatusOK {
			lastCacheModifiedTime := response.Header.Get("Last-Modified")
			cacheTime, _ := time.Parse(time.RFC1123, lastCacheModifiedTime)
			fileTime, _ := time.Parse(time.RFC1123, sinceTime)
			if fileTime.Sub(cacheTime) < 0 {
				request, err = http.NewRequest(http.MethodGet, url, nil)
				if err == nil {
					// Add date flag to check for file changes since the last download
					request.Header.Set("If-Modified-Since", sinceTime)
					response, err = client.Do(request)
					if err != nil {
						return nil, err
					}
					if response.StatusCode == http.StatusOK {
						return response, err
					}
					//					defer response.Body.Close()
				}
			}
		}
	}
	return response, err
}

func fileUpdate(url string, filespec string, sinceTime string) (response *http.Response, written int64, lastModified string, err error) {
	// Check if source file has been modified since the file last modified time.
	response, err = getResponse(url, sinceTime)
	if err == nil && response != nil {
		lastModified = response.Header.Get("Last-Modified")
		written, err = SaveFile(response, filespec, lastModified)
	}
	return response, written, lastModified, err
}

// DownloadFile will download a url-specified cached file to a local file.
func DownloadFile(schedule httpSchedule) {
	var response *http.Response
	filespec := schedule.dst
	url := schedule.url
	for {
		// Does destination data file exist?
		var updatedFileModifiedTime string
		var written int64
		// If file exists, then convert local file timestamp into GMT string
		file, err := os.Stat(filespec)
		if err == nil {
			lastFileModifiedTime, err := fileModifiedTime(file, referenceTimeZone)
			if err == nil {
				response, written, updatedFileModifiedTime, err = fileUpdate(url, filespec, lastFileModifiedTime)
				if err == nil {
					cacheTime, err := localTimeFromReferenceTimestamp(updatedFileModifiedTime)
					if err == nil {
						latency := time.Since(cacheTime)
						p("a. Now =", time.Now(), "; Filepath =", filespec, "; Written", written, "; Last modified =", cacheTime, "; Latency:", latency)
					}
				}
			}
		} else {
			// Otherwise, get the data & create a new file
			response, err = http.Get(url)
			if err == nil {
				updatedFileModifiedTime = response.Header.Get("Last-Modified")
				written, _ = SaveFile(response, filespec, updatedFileModifiedTime)
				cacheTime, err := localTimeFromReferenceTimestamp(updatedFileModifiedTime)
				if err == nil {
					latency := time.Since(cacheTime)
					p("b. Now =", time.Now(), "; Filepath =", filespec, "; Written", written, "; Last modified =", cacheTime, "; Latency:", latency)
				}
			}
		}
		body, err := ioutil.ReadAll(response.Body)
		go process.Process(body, len(body))
		//	response.Body.Close()

		//waitDuration, err := loopDuration(updatedFileModifiedTime, defaultDownloadInterval)
		//p("WaitDuration:", waitDuration)
		time.Sleep(defaultDownloadInterval)
	}
}

func loopDuration(datestamp string, interval time.Duration) (time.Duration, error) {
	var elapsed time.Duration
	timeValue, err := time.Parse(time.RFC1123, datestamp)
	if err == nil {
		elapsed = time.Since(timeValue)
	}
	//p("Interval:", interval, "Elapsed:", elapsed)
	return MinDuration(interval-elapsed, interval), err
}

// Convert reference timestamp into local time
func localTimeFromReferenceTimestamp(timestamp string) (time.Time, error) {
	var localTime time.Time
	referenceTime, err := time.Parse(time.RFC1123, timestamp)
	if err == nil {
		localTime = referenceTime.Local()
	}
	return localTime, err
}

// Convert local file timestamp into GMT string
func fileModifiedTime(file os.FileInfo, timeZone string) (string, error) {
	lastModifiedFileTime := file.ModTime()
	location, err := time.LoadLocation(timeZone)
	return lastModifiedFileTime.In(location).Format(time.RFC1123), err
}
