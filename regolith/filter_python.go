package regolith

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type PythonFilterDefinition struct {
	FilterDefinition
	Script   string `json:"script,omitempty"`
	VenvSlot int    `json:"venvSlot,omitempty"`
}

type PythonFilter struct {
	Filter
	Definition PythonFilterDefinition `json:"-"`
}

func PythonFilterDefinitionFromObject(id string, obj map[string]interface{}) (*PythonFilterDefinition, error) {
	filter := &PythonFilterDefinition{FilterDefinition: *FilterDefinitionFromObject(id, obj)}
	script, ok := obj["script"].(string)
	if !ok {
		return nil, WrapErrorf(
			nil, "Missing \"script\" property in filter definition %q.",
			filter.Id)
	}
	filter.Script = script
	filter.VenvSlot, _ = obj["venvSlot"].(int) // default venvSlot is 0
	return filter, nil
}

func (f *PythonFilter) Run(absoluteLocation string) error {
	// Disabled filters are skipped
	if f.Disabled {
		Logger.Infof("Filter \"%s\" is disabled, skipping.", f.Id)
		return nil
	}
	Logger.Infof("Running filter %s", f.Id)
	start := time.Now()
	defer Logger.Debugf("Executed in %s", time.Since(start))

	// Run filter
	pythonCommand, err := findPython()
	if err != nil {
		return PassError(err)
	}
	scriptPath := filepath.Join(absoluteLocation, f.Definition.Script)
	if needsVenv(filepath.Dir(scriptPath)) {
		venvPath, err := f.Definition.resolveVenvPath()
		if err != nil {
			return WrapError(err, "Failed to resolve venv path.")
		}
		Logger.Debug("Running Python filter using venv: ", venvPath)
		pythonCommand = filepath.Join(
			venvPath, venvScriptsPath, "python"+exeSuffix)
	}
	var args []string
	if len(f.Settings) == 0 {
		args = append([]string{"-u", scriptPath}, append(
			f.Definition.Arguments,
			f.Arguments...,
		)...)
	} else {
		jsonSettings, _ := json.Marshal(f.Settings)
		args = append(
			[]string{"-u", scriptPath, string(jsonSettings)},
			append(
				f.Definition.Arguments,
				f.Arguments...,
			)...,
		)
	}
	err = RunSubProcess(
		pythonCommand, args, absoluteLocation, GetAbsoluteWorkingDirectory())
	if err != nil {
		return WrapError(err, "Failed to run Python script.")
	}
	return nil
}

func (f *PythonFilterDefinition) CreateFilterRunner(runConfiguration map[string]interface{}) (FilterRunner, error) {
	basicFilter, err := FilterFromObject(runConfiguration)
	if err != nil {
		return nil, WrapError(err, "Failed to create Python filter.")
	}
	filter := &PythonFilter{
		Filter:     *basicFilter,
		Definition: *f,
	}
	return filter, nil
}

func (f *PythonFilterDefinition) InstallDependencies(parent *RemoteFilterDefinition) error {
	installLocation := ""
	// Install dependencies
	if parent != nil {
		installLocation = parent.GetDownloadPath()
	}
	Logger.Infof("Downloading dependencies for %s...", f.Id)
	scriptPath, err := filepath.Abs(filepath.Join(installLocation, f.Script))
	if err != nil {
		return WrapErrorf(err, "Unable to resolve path of %s script.", f.Id)
	}

	// Install the filter dependencies
	filterPath := filepath.Dir(scriptPath)
	if needsVenv(filterPath) {
		venvPath, err := f.resolveVenvPath()
		if err != nil {
			return WrapError(err, "Failed to resolve venv path.")
		}
		Logger.Info("Creating venv...")
		pythonCommand, err := findPython()
		if err != nil {
			return PassError(err)
		}
		// Create the "venv"
		err = RunSubProcess(
			pythonCommand, []string{"-m", "venv", venvPath}, filterPath, "")
		if err != nil {
			return WrapError(err, "Failed to create venv.")
		}
		Logger.Info("Installing pip dependencies...")
		err = RunSubProcess(
			filepath.Join(venvPath, venvScriptsPath, "pip"+exeSuffix),
			[]string{"install", "-r", "requirements.txt"}, filterPath, filterPath)
		if err != nil {
			return WrapErrorf(
				err, "couldn't run pip to install dependencies of %s",
				f.Id,
			)
		}
	}
	Logger.Infof("Dependencies for %s installed successfully.", f.Id)
	return nil
}

func (f *PythonFilterDefinition) Check() error {
	pythonCommand, err := findPython()
	if err != nil {
		return PassError(err)
	}
	cmd, err := exec.Command(pythonCommand, "--version").Output()
	if err != nil {
		return WrapError(err, "Python version check failed.")
	}
	a := strings.TrimPrefix(strings.Trim(string(cmd), " \n\t"), "Python ")
	Logger.Debugf("Found Python version %s.", a)
	return nil
}

func (f *PythonFilter) Check() error {
	return f.Definition.Check()
}

func (f *PythonFilter) CopyArguments(parent *RemoteFilter) {
	f.Arguments = parent.Arguments
	f.Settings = parent.Settings
	f.Definition.VenvSlot = parent.Definition.VenvSlot
}

func (f *PythonFilterDefinition) resolveVenvPath() (string, error) {
	resolvedPath, err := filepath.Abs(
		filepath.Join(".regolith/cache/venvs", strconv.Itoa(f.VenvSlot)))
	if err != nil {
		return "", WrapErrorf(
			err, "Unable to create venv for VenvSlot %v.", f.VenvSlot)
	}
	return resolvedPath, nil
}

func needsVenv(filterPath string) bool {
	stats, err := os.Stat(filepath.Join(filterPath, "requirements.txt"))
	if err == nil {
		return !stats.IsDir()
	}
	return false
}

func findPython() (string, error) {
	var err error
	for _, c := range []string{"python", "python3"} {
		_, err = exec.LookPath(c)
		if err == nil {
			return c, nil
		}
	}
	return "", WrappedError(
		"Python not found, download and install it from " +
			"https://www.python.org/downloads/")
}
