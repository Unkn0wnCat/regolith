package test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"bedrock-oss.github.com/regolith/regolith"
	"github.com/otiai10/copy"
)

// TestRegolithInit tests the results of InitializeRegolithProject against
// the values from test/testdata/fresh_project.
func TestRegolithInit(t *testing.T) {
	// Switching working directories in this test, make sure to go back
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal("Unable to get current working directory")
	}
	defer os.Chdir(wd)
	// Get paths expected in initialized project
	expectedPaths, err := listPaths(
		freshProjectPath, freshProjectPath)
	if err != nil {
		t.Fatal("Unable to get list of created paths:", err)
	}
	// Create temporary directory
	tmpDir, err := ioutil.TempDir("", "regolith-test")
	if err != nil {
		t.Fatal("Unable to create temporary directory:", err)
	}
	t.Log("Created temporary path:", tmpDir)
	// Before removing working dir make sure the script isn't using it anymore
	defer os.RemoveAll(tmpDir)
	defer os.Chdir(wd)

	// Change working directory to the tmp path
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal("Unable to change working directory:", err.Error())
	}
	// THE TEST
	err = regolith.Init(false, true)
	if err != nil {
		t.Fatal("'regolith init' failed:", err.Error())
	}
	createdPaths, err := listPaths(".", ".")
	if err != nil {
		t.Fatal("Unable to get list of created paths:", err)
	}
	comparePathMaps(expectedPaths, createdPaths, t)
}

// TestRegolithRunMissingRp tests the behavior of RunProfile when the packs/RP
// directory is missing.
func TestRegolithRunMissingRp(t *testing.T) {
	// SETUP
	// Switching working directories in this test, make sure to go back
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal("Unable to get current working directory")
	}
	defer os.Chdir(wd)
	// Create a temporary directory
	tmpDir, err := ioutil.TempDir("", "regolith-test")
	if err != nil {
		t.Fatal("Unable to create temporary directory:", err)
	}
	t.Log("Created temporary directory:", tmpDir)
	// Before deleting "workingDir" the test must stop using it
	defer os.RemoveAll(tmpDir)
	defer os.Chdir(wd)
	os.Mkdir(tmpDir, 0666)
	// Copy the test project to the working directory
	err = copy.Copy(
		runMissingRpProjectPath,
		tmpDir,
		copy.Options{PreserveTimes: false, Sync: false},
	)
	if err != nil {
		t.Fatalf(
			"Failed to copy test files %q into the working directory %q",
			multitargetProjectPath, tmpDir,
		)
	}
	// Switch to the working directory
	os.Chdir(tmpDir)
	// THE TEST
	err = regolith.Run("dev", true)
	if err != nil {
		t.Fatal("'regolith run' failed:", err)
	}
}

// TestLocalRequirementsInstallAndRun tests if Regolith properly installs the
// project that uses local script with requirements.txt by running
// "regolith install" first and then "regolith run" on that project.
func TestLocalRequirementsInstallAndRun(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal("Unable to get current working directory")
	}
	defer os.Chdir(wd)
	// Create a temporary directory
	tmpDir, err := ioutil.TempDir("", "regolith-test")
	if err != nil {
		t.Fatal("Unable to create temporary directory:", err)
	}
	t.Log("Created temporary directory:", tmpDir)
	// Before deleting "workingDir" the test must stop using it
	defer os.RemoveAll(tmpDir)
	defer os.Chdir(wd)
	// Copy the test project to the working directory
	err = copy.Copy(
		localRequirementsPath,
		tmpDir,
		copy.Options{PreserveTimes: false, Sync: false},
	)
	if err != nil {
		t.Fatalf(
			"Failed to copy test files %q into the working directory %q",
			localRequirementsPath, tmpDir,
		)
	}
	// Switch to the working directory
	os.Chdir(filepath.Join(tmpDir, "project"))
	// THE TEST
	err = regolith.InstallAll(false, true)
	if err != nil {
		t.Fatal("'regolith install-all' failed", err.Error())
	}
	if err := regolith.Unlock(true); err != nil {
		t.Fatal("'regolith unlock' failed:", err.Error())
	}
	if err := regolith.Run("dev", true); err != nil {
		t.Fatal("'regolith run' failed:", err.Error())
	}
}