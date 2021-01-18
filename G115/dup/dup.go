package main

import (
	"fmt"
	"io"
	"os"
	//    "strconv"
	"crypto/sha256"
	"path/filepath"
	"strings"
	"time"
)

type dupRecord struct {
	fullpath      string // full path of base file
	duplicateList string // list of duplicates, csv separated  "filename.ext","filename.ext","filename.ext"
}

const VERBOSE = false

const ERR_INVALID_ARGUMENTS = 0x0001
const ERR_INVALID_BASE_FOLDER = 0x0002
const ERR_INVALID_SEARCH_FOLDER = 0x0003

// missing some other exit codes.

var ctrEmptyFile int64
var ctrBaseUnique int64 // inaccurate name
var ctrBaseDup int64    // inaccurate name
var ctrSubFolders int64
var ctrFilesProcessed int64
var ctrGBytes float64
var dups map[string]dupRecord
var fileCtr int = 0
var fileCtr25 int = 0

// intent to set a status code; print more readable explanation; exit the application
func exitProc(exit_code int) {
	// todo; maybe turn exit code into nice text
	if VERBOSE {
		fmt.Println("Exit code", exit_code)
	}
	os.Exit(exit_code)
}

// return true if the folder name passed in a) exists, and b) is a folder (not a file)
// also returns false if an error calling os.stat; although unsure what all that covers
func folderExists(folder string) bool {
	finf, err := os.Stat(folder)

	if os.IsNotExist(err) {
		return false
	}

	if err == nil {
		if finf.IsDir() {
			return true
		} else {
			fmt.Println(folder, "exists, but is not a folder")
		}

	}

	fmt.Println("Error checking ", folder, err)
	return false
}

// generate a hash for the file passed in on filename
// currently using sha265
func hashFile(fn string) string {
	f, err := os.Open(fn)
	if err != nil {
		fmt.Println("Error opening", fn, err)
		return "0"
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		fmt.Println("Error copying", fn, err)
		return "0"
	}

	return fmt.Sprintf("%x", h.Sum(nil))
}

// walk file function, used by filepath.WalkFile
// prints crude status every 25 files, total GB processed every 250 files
// intent is to a) generate a hash for each __file__ and either a) add it to the duplicate
// map (if new) or b) add it as a duplicate if hash alread exists
// skips folders for hash
// skips folders for walking, unless recurse is true
// skips zero size files
// skips files > 75mb
func processFile(rootFolder string, recursive bool, isBase bool) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {

		// every 25 files, update the total files (print a .)
		fileCtr++
		if fileCtr > 25 {
			fmt.Print(".")
			fileCtr = 0
			fileCtr25++
		}

		// every 250 files, update the total GB processed
		if fileCtr25 > 10 {
			fmt.Println("Processed:", fmt.Sprintf("%.2fGb", ctrGBytes))
			fileCtr25 = 0
		}

		var key string
		var shash string
		var ssize string
		var duprec dupRecord

		var isRoot bool = strings.EqualFold(filepath.Clean(path), rootFolder)

		if info.IsDir() {
			if !isRoot {
				if !recursive {
					//fmt.Println("Skipping folder (recursive = off):", path, rootFolder)
					return filepath.SkipDir
				}

				tmp := filepath.Base(path)
				// could add more folders to skip here
				if strings.EqualFold(tmp, ".git") {
					//fmt.Println("Skipping excluded folder:", path)
					return filepath.SkipDir
				}
				ctrSubFolders++
			}
			return nil
		}

		if info.Size() <= 0 {
			ctrEmptyFile++
			//fmt.Println("Skipping empty file:", path)
			return nil // don't process empty file
		}

		if info.Size() >= 75000000 { // 75Mb
			ctrEmptyFile++
			fmt.Println("Skipping large file:", info.Size(), path)
			return nil // don't process empty file
		}

		if !isRoot {
			ssize = fmt.Sprintf("%012d", info.Size()) // 12 digits = 999gb  999,999,999,999
			shash = hashFile(path)
			key = ssize + shash

			ctrFilesProcessed++
			ctrGBytes += (float64(info.Size()) / (1024.0 * 1024.0 * 1024.0))

			//fmt.Println(info.Size(), fmt.Sprintf("%.2f", ctrGBytes))

			duprec.fullpath = path
			_, exists := dups[key]

			if !exists {
				if isBase { // add to map, if we're processing the base folder
					ctrBaseUnique++
					dups[key] = duprec
				}
			} else {
				ctrBaseDup++

				tmp := dups[key]
				tmp.duplicateList += "\"" + path + "\","

				dups[key] = tmp

				//fmt.Println("  Base File:            ", dups[key].fullpath)
				//if isBase {
				//	fmt.Println("  Duplicate (in base):  ", path)
				//} else {
				//	fmt.Println("  Duplicate (in search):", path)
				//}
				//fmt.Println("")
			}
		}

		return nil
	}
}

// returns string passed in but with double quotes around it
func doubleQuote(_s string) string {
	var s = strings.Trim(_s, " \"")
	return "\"" + s + "\""
}

// =======================================================================================Main
func main() {
	fmt.Println("")

	// program will find duplicate files
	// need 2 args; "main/base" folder, and folder to scan
	// files in "main/base" folder will be preferred/kept -- duplicate
	// files found outside of this folder will be flagged as duplicates

	if len(os.Args) != 5 {
		fmt.Println("Invalid number of arguments, four expected.")
		fmt.Println("Usage: dup.exe \"base-folder\" \"search-folder\" /B[R/N] /S[R/N]")
		fmt.Println("  /B=base folder; /S=search folder; R=recursive; N=not recursive")
		fmt.Println("  example:  dup.exe \"c:\\Images\" \"c:\\Temp\" /BR /SN")
		os.Exit(ERR_INVALID_ARGUMENTS)
	}

	var mainFolder string = filepath.Clean(os.Args[1])
	var searchFolder string = filepath.Clean(os.Args[2])

	// note - no error checking on order/contents of recurse arguments
	// only exactly /BR and /SR will trigger recursion ... anything else will not
	var baseRecurse bool = os.Args[3] == "/BR"
	var searchRecurse bool = os.Args[4] == "/SR"

	if !folderExists(mainFolder) {
		fmt.Println("Main folder does not exist: " + mainFolder)
		os.Exit(ERR_INVALID_BASE_FOLDER)
	} else {
		fmt.Println("OK: Using main/base folder: ", mainFolder, " Recursive:", baseRecurse)
	}

	if !folderExists(searchFolder) {
		fmt.Println("Search folder does not exist: " + searchFolder)
		os.Exit(ERR_INVALID_SEARCH_FOLDER)
	} else {
		fmt.Println("OK: Using search folder: ", searchFolder, " Recursive:", searchRecurse)
	}

	if strings.EqualFold(mainFolder, searchFolder) {
		fmt.Println("Base/Search folder cannot be the same.")
		os.Exit(0)
	}

	// should also check here to make sure SearchFolder is not a SUB Folder of base IF base Recursion is on
	// ...sub folder of base would be ok, as long as base folder is not recursive

	dups = make(map[string]dupRecord)

	// =====================Process base folder
	ctrEmptyFile = 0
	ctrBaseUnique = 0
	ctrBaseDup = 0
	ctrSubFolders = 0
	ctrFilesProcessed = 0
	ctrGBytes = 0.0
	start := time.Now()

	err := filepath.Walk(mainFolder, processFile(mainFolder, baseRecurse, true))

	if err != nil {
		fmt.Println("Error:", err)
		panic(err)
	}

	fmt.Println("")
	fmt.Println("Base Folder Processed")
	fmt.Println("  -Total files processed:", ctrFilesProcessed)
	fmt.Println("  -Total Gb processed:", fmt.Sprintf("%.2f", ctrGBytes))
	fmt.Println("  -Elapsed time:", fmt.Sprintf("%.2fs", time.Since(start).Seconds()), fmt.Sprintf("[%.2fgb/s]", ctrGBytes/(time.Since(start)).Seconds()))
	fmt.Println("  -Base folder, Unique Files:", ctrBaseUnique)
	fmt.Println("  -Base folder, Duplicates:", ctrBaseDup)
	fmt.Println("  -Sub folders Processed:", ctrSubFolders)
	fmt.Println("  -Empty files, Skipped:", ctrEmptyFile)
	fmt.Println("")

	// =====================Process sub folder
	ctrEmptyFile = 0
	ctrBaseUnique = 0
	ctrBaseDup = 0
	ctrSubFolders = 0
	ctrFilesProcessed = 0
	ctrGBytes = 0.0
	start = time.Now()

	err = filepath.Walk(searchFolder, processFile(searchFolder, searchRecurse, false))

	if err != nil {
		fmt.Println("Error:", err)
		panic(err)
	}

	var percnt float64 = 0.0
	if ctrBaseDup > 0 {
		percnt = (float64(ctrFilesProcessed) / float64(ctrBaseDup)) * 100.0
	}

	fmt.Println("")
	fmt.Println("Search Folder Processed")
	fmt.Println("  -Total files processed:", ctrFilesProcessed)
	fmt.Println("  -Total Gb processed:", fmt.Sprintf("%.2f", ctrGBytes))
	fmt.Println("  -Elapsed time:", fmt.Sprintf("%.2fs", time.Since(start).Seconds()), fmt.Sprintf("[%.2fgb/s]", ctrGBytes/(time.Since(start)).Seconds()))
	fmt.Println("  -Duplicates:", ctrBaseDup, fmt.Sprintf("[%.1f%%]", percnt))
	fmt.Println("  -Sub folders Processed:", ctrSubFolders)
	fmt.Println("  -Empty files, Skipped:", ctrEmptyFile)
	fmt.Println("")

	// open a file for writing the results
	// seems like a lot of steps to get the application(executable) name and change the extension
	var logName string
	logName, _ = os.Executable() // prob should check the error here as well
	logName = strings.ToLower(logName)
	logName = strings.TrimSuffix(logName, filepath.Ext(logName))
	logName += ".log"

	// delete any existing log file
	// likely not necesarry as os.Create will overwrite
	// could use to catch an error when file is in use
	os.Remove(logName) // prob should check for error

	// create new log file
	f, err := os.Create(logName)
	if err != nil {
		// prob should do something if err
	}
	defer f.Close()

	// write out the results to a log file; base file first, followed by any duplicates of that file
	var arr []string
	for _, v := range dups {
		if strings.Trim(v.duplicateList, " \"") != "" {
			//fmt.Println("Base File:    ", doubleQuote(v.fullpath))
			f.WriteString("Base File:    " + doubleQuote(v.fullpath) + "\n")

			arr = strings.Split(v.duplicateList, "\",") // split at  ",
			for s := range arr {
				arr[s] = doubleQuote(arr[s])
				if arr[s] != "\"\"" {
					//fmt.Println("  - Duplicate:", arr[s])
					f.WriteString("  - Duplicate:" + arr[s] + "\n")
				}
			}
			//fmt.Println("")
			f.WriteString("\n")
		}
	}

	fmt.Println("Finished.  See dup.log for results.")
	fmt.Println("")
}
