package regolith

import (
	"bufio"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/fatih/color"
)

// appDataCachePath is a path to the cache directory relative to the user's
// app data
const appDataCachePath = "regolith/project-cache"

func StringArrayContains(arr []string, str string) bool {
	for _, a := range arr {
		if a == str {
			return true
		}
	}
	return false
}

// nth returns the ordinal numeral of the index of a table. For example:
// nth(0) returns "1st", nth(1) returns "2nd", etc.
func nth(i int) string {
	i += 1
	j := i % 100
	if j > 10 && j < 20 {
		return fmt.Sprintf("%dth", i)
	}
	switch j % 10 {
	case 1:
		return fmt.Sprintf("%dst", i)
	case 2:
		return fmt.Sprintf("%dnd", i)
	case 3:
		return fmt.Sprintf("%drd", i)
	}
	return fmt.Sprintf("%dth", i)
}

// firstErr returns the first error in a list of errors. If the list is empty
// or all errors are nil, nil is returned.
func firstErr(errors ...error) error {
	for _, err := range errors {
		if err != nil {
			return err
		}
	}
	return nil
}

// wrapErrorStackTrace is used by other wrapped error functions to add a stack
// trace to the error message.
func wrapErrorStackTrace(err error, text string) error {
	text = strings.Replace(text, "\n", color.YellowString("\n   >> "), -1)

	if err != nil {
		text = fmt.Sprintf(
			"%s\n[%s]: %s", text, color.RedString("+"), err.Error())
	}
	if printStackTraces {
		pc, fn, line, _ := runtime.Caller(2)
		text = fmt.Sprintf(
			"%s\n   [%s] %s:%d", text, runtime.FuncForPC(pc).Name(),
			filepath.Base(fn), line)
	}
	return errors.New(text)
}

func FullFilterToNiceFilterName(name string) string {
	if strings.Contains(name, ":subfilter") {
		i, err := strconv.Atoi(strings.Split(name, ":subfilter")[1])
		if err != nil {
			return fmt.Sprintf("the \"%s\" filter", name)
		}
		return NiceSubfilterName(strings.Split(name, ":")[0], i)
	}
	return fmt.Sprintf("the \"%s\" filter", name)
}

func ShortFilterName(name string) string {
	if strings.Contains(name, ":subfilter") {
		return strings.Split(name, ":")[0]
	}
	return name
}

func NiceSubfilterName(name string, i int) string {
	return fmt.Sprintf("the %s subfilter of \"%s\" filter", nth(i), name)
}

// PassError adds stack trace to an error without any additional text.
func PassError(err error) error {
	text := err.Error()
	if printStackTraces {
		pc, fn, line, _ := runtime.Caller(1)
		text = fmt.Sprintf(
			"%s\n   [%s] %s:%d", text, runtime.FuncForPC(pc).Name(),
			filepath.Base(fn), line)
	}
	return errors.New(text)
}

// NotImplementedError is used by default functions, that need implementation.
func NotImplementedError(text string) error {
	text = fmt.Sprintf("Function not implemented: %s", text)
	return wrapErrorStackTrace(nil, text)
}

// WrappedError creates an error with a stack trace from text.
func WrappedError(text string) error {
	return wrapErrorStackTrace(nil, text)
}

// WrappedErrorf creates an error with a stack trace from formatted text.
func WrappedErrorf(text string, args ...interface{}) error {
	text = fmt.Sprintf(text, args...)
	return wrapErrorStackTrace(nil, text)
}

// WrapError wraps an error with a stack trace and adds additional text
// information.
func WrapError(err error, text string) error {
	return wrapErrorStackTrace(err, text)
}

// WrapErrorf wraps an error with a stack trace and adds additional formatted
// text information.
func WrapErrorf(err error, text string, args ...interface{}) error {
	return wrapErrorStackTrace(err, fmt.Sprintf(text, args...))
}

func CreateDirectoryIfNotExists(directory string, mustSucceed bool) error {
	if _, err := os.Stat(directory); os.IsNotExist(err) {
		err = os.MkdirAll(directory, 0755)
		if err != nil {
			if mustSucceed {
				// Error outside of this function should tell about the path
				return PassError(err)
			} else {
				Logger.Warnf(
					"Failed to create directory %s: %s.", directory,
					err.Error())
				return nil
			}
		}
	}
	return nil
}

// GetAbsoluteWorkingDirectory returns an absolute path to [dotRegolithPath]/tmp
func GetAbsoluteWorkingDirectory(dotRegolithPath string) string {
	absoluteWorkingDir, _ := filepath.Abs(filepath.Join(dotRegolithPath, "tmp"))
	return absoluteWorkingDir
}

// CreateEnvironmentVariables creates an array of environment variables including custom ones
func CreateEnvironmentVariables(filterDir string) ([]string, error) {
	projectDir, err := os.Getwd()
	if err != nil {
		return nil, WrapErrorf(err, osGetwdError)
	}
	// TODO - doesn't os.GetWd() already return an absolute path?
	projectDir, err = filepath.Abs(projectDir)
	if err != nil {
		return nil, WrapErrorf(err, filepathAbsError, projectDir)
	}
	return append(os.Environ(), "FILTER_DIR="+filterDir, "ROOT_DIR="+projectDir), nil
}

// RunSubProcess runs a sub-process with specified arguments and working
// directory
func RunSubProcess(command string, args []string, filterDir string, workingDir string, outputLabel string) error {
	Logger.Debugf("Exec: %s %s", command, strings.Join(args, " "))
	cmd := exec.Command(command, args...)
	cmd.Dir = workingDir
	out, _ := cmd.StdoutPipe()
	err, _ := cmd.StderrPipe()
	go LogStd(out, Logger.Infof, outputLabel)
	go LogStd(err, Logger.Errorf, outputLabel)
	env, err1 := CreateEnvironmentVariables(filterDir)
	if err1 != nil {
		return WrapErrorf(
			err1,
			"Failed to create FILTER_DIR and ROOT_DIR environment variables.")
	}
	cmd.Env = env

	return cmd.Run()
}

func LogStd(in io.ReadCloser, logFunc func(template string, args ...interface{}), outputLabel string) {
	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		logFunc("[%s] %s", outputLabel, scanner.Text())
	}
}

// GetDotRegolith returns the path to the directory where Regolith stores
// its cached data (like filters, Python venvs, etc.). If useAppData is set to
// false it returns relative director: ".regolith" otherwise it returns path
// inside the AppData directory. Based on the hash value of the
// project's root directory. If the path isn't .regolith it also logs a message
// which tells where the data is stored unless the silent flag is set to true.
// The projectRoot path can be relative or absolute and is resolved to an
// absolute path.
func GetDotRegolith(useAppData, silent bool, projectRoot string) (string, error) {
	// App data diabled - use .regolith
	if !useAppData {
		return ".regolith", nil
	}
	// App data enabled - use user cache dir
	userCache, err := os.UserCacheDir()
	if err != nil {
		return "", WrappedError(osUserCacheDirError)
	}
	// Make sure that projectsRoot is an absolute path
	absoluteProjectRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return "", WrapErrorf(err, filepathAbsError, projectRoot)
	}
	// Get the md5 of the project path
	hash := md5.New()
	hash.Write([]byte(absoluteProjectRoot))
	hashInBytes := hash.Sum(nil)
	projectPathHash := hex.EncodeToString(hashInBytes)
	// %userprofile%/AppData/Local/regolith/<md5 of project path>
	dotRegolithPath := filepath.Join(
		userCache, appDataCachePath, projectPathHash)
	if !silent {
		Logger.Infof(
			"Regolith project cache is in:\n\t%s",
			dotRegolithPath)
	}
	return dotRegolithPath, nil
}
